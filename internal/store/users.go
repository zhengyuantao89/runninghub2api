package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"

	UserActive   = "active"
	UserDisabled = "disabled"

	SettingRegistrationEnabled = "registration_enabled"
)

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type UserAPIKey struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Name       string `json:"name"`
	KeyPrefix  string `json:"key_prefix"`
	KeyPlain   string `json:"key,omitempty"` // owner-visible secret for console reveal/copy
	KeyHash    string `json:"-"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func hashAPIKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func (s *Store) ensureUsersSchema() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  status TEXT NOT NULL DEFAULT 'active',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role, status);
CREATE TABLE IF NOT EXISTS user_api_keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  key_prefix TEXT NOT NULL DEFAULT '',
  key_plain TEXT NOT NULL DEFAULT '',
  key_hash TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_used_at TEXT NOT NULL DEFAULT '',
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_api_keys_user ON user_api_keys(user_id);
`)
	if err != nil {
		return err
	}
	return s.addColumnIfMissing("user_api_keys", "key_plain", "key_plain TEXT NOT NULL DEFAULT ''")
}

func (s *Store) CountAdmins() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM users WHERE role=?`, RoleAdmin).Scan(&n)
	return n, err
}

// EnsureAdminUser bootstraps the first admin from env only when no admin exists.
// Existing admin username/password in DB are never overwritten by deploy/.env.
func (s *Store) EnsureAdminUser(username, password string) (User, error) {
	if n, err := s.CountAdmins(); err != nil {
		return User{}, err
	} else if n > 0 {
		if id := s.FirstAdminID(); id > 0 {
			return s.GetUser(id)
		}
	}

	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		username = "admin"
	}
	if u, err := s.GetUserByUsername(username); err == nil {
		if u.Role != RoleAdmin {
			_, _ = s.db.Exec(`UPDATE users SET role=?, updated_at=? WHERE id=?`, RoleAdmin, now(), u.ID)
			u.Role = RoleAdmin
		}
		return u, nil
	} else if !errors.Is(err, ErrNotFound) {
		return User{}, err
	}
	if strings.TrimSpace(password) == "" {
		password = "admin123"
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	return s.CreateUser(username, hash, RoleAdmin)
}

func (s *Store) CreateUser(username, passwordHash, role string) (User, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		return User{}, fmt.Errorf("username required")
	}
	if role != RoleAdmin {
		role = RoleUser
	}
	ts := now()
	res, err := s.db.Exec(
		`INSERT INTO users(username, password_hash, role, status, created_at, updated_at) VALUES(?,?,?,?,?,?)`,
		username, passwordHash, role, UserActive, ts, ts,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return User{}, fmt.Errorf("用户名已存在")
		}
		return User{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetUser(id)
}

func (s *Store) GetUser(id int64) (User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, username, password_hash, role, status, created_at, updated_at FROM users WHERE id=?`, id,
	))
}

