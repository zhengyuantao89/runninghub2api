package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// EgressNode mirrors gpt-plus proxies.toItem shape (single-tenant, password plaintext).
type EgressNode struct {
	ID               int64  `json:"id"`
	GroupName        string `json:"group_name"`
	Name             string `json:"name"`
	ProxyType        string `json:"proxy_type"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Username         string `json:"username"`
	Password         string `json:"password,omitempty"`
	HasPassword      bool   `json:"has_password"`
	Status           string `json:"status"`
	Latency          *int   `json:"latency"`
	Region           string `json:"region"`
	Remark           string `json:"remark"`
	UsageCount       int    `json:"usage_count"`
	SuccessCount     int    `json:"success_count"`
	FailureCount     int    `json:"failure_count"`
	MaxFailureCount  int    `json:"max_failure_count"`
	DisabledReason   string `json:"disabled_reason"`
	LastUsedAt       string `json:"last_used_at"`
	LastSuccessAt    string `json:"last_success_at"`
	LastFailureAt    string `json:"last_failure_at"`
	ExternalID       string `json:"external_id,omitempty"`
	Source           string `json:"source,omitempty"`
	CreatedAt        string `json:"created_at"`
}

func (n EgressNode) Label() string {
	if n.Host == "" {
		return ""
	}
	return n.Host + ":" + strconv.Itoa(n.Port)
}

const egressSelect = `SELECT id,group_name,name,proxy_type,host,port,username,password,status,latency,region,remark,
usage_count,success_count,failure_count,max_failure_count,disabled_reason,last_used_at,last_success_at,last_failure_at,
external_id,source,created_at FROM egress_nodes`

func (s *Store) scanEgress(row rowScanner) (EgressNode, error) {
	var n EgressNode
	var latency sql.NullInt64
	err := row.Scan(
		&n.ID, &n.GroupName, &n.Name, &n.ProxyType, &n.Host, &n.Port, &n.Username, &n.Password,
		&n.Status, &latency, &n.Region, &n.Remark,
		&n.UsageCount, &n.SuccessCount, &n.FailureCount, &n.MaxFailureCount, &n.DisabledReason,
		&n.LastUsedAt, &n.LastSuccessAt, &n.LastFailureAt, &n.ExternalID, &n.Source, &n.CreatedAt,
	)
	if err != nil {
		return n, err
	}
	if latency.Valid {
		v := int(latency.Int64)
		n.Latency = &v
	}
	n.HasPassword = strings.TrimSpace(n.Password) != ""
	return n, nil
}

func (s *Store) GetEgress(id int64) (EgressNode, error) {
	n, err := s.scanEgress(s.db.QueryRow(egressSelect+` WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return EgressNode{}, ErrNotFound
	}
	return n, err
}

func (s *Store) ListEgress(group string) ([]EgressNode, error) {
	group = strings.TrimSpace(group)
	var rows *sql.Rows
	var err error
	if group != "" {
		rows, err = s.db.Query(egressSelect+` WHERE group_name=? ORDER BY id DESC`, group)
	} else {
		rows, err = s.db.Query(egressSelect + ` ORDER BY id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]EgressNode, 0)
	for rows.Next() {
		n, err := s.scanEgress(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) ListEgressGroups() ([]map[string]any, error) {
	rows, err := s.db.Query(`SELECT group_name, COUNT(*) AS count FROM egress_nodes GROUP BY group_name ORDER BY group_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"group_name": name, "count": count})
	}
	return out, rows.Err()
}

// PickAvailableEgress returns a usable node, preferring least recently used.
// excludeID skips a failed node when rebinding.
func (s *Store) PickAvailableEgress(excludeID int64) (EgressNode, error) {
	q := egressSelect + ` WHERE status='可用'`
	args := []any{}
	if excludeID > 0 {
		q += ` AND id!=?`
		args = append(args, excludeID)
	}
	q += ` ORDER BY CASE WHEN last_used_at='' THEN 0 ELSE 1 END, last_used_at ASC, id ASC LIMIT 1`
	n, err := s.scanEgress(s.db.QueryRow(q, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return EgressNode{}, ErrNotFound
	}
	return n, err
}

func normalizeProxyType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "socks5", "socks", "socks5h":
		return "socks5"
	case "https":
		return "https"
	default:
		if t == "" {
			return "http"
		}
		return "http"
	}
}

