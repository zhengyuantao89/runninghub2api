package store

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	MailUnused = "unused"
	MailLeased = "leased"
	MailUsed   = "used"
	MailDead   = "dead"
)

type MailInbox struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	PickupURL string `json:"pickup_url"`
	Status    string `json:"status"`
	LastError string `json:"last_error,omitempty"`
	LeasedAt  string `json:"leased_at,omitempty"`
	UsedAt    string `json:"used_at,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Store) migrateMailInboxes() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS mail_inboxes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL DEFAULT 0,
  email TEXT NOT NULL DEFAULT '',
  pickup_url TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'unused',
  last_error TEXT NOT NULL DEFAULT '',
  leased_at TEXT NOT NULL DEFAULT '',
  used_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(email)
);
CREATE INDEX IF NOT EXISTS idx_mail_inboxes_status ON mail_inboxes(status);
CREATE INDEX IF NOT EXISTS idx_mail_inboxes_user ON mail_inboxes(user_id);
`)
	return err
}

// IsICloudEmail reports whether email is strictly @icloud.com (RH only accepts this domain).
func IsICloudEmail(email string) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	return email[at+1:] == "icloud.com"
}

func ParseInboxImportLine(line string) (email, pickup string, err error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", fmt.Errorf("empty")
	}
	parts := strings.Split(line, "----")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("需要 邮箱----取件链接")
	}
	email = strings.TrimSpace(parts[0])
	pickup = strings.TrimSpace(strings.Join(parts[1:], "----"))
	if email == "" || !strings.Contains(email, "@") {
		return "", "", fmt.Errorf("邮箱格式无效")
	}
	if !IsICloudEmail(email) {
		return "", "", fmt.Errorf("仅支持 @icloud.com")
	}
	if !strings.HasPrefix(strings.ToLower(pickup), "http://") && !strings.HasPrefix(strings.ToLower(pickup), "https://") {
		return "", "", fmt.Errorf("取件链接必须是 http(s) URL")
	}
	if _, e := url.Parse(pickup); e != nil {
		return "", "", fmt.Errorf("取件链接无效")
	}
	return email, pickup, nil
}

func (s *Store) ImportMailInboxes(userID int64, text string) (imported, updated, skipped int, errors []string) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, raw := range lines {
		email, pickup, err := ParseInboxImportLine(raw)
		if err != nil {
			if strings.TrimSpace(raw) == "" || strings.HasPrefix(strings.TrimSpace(raw), "#") {
				continue
			}
			skipped++
			errors = append(errors, fmt.Sprintf("L%d: %s", i+1, err.Error()))
			continue
		}
		var exists int
		_ = s.db.QueryRow(`SELECT COUNT(1) FROM mail_inboxes WHERE email=?`, email).Scan(&exists)
		ts := now()
		_, err = s.db.Exec(`
INSERT INTO mail_inboxes(user_id, email, pickup_url, status, created_at, updated_at)
VALUES(?,?,?,?,?,?)
ON CONFLICT(email) DO UPDATE SET
  pickup_url=excluded.pickup_url,
  user_id=CASE WHEN mail_inboxes.user_id=0 THEN excluded.user_id ELSE mail_inboxes.user_id END,
  status=CASE
    WHEN mail_inboxes.status IN ('used','dead') THEN mail_inboxes.status
    ELSE 'unused'
  END,
  last_error=CASE
    WHEN mail_inboxes.status IN ('used','dead') THEN mail_inboxes.last_error
    ELSE ''
  END,
  updated_at=excluded.updated_at
`, userID, email, pickup, MailUnused, ts, ts)
		if err != nil {
			skipped++
			errors = append(errors, fmt.Sprintf("L%d: %s", i+1, err.Error()))
			continue
		}
		if exists > 0 {
			updated++
		} else {
			imported++
		}
	}
	return imported, updated, skipped, errors
}

