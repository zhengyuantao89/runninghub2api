package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// EgressSubscription mirrors gpt-plus proxy_subscriptions (single-tenant).
type EgressSubscription struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	GroupName       string `json:"group_name"`
	URL             string `json:"url"`
	IntervalMinutes int    `json:"interval_minutes"`
	Enabled         bool   `json:"enabled"`
	LastSyncAt      string `json:"last_sync_at"`
	LastSyncStatus  string `json:"last_sync_status"`
	LastError       string `json:"last_error"`
	LastAdded       int    `json:"last_added"`
	LastUpdated     int    `json:"last_updated"`
	LastRemoved     int    `json:"last_removed"`
	LastTotal       int    `json:"last_total"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

const subSelect = `SELECT id,name,group_name,url,interval_minutes,enabled,last_sync_at,last_sync_status,last_error,
last_added,last_updated,last_removed,last_total,created_at,updated_at FROM egress_subscriptions`

func (s *Store) scanSubscription(row rowScanner) (EgressSubscription, error) {
	var sub EgressSubscription
	var enabled int
	err := row.Scan(
		&sub.ID, &sub.Name, &sub.GroupName, &sub.URL, &sub.IntervalMinutes, &enabled,
		&sub.LastSyncAt, &sub.LastSyncStatus, &sub.LastError,
		&sub.LastAdded, &sub.LastUpdated, &sub.LastRemoved, &sub.LastTotal,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	sub.Enabled = enabled == 1
	return sub, err
}

func (s *Store) GetEgressSubscription() (EgressSubscription, error) {
	sub, err := s.scanSubscription(s.db.QueryRow(subSelect + ` ORDER BY id ASC LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return EgressSubscription{}, ErrNotFound
	}
	return sub, err
}

func (s *Store) ListEgressSubscriptions() ([]EgressSubscription, error) {
	rows, err := s.db.Query(subSelect + ` ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]EgressSubscription, 0)
	for rows.Next() {
		sub, err := s.scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func normalizeSubInterval(v int) int {
	switch v {
	case 10, 30, 60:
		return v
	}
	if v >= 5 && v <= 1440 {
		return v
	}
	return 30
}

func (s *Store) UpsertEgressSubscription(url string, intervalMinutes int, enabled bool, name, group string) (EgressSubscription, error) {
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(strings.ToLower(url), "http://") && !strings.HasPrefix(strings.ToLower(url), "https://") {
		return EgressSubscription{}, fmt.Errorf("订阅地址必须以 http:// 或 https:// 开头")
	}
	if name = strings.TrimSpace(name); name == "" {
		name = "nodes"
	}
	if group = strings.TrimSpace(group); group == "" {
		group = "nodes"
	}
	interval := normalizeSubInterval(intervalMinutes)
	on := 0
	if enabled {
		on = 1
	}
	ts := now()
	existing, err := s.GetEgressSubscription()
	if err == nil {
		_, err = s.db.Exec(`
UPDATE egress_subscriptions
SET url=?, interval_minutes=?, enabled=?, name=?, group_name=?, updated_at=?
WHERE id=?`, url, interval, on, name, group, ts, existing.ID)
		if err != nil {
			return EgressSubscription{}, err
		}
		return s.GetEgressSubscription()
	}
	if !errors.Is(err, ErrNotFound) {
		return EgressSubscription{}, err
	}
	_, err = s.db.Exec(`
INSERT INTO egress_subscriptions(name,group_name,url,interval_minutes,enabled,created_at,updated_at)
VALUES(?,?,?,?,?,?,?)`, name, group, url, interval, on, ts, ts)
	if err != nil {
		return EgressSubscription{}, err
	}
	return s.GetEgressSubscription()
}

func (s *Store) DeleteEgressSubscription(clearGroup bool) (map[string]any, error) {
	existing, err := s.GetEgressSubscription()
	if errors.Is(err, ErrNotFound) {
		return map[string]any{"ok": true, "removed": false}, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`DELETE FROM egress_subscriptions WHERE id=?`, existing.ID); err != nil {
		return nil, err
	}
	cleared := int64(0)
	if clearGroup {
		cleared, err = s.DeleteEgressByGroup(existing.GroupName)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{"ok": true, "removed": true, "cleared": cleared}, nil
}

func (s *Store) MarkEgressSubscriptionSync(id int64, status, errMsg string, stats SyncStats) error {
	ts := now()
	_, err := s.db.Exec(`
UPDATE egress_subscriptions SET
  last_sync_at=?, last_sync_status=?, last_error=?,
  last_added=?, last_updated=?, last_removed=?, last_total=?,
  updated_at=?
WHERE id=?`,
		ts, status, truncate(errMsg, 500),
		stats.Added, stats.Updated, stats.Removed, stats.Total,
		ts, id,
	)
	return err
}

// ListDueEgressSubscriptions returns enabled subscriptions whose interval elapsed.
func (s *Store) ListDueEgressSubscriptions(nowTs time.Time) ([]EgressSubscription, error) {
	list, err := s.ListEgressSubscriptions()
	if err != nil {
		return nil, err
	}
	out := make([]EgressSubscription, 0)
	for _, sub := range list {
		if !sub.Enabled || strings.TrimSpace(sub.URL) == "" {
			continue
		}
		interval := time.Duration(normalizeSubInterval(sub.IntervalMinutes)) * time.Minute
		if sub.LastSyncAt == "" {
			out = append(out, sub)
			continue
		}
		last, e := time.Parse(time.RFC3339, sub.LastSyncAt)
		if e != nil {
			// sqlite datetime('now') style
			last, e = time.ParseInLocation("2006-01-02 15:04:05", sub.LastSyncAt, time.Local)
		}
		if e != nil || nowTs.Sub(last) >= interval {
			out = append(out, sub)
		}
	}
	return out, nil
}