func normalizeEgressStatus(st string) string {
	st = strings.TrimSpace(st)
	if st == "" {
		return "可用"
	}
	low := strings.ToLower(st)
	switch low {
	case "ok", "available", "active", "up":
		return "可用"
	case "fail", "failed", "down", "error", "dead":
		return "失败"
	case "disabled", "off":
		return "禁用"
	}
	return st
}

// UpsertEgress inserts or updates by external_id (if set) else (proxy_type,host,port,username).
func (s *Store) UpsertEgress(n EgressNode) (EgressNode, error) {
	n.Host = strings.TrimSpace(n.Host)
	if n.Host == "" || n.Port <= 0 {
		return EgressNode{}, fmt.Errorf("host and port required")
	}
	n.ProxyType = normalizeProxyType(n.ProxyType)
	n.GroupName = strings.TrimSpace(n.GroupName)
	if n.GroupName == "" {
		n.GroupName = "默认分组"
	}
	n.Name = strings.TrimSpace(n.Name)
	n.Username = strings.TrimSpace(n.Username)
	n.Status = normalizeEgressStatus(n.Status)
	n.Region = strings.TrimSpace(n.Region)
	n.Remark = strings.TrimSpace(n.Remark)
	n.ExternalID = strings.TrimSpace(n.ExternalID)
	n.Source = strings.TrimSpace(n.Source)
	if n.MaxFailureCount <= 0 {
		n.MaxFailureCount = 3
	}

	var existingID int64
	if n.ExternalID != "" {
		_ = s.db.QueryRow(`SELECT id FROM egress_nodes WHERE external_id=? LIMIT 1`, n.ExternalID).Scan(&existingID)
	}
	if existingID == 0 {
		_ = s.db.QueryRow(
			`SELECT id FROM egress_nodes WHERE proxy_type=? AND host=? AND port=? AND username=? LIMIT 1`,
			n.ProxyType, n.Host, n.Port, n.Username,
		).Scan(&existingID)
	}

	if existingID > 0 {
		_, err := s.db.Exec(`
UPDATE egress_nodes SET
  group_name=?, name=?, proxy_type=?, host=?, port=?, username=?,
  password=CASE WHEN ?='' THEN password ELSE ? END,
  status=?, region=?, remark=?,
  max_failure_count=?,
  external_id=CASE WHEN ?='' THEN external_id ELSE ? END,
  source=CASE WHEN ?='' THEN source ELSE ? END
WHERE id=?`,
			n.GroupName, n.Name, n.ProxyType, n.Host, n.Port, n.Username,
			n.Password, n.Password,
			n.Status, n.Region, n.Remark,
			n.MaxFailureCount,
			n.ExternalID, n.ExternalID,
			n.Source, n.Source,
			existingID,
		)
		if err != nil {
			return EgressNode{}, err
		}
		return s.GetEgress(existingID)
	}

	res, err := s.db.Exec(`
INSERT INTO egress_nodes(
  group_name,name,proxy_type,host,port,username,password,status,region,remark,
  max_failure_count,external_id,source
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		n.GroupName, n.Name, n.ProxyType, n.Host, n.Port, n.Username, n.Password, n.Status, n.Region, n.Remark,
		n.MaxFailureCount, n.ExternalID, n.Source,
	)
	if err != nil {
		return EgressNode{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetEgress(id)
}

func (s *Store) DeleteEgress(id int64) error {
	res, err := s.db.Exec(`DELETE FROM egress_nodes WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	_, _ = s.db.Exec(`UPDATE accounts SET egress_id=0, updated_at=? WHERE egress_id=?`, now(), id)
	return nil
}

func (s *Store) SetEgressStatus(id int64, status, reason string) error {
	status = normalizeEgressStatus(status)
	_, err := s.db.Exec(`UPDATE egress_nodes SET status=?, disabled_reason=? WHERE id=?`, status, truncate(reason, 500), id)
	return err
}

func (s *Store) MarkEgressUsed(id int64) error {
	ts := now()
	_, err := s.db.Exec(`UPDATE egress_nodes SET usage_count=usage_count+1, last_used_at=? WHERE id=?`, ts, id)
	return err
}

func (s *Store) MarkEgressSuccess(id int64, latencyMs int) error {
	ts := now()
	_, err := s.db.Exec(`
UPDATE egress_nodes SET success_count=success_count+1, last_success_at=?, latency=?, status='可用', disabled_reason='', failure_count=0
WHERE id=?`, ts, latencyMs, id)
	return err
}

func (s *Store) MarkEgressFailure(id int64, reason string) (EgressNode, error) {
	ts := now()
	_, err := s.db.Exec(`
UPDATE egress_nodes SET
  failure_count=failure_count+1,
  last_failure_at=?,
  disabled_reason=?
WHERE id=?`, ts, truncate(reason, 500), id)
	if err != nil {
		return EgressNode{}, err
	}
	n, err := s.GetEgress(id)
	if err != nil {
		return n, err
	}
	maxFail := n.MaxFailureCount
	if maxFail <= 0 {
		maxFail = 3
	}
	if n.FailureCount >= maxFail {
		_ = s.SetEgressStatus(id, "失败", reason)
		n.Status = "失败"
	}
	return n, nil
}

func (s *Store) UpdateEgressProbe(id int64, ok bool, latencyMs int, errMsg string) error {
	if ok {
		return s.MarkEgressSuccess(id, latencyMs)
	}
	_, err := s.MarkEgressFailure(id, errMsg)
	return err
}

func (s *Store) DeleteEgressByGroup(group string) (int64, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return 0, fmt.Errorf("group required")
	}
	rows, err := s.db.Query(`SELECT id FROM egress_nodes WHERE group_name=?`, group)
	if err != nil {
		return 0, err
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	res, err := s.db.Exec(`DELETE FROM egress_nodes WHERE group_name=?`, group)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	for _, id := range ids {
		_, _ = s.db.Exec(`UPDATE accounts SET egress_id=0, updated_at=? WHERE egress_id=?`, now(), id)
	}
	return n, nil
}

// SyncStats is the result of exact group sync (subscription).
type SyncStats struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Removed int `json:"removed"`
	Total   int `json:"total"`
}

