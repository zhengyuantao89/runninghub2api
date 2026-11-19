package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type UserSession struct {
	ID        string
	UserID    int64
	Username  string
	Role      string
	ExpiresAt time.Time
}

func (s *Store) migrateSessions() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS user_sessions (
  id TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL,
  username TEXT NOT NULL DEFAULT '',
  role TEXT NOT NULL DEFAULT '',
  expires_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires ON user_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user ON user_sessions(user_id);
`)
	return err
}

func (s *Store) CreateUserSession(id string, userID int64, username, role string, expiresAt time.Time) error {
	id = strings.TrimSpace(id)
	if id == "" || userID <= 0 {
		return errors.New("invalid session")
	}
	ts := now()
	exp := expiresAt.UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
INSERT INTO user_sessions(id,user_id,username,role,expires_at,created_at,updated_at)
VALUES(?,?,?,?,?,?,?)
`, id, userID, strings.TrimSpace(username), strings.TrimSpace(role), exp, ts, ts)
	return err
}

func (s *Store) GetUserSession(id string) (UserSession, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return UserSession{}, ErrNotFound
	}
	var out UserSession
	var exp string
	err := s.db.QueryRow(`
SELECT id,user_id,username,role,expires_at FROM user_sessions WHERE id=?
`, id).Scan(&out.ID, &out.UserID, &out.Username, &out.Role, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return UserSession{}, ErrNotFound
	}
	if err != nil {
		return UserSession{}, err
	}
	t, err := time.Parse(time.RFC3339, exp)
	if err != nil {
		return UserSession{}, err
	}
	out.ExpiresAt = t
	return out, nil
}

func (s *Store) TouchUserSession(id string, expiresAt time.Time) error {
	_, err := s.db.Exec(`UPDATE user_sessions SET expires_at=?, updated_at=? WHERE id=?`,
		expiresAt.UTC().Format(time.RFC3339), now(), strings.TrimSpace(id))
	return err
}

func (s *Store) DeleteUserSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM user_sessions WHERE id=?`, strings.TrimSpace(id))
	return err
}

func (s *Store) PurgeExpiredUserSessions() error {
	_, err := s.db.Exec(`DELETE FROM user_sessions WHERE expires_at<>'' AND expires_at<?`, time.Now().UTC().Format(time.RFC3339))
	return err
}