func (s *Store) ListMailInboxes(userID int64, status string, limit, offset int) ([]MailInbox, error) {
	where := []string{}
	args := []any{}
	if userID > 0 {
		where = append(where, "user_id=?")
		args = append(args, userID)
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == MailUnused || status == MailLeased || status == MailUsed || status == MailDead {
		where = append(where, "status=?")
		args = append(args, status)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	sqlText := `SELECT id,user_id,email,pickup_url,status,last_error,leased_at,used_at,created_at,updated_at FROM mail_inboxes` + clause + ` ORDER BY id DESC`
	if limit > 0 {
		if offset < 0 {
			offset = 0
		}
		sqlText += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	rows, err := s.db.Query(sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MailInbox, 0)
	for rows.Next() {
		var m MailInbox
		if err := rows.Scan(&m.ID, &m.UserID, &m.Email, &m.PickupURL, &m.Status, &m.LastError, &m.LeasedAt, &m.UsedAt, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) CountMailInboxes(userID int64, status string) (int, error) {
	where := []string{}
	args := []any{}
	if userID > 0 {
		where = append(where, "user_id=?")
		args = append(args, userID)
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == MailUnused || status == MailLeased || status == MailUsed || status == MailDead {
		where = append(where, "status=?")
		args = append(args, status)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_inboxes`+clause, args...).Scan(&n)
	return n, err
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "busy")
}

func (s *Store) LeaseMailInbox(userID int64) (MailInbox, error) {
	var last error
	for attempt := 0; attempt < 8; attempt++ {
		m, err := s.leaseMailInboxOnce(userID)
		if err == nil || err == ErrNotFound {
			return m, err
		}
		last = err
		if !isSQLiteBusy(err) {
			return MailInbox{}, err
		}
		time.Sleep(time.Duration(40*(attempt+1)) * time.Millisecond)
	}
	return MailInbox{}, last
}

func (s *Store) leaseMailInboxOnce(userID int64) (MailInbox, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return MailInbox{}, err
	}
	defer tx.Rollback()

	ts := now()
	q := `
UPDATE mail_inboxes
SET status=?, leased_at=?, last_error='', updated_at=?
WHERE id=(
  SELECT id FROM mail_inboxes WHERE status=?`
	args := []any{MailLeased, ts, ts, MailUnused}
	if userID > 0 {
		q += ` AND user_id=?`
		args = append(args, userID)
	}
	q += ` ORDER BY id ASC LIMIT 1
) AND status=?
RETURNING id,user_id,email,pickup_url,status,last_error,leased_at,used_at,created_at,updated_at`
	args = append(args, MailUnused)

	var m MailInbox
	err = tx.QueryRow(q, args...).Scan(
		&m.ID, &m.UserID, &m.Email, &m.PickupURL, &m.Status, &m.LastError, &m.LeasedAt, &m.UsedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return MailInbox{}, ErrNotFound
	}
	if err != nil {
		return MailInbox{}, err
	}
	if err := tx.Commit(); err != nil {
		return MailInbox{}, err
	}
	return m, nil
}

func (s *Store) ReleaseMailInbox(id int64, toStatus, lastError string) error {
	toStatus = strings.ToLower(strings.TrimSpace(toStatus))
	if toStatus == "" {
		toStatus = MailUnused
	}
	ts := now()
	usedAt := ""
	if toStatus == MailUsed {
		usedAt = ts
	}
	_, err := s.db.Exec(`UPDATE mail_inboxes SET status=?, last_error=?, used_at=CASE WHEN ?!='' THEN ? ELSE used_at END, updated_at=? WHERE id=?`,
		toStatus, strings.TrimSpace(lastError), usedAt, usedAt, ts, id)
	return err
}

func (s *Store) DeleteMailInbox(id, userID int64) error {
	if userID > 0 {
		res, err := s.db.Exec(`DELETE FROM mail_inboxes WHERE id=? AND user_id=?`, id, userID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	res, err := s.db.Exec(`DELETE FROM mail_inboxes WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteMailInboxes deletes many rows. userID>0 scopes to that owner; 0 = admin all.
func (s *Store) DeleteMailInboxes(ids []int64, userID int64) (int, error) {
	uniq := make([]int64, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return 0, nil
	}
	deleted := 0
	for _, id := range uniq {
		if err := s.DeleteMailInbox(id, userID); err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func (s *Store) migrateRegisterJobs() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS register_jobs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'queued',
  quantity INTEGER NOT NULL DEFAULT 0,
  success INTEGER NOT NULL DEFAULT 0,
  failed INTEGER NOT NULL DEFAULT 0,
  invite_code TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  finished_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS register_job_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id INTEGER NOT NULL,
  level TEXT NOT NULL DEFAULT 'info',
  message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_register_job_logs_job ON register_job_logs(job_id);
`)
	return err
}

type RegisterJob struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Status     string `json:"status"`
	Quantity   int    `json:"quantity"`
	Success    int    `json:"success"`
	Failed     int    `json:"failed"`
	InviteCode string `json:"invite_code,omitempty"`
	Error      string `json:"error,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	FinishedAt string `json:"finished_at,omitempty"`
}

type RegisterJobLog struct {
	ID        int64  `json:"id"`
	JobID     int64  `json:"job_id"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) CreateRegisterJob(userID int64, quantity int, inviteCode string) (RegisterJob, error) {
	ts := now()
	res, err := s.db.Exec(`INSERT INTO register_jobs(user_id,status,quantity,invite_code,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		userID, "queued", quantity, strings.TrimSpace(inviteCode), ts, ts)
	if err != nil {
		return RegisterJob{}, err
	}
	id, _ := res.LastInsertId()
	return RegisterJob{ID: id, UserID: userID, Status: "queued", Quantity: quantity, InviteCode: inviteCode, CreatedAt: ts, UpdatedAt: ts}, nil
}

func (s *Store) UpdateRegisterJob(id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	cols := make([]string, 0, len(fields)+1)
	args := make([]any, 0, len(fields)+2)
	for k, v := range fields {
		cols = append(cols, k+"=?")
		args = append(args, v)
	}
	cols = append(cols, "updated_at=?")
	args = append(args, now(), id)
	_, err := s.db.Exec(`UPDATE register_jobs SET `+strings.Join(cols, ",")+` WHERE id=?`, args...)
	return err
}

func (s *Store) AppendRegisterJobLog(jobID int64, level, message string) error {
	_, err := s.db.Exec(`INSERT INTO register_job_logs(job_id,level,message,created_at) VALUES(?,?,?,?)`,
		jobID, strings.TrimSpace(level), strings.TrimSpace(message), now())
	return err
}

func (s *Store) ListRegisterJobLogs(jobID int64, afterID int64, limit int) ([]RegisterJobLog, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT id,job_id,level,message,created_at FROM register_job_logs WHERE job_id=? AND id>? ORDER BY id ASC LIMIT ?`,
		jobID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RegisterJobLog, 0)
	for rows.Next() {
		var l RegisterJobLog
		if err := rows.Scan(&l.ID, &l.JobID, &l.Level, &l.Message, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) GetRegisterJob(id int64) (RegisterJob, error) {
	var j RegisterJob
	err := s.db.QueryRow(`SELECT id,user_id,status,quantity,success,failed,invite_code,error,created_at,updated_at,finished_at FROM register_jobs WHERE id=?`, id).
		Scan(&j.ID, &j.UserID, &j.Status, &j.Quantity, &j.Success, &j.Failed, &j.InviteCode, &j.Error, &j.CreatedAt, &j.UpdatedAt, &j.FinishedAt)
	if err == sql.ErrNoRows {
		return RegisterJob{}, ErrNotFound
	}
	return j, err
}

func (s *Store) ListRegisterJobs(userID int64, limit int) ([]RegisterJob, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT id,user_id,status,quantity,success,failed,invite_code,error,created_at,updated_at,finished_at FROM register_jobs`
	args := []any{}
	if userID > 0 {
		q += ` WHERE user_id=?`
		args = append(args, userID)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RegisterJob, 0)
	for rows.Next() {
		var j RegisterJob
		if err := rows.Scan(&j.ID, &j.UserID, &j.Status, &j.Quantity, &j.Success, &j.Failed, &j.InviteCode, &j.Error, &j.CreatedAt, &j.UpdatedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// PurgeFinishedRegisterJobsBefore deletes done/cancelled register jobs (and logs) older than cutoff.
func (s *Store) PurgeFinishedRegisterJobsBefore(cutoffRFC3339 string, limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`
SELECT id FROM register_jobs
WHERE status IN ('done','cancelled') AND (
  (finished_at!='' AND finished_at < ?) OR (finished_at='' AND updated_at < ?)
)
ORDER BY id ASC LIMIT ?`, cutoffRFC3339, cutoffRFC3339, limit)
	if err != nil {
		return 0, err
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	deleted := 0
	for _, id := range ids {
		if _, err := s.db.Exec(`DELETE FROM register_job_logs WHERE job_id=?`, id); err != nil {
			return deleted, err
		}
		res, err := s.db.Exec(`DELETE FROM register_jobs WHERE id=?`, id)
		if err != nil {
			return deleted, err
		}
		n, _ := res.RowsAffected()
		deleted += int(n)
	}
	return deleted, nil
}

// ClearFinishedRegisterJobs deletes terminal register jobs (and logs) for a user.
// userID<=0 clears all users. Does not touch queued/running/cancelling.
func (s *Store) ClearFinishedRegisterJobs(userID int64) (int, error) {
	q := `SELECT id FROM register_jobs WHERE status IN ('done','failed','cancelled')`
	args := []any{}
	if userID > 0 {
		q += ` AND user_id=?`
		args = append(args, userID)
	}
	q += ` ORDER BY id ASC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return 0, err
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	deleted := 0
	for _, id := range ids {
		if _, err := s.db.Exec(`DELETE FROM register_job_logs WHERE job_id=?`, id); err != nil {
			return deleted, err
		}
		res, err := s.db.Exec(`DELETE FROM register_jobs WHERE id=?`, id)
		if err != nil {
			return deleted, err
		}
		n, _ := res.RowsAffected()
		deleted += int(n)
	}
	return deleted, nil
}