// ExactSyncEgressGroup mirrors gpt-plus subscription exact sync for a fixed group.
func (s *Store) ExactSyncEgressGroup(group string, desired []EgressNode, fingerprint func(EgressNode) string) (SyncStats, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		group = "nodes"
	}
	existing, err := s.ListEgress(group)
	if err != nil {
		return SyncStats{}, err
	}
	byFP := map[string]EgressNode{}
	for _, row := range existing {
		byFP[fingerprint(row)] = row
	}
	want := map[string]EgressNode{}
	for _, row := range desired {
		row.GroupName = group
		if row.ProxyType == "" {
			row.ProxyType = "http"
		}
		if row.Status == "" {
			row.Status = "可用"
		}
		if row.Source == "" {
			row.Source = "subscription"
		}
		if row.Remark == "" {
			row.Remark = "subscription:" + group
		}
		fp := fingerprint(row)
		if _, ok := want[fp]; !ok {
			want[fp] = row
		}
	}

	stats := SyncStats{Total: len(want)}
	keep := map[int64]struct{}{}

	for fp, row := range want {
		prev, ok := byFP[fp]
		if !ok {
			n, e := s.UpsertEgress(row)
			if e != nil {
				return stats, e
			}
			keep[n.ID] = struct{}{}
			stats.Added++
			continue
		}
		keep[prev.ID] = struct{}{}
		needUpdate := prev.Password != row.Password ||
			prev.ProxyType != row.ProxyType ||
			prev.Remark != row.Remark ||
			prev.Source != row.Source
		if !needUpdate {
			continue
		}
		status := prev.Status
		if status != "失败" {
			status = "可用"
		}
		_, err := s.db.Exec(`
UPDATE egress_nodes SET
  proxy_type=?, host=?, port=?, username=?, password=?,
  status=?, remark=?, source=?,
  disabled_reason=CASE WHEN status='失败' THEN disabled_reason ELSE '' END
WHERE id=?`,
			row.ProxyType, row.Host, row.Port, row.Username, row.Password,
			status, row.Remark, row.Source, prev.ID,
		)
		if err != nil {
			return stats, err
		}
		stats.Updated++
	}

	for _, row := range existing {
		if _, ok := keep[row.ID]; ok {
			continue
		}
		if err := s.DeleteEgress(row.ID); err != nil {
			return stats, err
		}
		stats.Removed++
	}
	return stats, nil
}