func (s *Store) GetUserByUsername(username string) (User, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	return s.scanUser(s.db.QueryRow(
		`SELECT id, username, password_hash, role, status, created_at, updated_at FROM users WHERE username=? COLLATE NOCASE`,
		username,
	))
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, password_hash, role, status, created_at, updated_at FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := s.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UsernameByID returns id -> username for all users.
func (s *Store) UsernameByID() (map[int64]string, error) {
	rows, err := s.db.Query(`SELECT id, username FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]string)
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

func (s *Store) SetUserStatus(id int64, status string) error {
	if status != UserActive && status != UserDisabled {
		return fmt.Errorf("invalid status")
	}
	res, err := s.db.Exec(`UPDATE users SET status=?, updated_at=? WHERE id=?`, status, now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetUserPassword(id int64, passwordHash string) error {
	res, err := s.db.Exec(`UPDATE users SET password_hash=?, updated_at=? WHERE id=?`, passwordHash, now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetUserUsername(id int64, username string) error {
	username = strings.TrimSpace(strings.ToLower(username))
	if len(username) < 3 {
		return fmt.Errorf("用户名至少 3 位")
	}
	res, err := s.db.Exec(`UPDATE users SET username=?, updated_at=? WHERE id=?`, username, now(), id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("用户名已存在")
		}
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RegistrationEnabled() bool {
	v, err := s.GetSetting(SettingRegistrationEnabled)
	if err != nil || strings.TrimSpace(v) == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

func (s *Store) SetRegistrationEnabled(on bool) error {
	val := "0"
	if on {
		val = "1"
	}
	return s.SetSetting(SettingRegistrationEnabled, val)
}

// CreateUserAPIKey creates a key and persists plaintext for owner console reveal/copy.
func (s *Store) CreateUserAPIKey(userID int64, name string) (UserAPIKey, string, error) {
	if userID <= 0 {
		return UserAPIKey{}, "", fmt.Errorf("user_id required")
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return UserAPIKey{}, "", err
	}
	plain := "rh2_" + hex.EncodeToString(raw)
	prefix := plain
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}
	ts := now()
	res, err := s.db.Exec(
		`INSERT INTO user_api_keys(user_id, name, key_prefix, key_plain, key_hash, created_at, last_used_at) VALUES(?,?,?,?,?,?,?)`,
		userID, name, prefix, plain, hashAPIKey(plain), ts, "",
	)
	if err != nil {
		return UserAPIKey{}, "", err
	}
	id, _ := res.LastInsertId()
	k, err := s.getUserAPIKey(id)
	return k, plain, err
}

func (s *Store) ListUserAPIKeys(userID int64) ([]UserAPIKey, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, name, key_prefix, key_plain, key_hash, created_at, last_used_at FROM user_api_keys WHERE user_id=? ORDER BY id DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserAPIKey
	for rows.Next() {
		k, err := s.scanUserAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) DeleteUserAPIKey(userID, keyID int64) error {
	res, err := s.db.Exec(`DELETE FROM user_api_keys WHERE id=? AND user_id=?`, keyID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) LookupUserByAPIKey(plain string) (User, UserAPIKey, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return User{}, UserAPIKey{}, ErrNotFound
	}
	k, err := s.scanUserAPIKey(s.db.QueryRow(
		`SELECT id, user_id, name, key_prefix, key_plain, key_hash, created_at, last_used_at FROM user_api_keys WHERE key_hash=?`,
		hashAPIKey(plain),
	))
	if err != nil {
		return User{}, UserAPIKey{}, err
	}
	u, err := s.GetUser(k.UserID)
	if err != nil {
		return User{}, UserAPIKey{}, err
	}
	_, _ = s.db.Exec(`UPDATE user_api_keys SET last_used_at=? WHERE id=?`, now(), k.ID)
	return u, k, nil
}

func (s *Store) getUserAPIKey(id int64) (UserAPIKey, error) {
	return s.scanUserAPIKey(s.db.QueryRow(
		`SELECT id, user_id, name, key_prefix, key_plain, key_hash, created_at, last_used_at FROM user_api_keys WHERE id=?`, id,
	))
}

func (s *Store) scanUser(row rowScanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) scanUserAPIKey(row rowScanner) (UserAPIKey, error) {
	var k UserAPIKey
	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyPlain, &k.KeyHash, &k.CreatedAt, &k.LastUsedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return UserAPIKey{}, ErrNotFound
	}
	return k, err
}

// AssignOrphanRowsToUser sets user_id on legacy rows still at 0.
func (s *Store) AssignOrphanRowsToUser(userID int64) error {
	if userID <= 0 {
		return fmt.Errorf("user_id required")
	}
	_, err := s.db.Exec(`UPDATE accounts SET user_id=? WHERE IFNULL(user_id,0)=0`, userID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE video_tasks SET user_id=? WHERE IFNULL(user_id,0)=0`, userID)
	return err
}

func (s *Store) FirstAdminID() int64 {
	var id int64
	_ = s.db.QueryRow(`SELECT id FROM users WHERE role=? ORDER BY id ASC LIMIT 1`, RoleAdmin).Scan(&id)
	return id
}
