package store

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrAccountBusy = errors.New("account busy")
)

type Account struct {
	ID                int64  `json:"id"`
	UserID            int64  `json:"user_id"`
	Owner             string `json:"owner,omitempty"` // console username; filled by API
	Email             string `json:"email"`
	Password          string `json:"password,omitempty"`
	AccessToken       string `json:"access_token,omitempty"`
	RefreshToken      string `json:"refresh_token,omitempty"`
	ATExpiresAt       string `json:"at_expires_at,omitempty"` // RFC3339 from JWT exp; empty if unknown
	APIKey            string `json:"api_key,omitempty"`
	Identify          string `json:"identify,omitempty"`
	RHUserID          string `json:"rh_user_id,omitempty"`
	RemainCoins       string `json:"remain_coins,omitempty"`  // 旧体系 RH 币
	RemainPoints      string `json:"remain_points,omitempty"` // 新体系积分 (POINT)
	FreeImageRemaining int   `json:"free_image_remaining"`
	FreeImageTotal     int   `json:"free_image_total"`
	FreeVideoRemaining int   `json:"free_video_remaining"`
	FreeVideoTotal     int   `json:"free_video_total"`
	LastFreeQuotaAt   string `json:"last_free_quota_at,omitempty"`
	CurrentTaskCounts string `json:"current_task_counts,omitempty"` // RH 侧上报，仅展示
	APIType           string `json:"api_type,omitempty"`
	BillingVersion    string `json:"billing_version,omitempty"` // NEW | OLD | empty=unknown
	LastBalanceAt     string `json:"last_balance_at,omitempty"`
	LastLoginCoinAt   string `json:"last_login_coin_at,omitempty"` // Asia/Shanghai date YYYY-MM-DD
	AnonymousID       string `json:"anonymous_id,omitempty"`
	DeviceID          string `json:"device_id,omitempty"`
	FingerprintJSON   string `json:"fingerprint_json,omitempty"`
	NickName          string `json:"nick_name,omitempty"`
	EgressID          int64  `json:"egress_id,omitempty"`
	EgressLabel       string `json:"egress_label,omitempty"` // host:port for list/UI
	MaxConcurrency    int    `json:"max_concurrency"`          // 本网关每号并发上限，默认 1
	Inflight          int    `json:"inflight"`                 // 本网关已占用槽位数
	DispatchPaused    bool   `json:"dispatch_paused"`          // 手动暂停调度；status 仍可为 active
	Status            string `json:"status"`
	Source            string `json:"source"`
	ExternalID        string `json:"external_id,omitempty"`
	CooldownUntil      string `json:"cooldown_until,omitempty"`
	LastSubmitAt      string `json:"last_submit_at,omitempty"`
	FailCount         int    `json:"fail_count"`
	LastError         string `json:"last_error,omitempty"`
	LeaseTaskID       string `json:"lease_task_id,omitempty"`
	LeaseUntil        string `json:"lease_until,omitempty"`
	Busy              bool   `json:"busy"`
	HasAPIKey         bool   `json:"has_api_key"`
	HasAccessToken    bool   `json:"has_access_token"`
	HasRefreshToken   bool   `json:"has_refresh_token"`
	HasFingerprint    bool   `json:"has_fingerprint"`
	Dispatchable      bool   `json:"dispatchable"`
	UpdatedAt         string `json:"updated_at"`
	CreatedAt         string `json:"created_at"`
}

// MaxSlots returns the effective per-account concurrency cap (>=1).
func (a Account) MaxSlots() int {
	if a.MaxConcurrency < 1 {
		return 1
	}
	return a.MaxConcurrency
}

type VideoTask struct {
	ID           string `json:"id"`
	UserID       int64  `json:"user_id"`
	Owner        string `json:"owner,omitempty"` // console username; filled by API
	Status       string `json:"status"`
	Prompt       string `json:"prompt"`
	ImagePath    string `json:"image_path,omitempty"`
	WebappID     string `json:"webapp_id"`
	TemplateID   string `json:"template_id,omitempty"`
	Duration     string `json:"duration,omitempty"`
	AspectRatio  string `json:"aspect_ratio,omitempty"`
	Resolution   string `json:"resolution,omitempty"`
	NodeInfoJSON string `json:"node_info_json,omitempty"`
	AccountID    int64  `json:"account_id,omitempty"`
	AccountEmail string `json:"account_email,omitempty"`
	RHTaskID     string `json:"rh_task_id,omitempty"`
	RHStatus     string `json:"rh_status,omitempty"`
	QueueIndex   int    `json:"queue_index,omitempty"`
	QueueCount   int    `json:"queue_count,omitempty"`
	ProgressText string `json:"progress_text,omitempty"`
	FileURL      string `json:"file_url,omitempty"`
	PreviewURL   string `json:"preview_url,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	Error        string `json:"error,omitempty"`
	Attempts     int    `json:"attempts"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	FinishedAt   string `json:"finished_at,omitempty"`
}

type Store struct {
	db *sql.DB
}

func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "rh2api.db")
	// Single writer: MaxOpenConns(1) avoids SQLITE_BUSY under Docker bind mounts on Windows.
	// busy_timeout gives writers time to wait instead of failing immediately.
	dsn := dbPath + "?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// WAL + busy_timeout allow a small pool. MaxOpenConns(1) deadlocks if any
	// query holds Rows open while calling another DB method (e.g. list+GetEgress).
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(-1)
	if _, err := db.Exec(`PRAGMA busy_timeout=30000`); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS accounts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password TEXT NOT NULL DEFAULT '',
  access_token TEXT NOT NULL DEFAULT '',
  refresh_token TEXT NOT NULL DEFAULT '',
  identify TEXT NOT NULL DEFAULT '',
  rh_user_id TEXT NOT NULL DEFAULT '',
  remain_coins TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  source TEXT NOT NULL DEFAULT 'manual',
  external_id TEXT NOT NULL DEFAULT '',
  cooldown_until TEXT NOT NULL DEFAULT '',
  fail_count INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  lease_task_id TEXT NOT NULL DEFAULT '',
  lease_until TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS video_tasks (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  prompt TEXT NOT NULL DEFAULT '',
  image_path TEXT NOT NULL DEFAULT '',
  webapp_id TEXT NOT NULL DEFAULT '',
  duration TEXT NOT NULL DEFAULT '8',
  aspect_ratio TEXT NOT NULL DEFAULT '',
  account_id INTEGER NOT NULL DEFAULT 0,
  account_email TEXT NOT NULL DEFAULT '',
  rh_task_id TEXT NOT NULL DEFAULT '',
  file_url TEXT NOT NULL DEFAULT '',
  preview_url TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  attempts INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  finished_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_video_tasks_status ON video_tasks(status, created_at);
CREATE INDEX IF NOT EXISTS idx_accounts_status ON accounts(status, cooldown_until);
CREATE TABLE IF NOT EXISTS app_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS user_settings (
  user_id INTEGER NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (user_id, key)
);
CREATE INDEX IF NOT EXISTS idx_user_settings_user ON user_settings(user_id);
`)
	if err != nil {
		return err
	}
	for _, col := range []struct{ name, ddl string }{
		{"api_key", "api_key TEXT NOT NULL DEFAULT ''"},
		{"current_task_counts", "current_task_counts TEXT NOT NULL DEFAULT ''"},
		{"api_type", "api_type TEXT NOT NULL DEFAULT ''"},
		{"last_balance_at", "last_balance_at TEXT NOT NULL DEFAULT ''"},
		{"anonymous_id", "anonymous_id TEXT NOT NULL DEFAULT ''"},
		{"device_id", "device_id TEXT NOT NULL DEFAULT ''"},
		{"fingerprint_json", "fingerprint_json TEXT NOT NULL DEFAULT ''"},
		{"last_login_coin_at", "last_login_coin_at TEXT NOT NULL DEFAULT ''"},
		{"egress_id", "egress_id INTEGER NOT NULL DEFAULT 0"},
		{"max_concurrency", "max_concurrency INTEGER NOT NULL DEFAULT 1"},
		{"inflight", "inflight INTEGER NOT NULL DEFAULT 0"},
		{"last_submit_at", "last_submit_at TEXT NOT NULL DEFAULT ''"},
		{"user_id", "user_id INTEGER NOT NULL DEFAULT 0"},
		{"at_expires_at", "at_expires_at TEXT NOT NULL DEFAULT ''"},
		{"billing_version", "billing_version TEXT NOT NULL DEFAULT ''"},
		{"remain_points", "remain_points TEXT NOT NULL DEFAULT ''"},
		{"free_image_remaining", "free_image_remaining INTEGER NOT NULL DEFAULT 0"},
		{"free_image_total", "free_image_total INTEGER NOT NULL DEFAULT 0"},
		{"free_video_remaining", "free_video_remaining INTEGER NOT NULL DEFAULT 0"},
		{"free_video_total", "free_video_total INTEGER NOT NULL DEFAULT 0"},
		{"last_free_quota_at", "last_free_quota_at TEXT NOT NULL DEFAULT ''"},
		{"nick_name", "nick_name TEXT NOT NULL DEFAULT ''"},
		{"dispatch_paused", "dispatch_paused INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := s.addColumnIfMissing("accounts", col.name, col.ddl); err != nil {
			return err
		}
	}
	// 旧「手动禁用」迁成：status=active + dispatch_paused=1（NO_COINS 仍保持 disabled）
	_, _ = s.db.Exec(`
UPDATE accounts SET dispatch_paused=1, status='active', last_error='', updated_at=?
WHERE status='disabled'
  AND (last_error IS NULL OR trim(last_error)='' OR last_error NOT LIKE 'NO_COINS%')
  AND dispatch_paused=0`, now())
	s.backfillATExpiresAt()
	// 旧数据：若仍占用租约但 inflight=0，补 1，避免升级后立刻重复派发
	_, _ = s.db.Exec(`UPDATE accounts SET inflight=1 WHERE inflight=0 AND lease_task_id!='' AND lease_until!=''`)
	// RH 业务失败不应冷却账号；FREQUENT_OPER 是登录限流误冷却，一并清掉
	_, _ = s.db.Exec(`UPDATE accounts SET cooldown_until='' WHERE cooldown_until!='' AND (
		last_error LIKE '%FREQUENT%' OR
		(last_error NOT LIKE 'AUTH:%' AND last_error NOT LIKE 'LOGIN:%')
	)`)
	// 调度余额：RH币与积分任一可用即可。修复「NEW 积分=0 但 RH币仍有」被误禁的号。
	ts := now()
	_, _ = s.db.Exec(`
UPDATE accounts SET status='active', last_error='', updated_at=?
WHERE status='disabled'
  AND last_error LIKE 'NO_COINS%'
  AND (
    CAST(CASE
      WHEN trim(remain_coins)='' THEN '0'
      WHEN instr(remain_coins,'.')>0 THEN substr(remain_coins,1,instr(remain_coins,'.')-1)
      ELSE remain_coins
    END AS INTEGER) > 0
    OR
    CAST(CASE
      WHEN trim(remain_points)='' THEN '0'
      WHEN instr(remain_points,'.')>0 THEN substr(remain_points,1,instr(remain_points,'.')-1)
      ELSE remain_points
    END AS INTEGER) > 0
  )`, ts)
	// 两边都为 0 的活跃号才禁用（仍有 RH币 的不禁，留给上游拒绝）
	_, _ = s.db.Exec(`
UPDATE accounts SET status='disabled', last_error=?, updated_at=?,
  inflight=0, lease_task_id='', lease_until='', cooldown_until=''
WHERE status='active'
  AND last_balance_at!=''
  AND CAST(CASE
        WHEN trim(remain_coins)='' THEN '0'
        WHEN instr(remain_coins,'.')>0 THEN substr(remain_coins,1,instr(remain_coins,'.')-1)
        ELSE remain_coins
      END AS INTEGER) <= 0
  AND CAST(CASE
        WHEN trim(remain_points)='' THEN '0'
        WHEN instr(remain_points,'.')>0 THEN substr(remain_points,1,instr(remain_points,'.')-1)
        ELSE remain_points
      END AS INTEGER) <= 0`, "NO_COINS: coins=0 points=0", ts)
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS egress_nodes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  group_name TEXT NOT NULL DEFAULT '默认分组',
  name TEXT NOT NULL DEFAULT '',
  proxy_type TEXT NOT NULL DEFAULT 'http',
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  username TEXT NOT NULL DEFAULT '',
  password TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '可用',
  latency INTEGER,
  region TEXT NOT NULL DEFAULT '',
  remark TEXT NOT NULL DEFAULT '',
  usage_count INTEGER NOT NULL DEFAULT 0,
  success_count INTEGER NOT NULL DEFAULT 0,
  failure_count INTEGER NOT NULL DEFAULT 0,
  max_failure_count INTEGER NOT NULL DEFAULT 3,
  disabled_reason TEXT NOT NULL DEFAULT '',
  last_used_at TEXT NOT NULL DEFAULT '',
  last_success_at TEXT NOT NULL DEFAULT '',
  last_failure_at TEXT NOT NULL DEFAULT '',
  external_id TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_egress_nodes_status ON egress_nodes(status);
CREATE INDEX IF NOT EXISTS idx_egress_nodes_host ON egress_nodes(proxy_type, host, port, username);
CREATE INDEX IF NOT EXISTS idx_egress_nodes_external ON egress_nodes(external_id);
CREATE TABLE IF NOT EXISTS egress_subscriptions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL DEFAULT 'nodes',
  group_name TEXT NOT NULL DEFAULT 'nodes',
  url TEXT NOT NULL DEFAULT '',
  interval_minutes INTEGER NOT NULL DEFAULT 30,
  enabled INTEGER NOT NULL DEFAULT 1,
  last_sync_at TEXT NOT NULL DEFAULT '',
  last_sync_status TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  last_added INTEGER NOT NULL DEFAULT 0,
  last_updated INTEGER NOT NULL DEFAULT 0,
  last_removed INTEGER NOT NULL DEFAULT 0,
  last_total INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`); err != nil {
		return err
	}
	for _, col := range []struct{ name, ddl string }{
		{"rh_status", "rh_status TEXT NOT NULL DEFAULT ''"},
		{"queue_index", "queue_index INTEGER NOT NULL DEFAULT 0"},
		{"queue_count", "queue_count INTEGER NOT NULL DEFAULT 0"},
		{"progress_text", "progress_text TEXT NOT NULL DEFAULT ''"},
		{"node_info_json", "node_info_json TEXT NOT NULL DEFAULT ''"},
		{"template_id", "template_id TEXT NOT NULL DEFAULT ''"},
		{"user_id", "user_id INTEGER NOT NULL DEFAULT 0"},
		{"resolution", "resolution TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.addColumnIfMissing("video_tasks", col.name, col.ddl); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS webapp_templates (
  id TEXT PRIMARY KEY,
  webapp_id TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  node_info_json TEXT NOT NULL DEFAULT '[]',
  example_json TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_webapp_templates_updated ON webapp_templates(updated_at DESC);
`); err != nil {
		return err
	}
	// 历史来源标签归一为 sync（旧外部同步标签）
	legacySrc := strings.ToLower("GPT" + "-" + "PLUS")
	if _, err := s.db.Exec(`UPDATE accounts SET source='sync' WHERE lower(source)=? OR lower(replace(source,'_',''))=?`, legacySrc, strings.ReplaceAll(legacySrc, "-", "")); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS webapp_templates (
  id TEXT PRIMARY KEY,
  webapp_id TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  node_info_json TEXT NOT NULL DEFAULT '[]',
  example_json TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_webapp_templates_updated ON webapp_templates(updated_at);
`); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("webapp_templates", "access_password", "access_password TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("webapp_templates", "listed", "listed INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := s.ensureUsersSchema(); err != nil {
		return err
	}
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_accounts_user ON accounts(user_id, status)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_video_tasks_user ON video_tasks(user_id, status, created_at)`)
	if err := s.migrateSessions(); err != nil {
		return err
	}
	return nil
}

func (s *Store) addColumnIfMissing(table, name, ddl string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var colName, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &colName, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if colName == name {
			return nil
		}
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + ddl)
	return err
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

const accountSelect = `SELECT id,user_id,email,password,access_token,refresh_token,at_expires_at,api_key,identify,rh_user_id,remain_coins,remain_points,free_image_remaining,free_image_total,free_video_remaining,free_video_total,last_free_quota_at,current_task_counts,api_type,billing_version,last_balance_at,last_login_coin_at,anonymous_id,device_id,fingerprint_json,nick_name,egress_id,max_concurrency,inflight,dispatch_paused,status,source,external_id,cooldown_until,last_submit_at,fail_count,last_error,lease_task_id,lease_until,created_at,updated_at FROM accounts`

const taskSelect = `SELECT id,user_id,status,prompt,image_path,webapp_id,template_id,duration,aspect_ratio,resolution,node_info_json,account_id,account_email,rh_task_id,rh_status,queue_index,queue_count,progress_text,file_url,preview_url,error_code,error,attempts,created_at,updated_at,finished_at FROM video_tasks`

func (s *Store) UpsertAccount(a Account) (Account, error) {
	// Keep original email casing for display/login; UNIQUE COLLATE NOCASE still de-dupes.
	a.Email = strings.TrimSpace(a.Email)
	if a.Email == "" {
		return Account{}, fmt.Errorf("email required")
	}
	if a.Status == "" {
		a.Status = "active"
	}
	if a.Source == "" {
		a.Source = "manual"
	}
	setMax := a.MaxConcurrency > 0
	maxConc := a.MaxConcurrency
	if maxConc < 1 {
		maxConc = 1
	}
	if maxConc > 64 {
		maxConc = 64
	}
	ts := now()
	_, err := s.db.Exec(`
INSERT INTO accounts(user_id,email,password,access_token,refresh_token,api_key,identify,rh_user_id,remain_coins,current_task_counts,api_type,last_balance_at,anonymous_id,device_id,fingerprint_json,max_concurrency,status,source,external_id,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(email) DO UPDATE SET
  email=excluded.email,
  user_id=CASE WHEN excluded.user_id=0 THEN accounts.user_id ELSE excluded.user_id END,
  password=CASE WHEN excluded.password='' THEN accounts.password ELSE excluded.password END,
  access_token=CASE WHEN excluded.access_token='' THEN accounts.access_token ELSE excluded.access_token END,
  refresh_token=CASE WHEN excluded.refresh_token='' THEN accounts.refresh_token ELSE excluded.refresh_token END,
  api_key=CASE WHEN excluded.api_key='' THEN accounts.api_key ELSE excluded.api_key END,
  identify=CASE WHEN excluded.identify='' THEN accounts.identify ELSE excluded.identify END,
  rh_user_id=CASE WHEN excluded.rh_user_id='' THEN accounts.rh_user_id ELSE excluded.rh_user_id END,
  remain_coins=CASE WHEN excluded.remain_coins='' THEN accounts.remain_coins ELSE excluded.remain_coins END,
  current_task_counts=CASE WHEN excluded.current_task_counts='' THEN accounts.current_task_counts ELSE excluded.current_task_counts END,
  api_type=CASE WHEN excluded.api_type='' THEN accounts.api_type ELSE excluded.api_type END,
  last_balance_at=CASE WHEN excluded.last_balance_at='' THEN accounts.last_balance_at ELSE excluded.last_balance_at END,
  anonymous_id=CASE WHEN excluded.anonymous_id='' THEN accounts.anonymous_id ELSE excluded.anonymous_id END,
  device_id=CASE WHEN excluded.device_id='' THEN accounts.device_id ELSE excluded.device_id END,
  fingerprint_json=CASE WHEN excluded.fingerprint_json='' THEN accounts.fingerprint_json ELSE excluded.fingerprint_json END,
  max_concurrency=CASE WHEN ? THEN excluded.max_concurrency ELSE accounts.max_concurrency END,
  status=excluded.status,
  source=excluded.source,
  external_id=CASE WHEN excluded.external_id='' THEN accounts.external_id ELSE excluded.external_id END,
  updated_at=excluded.updated_at
`, a.UserID, a.Email, a.Password, a.AccessToken, a.RefreshToken, a.APIKey, a.Identify, a.RHUserID, a.RemainCoins, a.CurrentTaskCounts, a.APIType, a.LastBalanceAt, a.AnonymousID, a.DeviceID, a.FingerprintJSON, maxConc, a.Status, a.Source, a.ExternalID, ts, ts, setMax)
	if err != nil {
		return Account{}, err
	}
	acc, err := s.GetAccountByEmail(a.Email)
	if err != nil {
		return Account{}, err
	}
	if at := strings.TrimSpace(a.AccessToken); at != "" {
		if exp := jwtExpiresAtRFC3339(at); exp != "" && exp != acc.ATExpiresAt {
			_, _ = s.db.Exec(`UPDATE accounts SET at_expires_at=? WHERE id=?`, exp, acc.ID)
			acc.ATExpiresAt = exp
		}
	}
	return acc, nil
}

func (s *Store) GetAccountByEmail(email string) (Account, error) {
	acc, err := s.scanAccount(s.db.QueryRow(accountSelect+` WHERE email=? COLLATE NOCASE`, email))
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	return acc, err
}

func (s *Store) GetAccount(id int64) (Account, error) {
	acc, err := s.scanAccount(s.db.QueryRow(accountSelect+` WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return acc, err
	}
	if acc.EgressID > 0 {
		if n, eerr := s.GetEgress(acc.EgressID); eerr == nil {
			acc.EgressLabel = n.Label()
		}
	}
	return acc, nil
}

// FindAccountWithAPIKey returns one account that has a consumer api_key (for RH official demos).
func (s *Store) FindAccountWithAPIKey() (Account, error) {
	row := s.db.QueryRow(accountSelect + ` WHERE api_key != '' ORDER BY CASE status WHEN 'active' THEN 0 ELSE 1 END, id ASC LIMIT 1`)
	acc, err := s.scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	return acc, err
}

// ListAccounts returns accounts. userID=0 means all (admin).
func (s *Store) ListAccounts(userID int64) ([]Account, error) {
	return s.ListAccountsPage(userID, 0, "", "", "", 0, 0)
}

// spendableBalanceSQLExpr is true when RH币 or 积分 is > 0 (mirrors SpendableBalance).
const spendableBalanceSQLExpr = `(
  CAST(CASE
    WHEN trim(IFNULL(remain_coins,''))='' THEN '0'
    WHEN instr(remain_coins,'.')>0 THEN substr(remain_coins,1,instr(remain_coins,'.')-1)
    ELSE remain_coins
  END AS INTEGER) > 0
  OR
  CAST(CASE
    WHEN trim(IFNULL(remain_points,''))='' THEN '0'
    WHEN instr(remain_points,'.')>0 THEN substr(remain_points,1,instr(remain_points,'.')-1)
    ELSE remain_points
  END AS INTEGER) > 0
)`

// accountDispatchableSQL matches Account.decoratePublic Dispatchable (nowTs = RFC3339 UTC).
func accountDispatchableSQL(nowTs string) (string, []any) {
	return `(
  status='active'
  AND IFNULL(dispatch_paused,0)=0
  AND inflight < CASE WHEN IFNULL(max_concurrency,0) < 1 THEN 1 ELSE max_concurrency END
  AND (cooldown_until='' OR cooldown_until<=?)
  AND NOT (last_balance_at!='' AND NOT ` + spendableBalanceSQLExpr + `)
  AND (password!='' OR access_token!='' OR api_key!='')
)`, []any{nowTs}
}

func accountListWhere(userID, ownerUserID int64, q, status, dispatch string) (string, []any) {
	where := make([]string, 0, 6)
	args := make([]any, 0, 10)
	if userID > 0 {
		where = append(where, "user_id=?")
		args = append(args, userID)
	}
	if ownerUserID > 0 {
		where = append(where, "user_id=?")
		args = append(args, ownerUserID)
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "active" || status == "disabled" {
		where = append(where, "status=?")
		args = append(args, status)
	}
	dispatch = strings.ToLower(strings.TrimSpace(dispatch))
	nowTs := time.Now().UTC().Format(time.RFC3339)
	switch dispatch {
	case "dispatchable", "yes", "1":
		expr, dArgs := accountDispatchableSQL(nowTs)
		where = append(where, expr)
		args = append(args, dArgs...)
	case "unavailable", "no", "0":
		expr, dArgs := accountDispatchableSQL(nowTs)
		where = append(where, "NOT "+expr)
		args = append(args, dArgs...)
	case "paused":
		where = append(where, "IFNULL(dispatch_paused,0)!=0")
	case "cooldown":
		where = append(where, "cooldown_until!='' AND cooldown_until>?")
		args = append(args, nowTs)
	case "busy":
		where = append(where, "inflight>0")
	}
	q = strings.TrimSpace(q)
	if q != "" {
		like := "%" + q + "%"
		where = append(where, `(CAST(id AS TEXT) LIKE ? OR email LIKE ? OR status LIKE ? OR source LIKE ? OR external_id LIKE ? OR api_type LIKE ?)`)
		args = append(args, like, like, like, like, like, like)
	}
	if len(where) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

func (s *Store) CountAccounts(userID, ownerUserID int64, q, status, dispatch string) (int, error) {
	clause, args := accountListWhere(userID, ownerUserID, q, status, dispatch)
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM accounts`+clause, args...).Scan(&n)
	return n, err
}

// AccountQuotaStats is aggregate wallet/free-quota across a filtered account set.
type AccountQuotaStats struct {
	ImageRemaining int     `json:"image_remaining"`
	ImageTotal     int     `json:"image_total"`
	VideoRemaining int     `json:"video_remaining"`
	VideoTotal     int     `json:"video_total"`
	Points         float64 `json:"points"`
	Coins          float64 `json:"coins"`
	WithImage      int     `json:"with_image"`  // accounts with image remaining > 0
	WithVideo      int     `json:"with_video"`
	WithPoints     int     `json:"with_points"` // remain_points > 0
	Accounts       int     `json:"accounts"`
}

func (s *Store) AggregateAccountQuotaStats(userID, ownerUserID int64, q, status, dispatch string) (AccountQuotaStats, error) {
	clause, args := accountListWhere(userID, ownerUserID, q, status, dispatch)
	var st AccountQuotaStats
	err := s.db.QueryRow(`
SELECT
  COUNT(*),
  COALESCE(SUM(free_image_remaining),0),
  COALESCE(SUM(free_image_total),0),
  COALESCE(SUM(free_video_remaining),0),
  COALESCE(SUM(free_video_total),0),
  COALESCE(SUM(CASE WHEN CAST(IFNULL(NULLIF(trim(remain_points),''),'0') AS REAL) > 0 THEN CAST(remain_points AS REAL) ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN CAST(IFNULL(NULLIF(trim(remain_coins),''),'0') AS REAL) > 0 THEN CAST(remain_coins AS REAL) ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN free_image_remaining > 0 THEN 1 ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN free_video_remaining > 0 THEN 1 ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN CAST(IFNULL(NULLIF(trim(remain_points),''),'0') AS REAL) > 0 THEN 1 ELSE 0 END),0)
FROM accounts`+clause, args...).Scan(
		&st.Accounts,
		&st.ImageRemaining, &st.ImageTotal,
		&st.VideoRemaining, &st.VideoTotal,
		&st.Points, &st.Coins,
		&st.WithImage, &st.WithVideo, &st.WithPoints,
	)
	return st, err
}

// ListAccountsPage returns accounts ordered by id DESC.
// limit<=0 means no LIMIT (full list).
// status filters exact active/disabled when set; empty means all.
// dispatch filters dispatchable|unavailable|paused|cooldown|busy when set.
func (s *Store) ListAccountsPage(userID, ownerUserID int64, q, status, dispatch string, limit, offset int) ([]Account, error) {
	clause, args := accountListWhere(userID, ownerUserID, q, status, dispatch)
	sqlText := accountSelect + clause + ` ORDER BY id DESC`
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
	nowTs := time.Now().UTC()
	out := make([]Account, 0)
	for rows.Next() {
		a, err := s.scanAccountRows(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		a.decoratePublic(nowTs)
		a.Password = ""
		a.AccessToken = ""
		a.RefreshToken = ""
		a.APIKey = ""
		a.FingerprintJSON = "" // list 只回 has_fingerprint，完整指纹仅 GetAccount / 调度内用
		a.LeaseTaskID = ""
		a.LeaseUntil = ""
		out = append(out, a)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	// Fill labels after Rows are closed — nested GetEgress while Rows hold the
	// only pool conn deadlocks under low MaxOpenConns.
	s.fillAccountEgressLabels(out)
	return out, nil
}

func (s *Store) fillAccountEgressLabels(list []Account) {
	cache := make(map[int64]string)
	for i := range list {
		id := list[i].EgressID
		if id <= 0 {
			continue
		}
		if label, ok := cache[id]; ok {
			list[i].EgressLabel = label
			continue
		}
		n, err := s.GetEgress(id)
		if err != nil {
			continue
		}
		label := n.Label()
		cache[id] = label
		list[i].EgressLabel = label
	}
}

func (s *Store) reclaimExpiredLeases(nowTs time.Time) {
	_, _ = s.db.Exec(
		`UPDATE accounts SET inflight=0, lease_task_id='', lease_until='', updated_at=? WHERE lease_until!='' AND lease_until < ? AND inflight>0`,
		nowTs.Format(time.RFC3339), nowTs.Format(time.RFC3339),
	)
}

func (s *Store) ListDispatchableAccounts(userID int64, submitGap time.Duration) ([]Account, error) {
	nowTs := time.Now().UTC()
	s.reclaimExpiredLeases(nowTs)
	var (
		rows *sql.Rows
		err  error
	)
	if userID > 0 {
		rows, err = s.db.Query(accountSelect+` WHERE status='active' AND dispatch_paused=0 AND user_id=? ORDER BY updated_at ASC, id ASC`, userID)
	} else {
		rows, err = s.db.Query(accountSelect + ` WHERE status='active' AND dispatch_paused=0 ORDER BY updated_at ASC, id ASC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		a, err := s.scanAccountRows(rows)
		if err != nil {
			return nil, err
		}
		if a.DispatchPaused {
			continue
		}
		if a.Password == "" && a.AccessToken == "" {
			continue
		}
		if until, err := time.Parse(time.RFC3339, a.CooldownUntil); err == nil && until.After(nowTs) {
			continue
		}
		// 同号两次提交间隔，避免 RH 失败后立刻连打
		if submitGap > 0 && a.LastSubmitAt != "" {
			if t, err := time.Parse(time.RFC3339, a.LastSubmitAt); err == nil && t.Add(submitGap).After(nowTs) {
				continue
			}
		}
		// 本网关槽位未满才可再派
		if a.Inflight >= a.MaxSlots() {
			continue
		}
		// 已刷过余额且可消费余额为 0：无法再生成
		if a.LastBalanceAt != "" && a.SpendableBalance() <= 0 {
			continue
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (a *Account) PublicView() {
	a.decoratePublic(time.Now().UTC())
	a.Password = ""
	a.AccessToken = ""
	a.RefreshToken = ""
	a.APIKey = ""
}

func (a *Account) decoratePublic(nowTs time.Time) {
	if a.MaxConcurrency < 1 {
		a.MaxConcurrency = 1
	}
	if a.Inflight < 0 {
		a.Inflight = 0
	}
	a.HasAPIKey = strings.TrimSpace(a.APIKey) != ""
	a.HasAccessToken = strings.TrimSpace(a.AccessToken) != ""
	a.HasRefreshToken = strings.TrimSpace(a.RefreshToken) != ""
	a.HasFingerprint = strings.TrimSpace(a.FingerprintJSON) != ""
	slotsFull := a.Inflight >= a.MaxSlots()
	a.Busy = a.Inflight > 0
	cooling := false
	if until, err := time.Parse(time.RFC3339, a.CooldownUntil); err == nil && until.After(nowTs) {
		cooling = true
	}
	noCoins := a.LastBalanceAt != "" && a.SpendableBalance() <= 0
	a.Dispatchable = a.Status == "active" && !a.DispatchPaused && !slotsFull && !cooling && !noCoins && (a.Password != "" || a.AccessToken != "" || a.HasAPIKey)
}

// SpendableBalance is any usable balance for scheduling.
// Webapp templates charge RH币; Runow paid paths charge 积分.
// Having either > 0 keeps the account dispatchable — do not gate on billing_version alone.
func (a Account) SpendableBalance() int {
	coins := parseCoins(a.RemainCoins)
	points := parseCoins(a.RemainPoints)
	if coins > points {
		return coins
	}
	return points
}

func parseCoins(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// remainCoins may be float-like ("1100.0")
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func (s *Store) LeaseAccount(id int64, taskID string, ttl time.Duration) error {
	until := time.Now().UTC().Add(ttl).Format(time.RFC3339)
	ts := now()
	res, err := s.db.Exec(`
UPDATE accounts SET
  inflight = inflight + 1,
  lease_task_id = ?,
  lease_until = ?,
  last_submit_at = ?,
  updated_at = ?
WHERE id = ?
  AND inflight < CASE WHEN max_concurrency < 1 THEN 1 ELSE max_concurrency END
`, taskID, until, ts, ts, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("account %d already at concurrency limit", id)
	}
	return nil
}

func (s *Store) ReleaseAccountOK(id int64) error {
	_, err := s.db.Exec(`
UPDATE accounts SET
  inflight = CASE WHEN inflight > 0 THEN inflight - 1 ELSE 0 END,
  lease_task_id = CASE WHEN inflight <= 1 THEN '' ELSE lease_task_id END,
  lease_until = CASE WHEN inflight <= 1 THEN '' ELSE lease_until END,
  fail_count = 0,
  last_error = '',
  cooldown_until = '',
  updated_at = ?
WHERE id = ?`, now(), id)
	return err
}

func (s *Store) ReleaseAccountFail(id int64, code, msg string, cooldown time.Duration) error {
	until := ""
	if cooldown > 0 {
		until = time.Now().UTC().Add(cooldown).Format(time.RFC3339)
	}
	_, err := s.db.Exec(`
UPDATE accounts SET
  inflight = CASE WHEN inflight > 0 THEN inflight - 1 ELSE 0 END,
  lease_task_id = CASE WHEN inflight <= 1 THEN '' ELSE lease_task_id END,
  lease_until = CASE WHEN inflight <= 1 THEN '' ELSE lease_until END,
  fail_count = fail_count + 1,
  last_error = ?,
  cooldown_until = ?,
  updated_at = ?
WHERE id = ?`,
		truncate(code+": "+msg, 500), until, now(), id)
	return err
}

// SetAccountMaxConcurrency sets per-account gateway concurrency (1..64).
func (s *Store) SetAccountMaxConcurrency(id int64, maxConcurrency int) error {
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	if maxConcurrency > 64 {
		maxConcurrency = 64
	}
	res, err := s.db.Exec(`UPDATE accounts SET max_concurrency=?, updated_at=? WHERE id=?`, maxConcurrency, now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DisableAccount(id int64, reason string) error {
	_, err := s.db.Exec(`UPDATE accounts SET
  status='disabled',
  inflight=0,
  lease_task_id='',
  lease_until='',
  cooldown_until='',
  last_error=?,
  updated_at=?
WHERE id=?`, truncate(reason, 500), now(), id)
	return err
}

func (s *Store) EnableAccount(id int64) error {
	_, err := s.db.Exec(`UPDATE accounts SET status='active', last_error='', updated_at=? WHERE id=?`, now(), id)
	return err
}

// PauseAccountDispatch manually pauses task dispatch; status stays active for ops (claim/refresh/…).
func (s *Store) PauseAccountDispatch(id int64) error {
	res, err := s.db.Exec(`UPDATE accounts SET dispatch_paused=1, updated_at=? WHERE id=?`, now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ResumeAccountDispatch clears dispatch pause, cooldown, and reactivates disabled rows
// (including NO_COINS) when the user explicitly recovers the account.
func (s *Store) ResumeAccountDispatch(id int64) error {
	acc, err := s.GetAccount(id)
	if err != nil {
		return err
	}
	ts := now()
	if acc.Status == "disabled" {
		_, err = s.db.Exec(`UPDATE accounts SET dispatch_paused=0, status='active', last_error='', cooldown_until='', updated_at=? WHERE id=?`, ts, id)
	} else {
		_, err = s.db.Exec(`UPDATE accounts SET dispatch_paused=0, cooldown_until='', updated_at=? WHERE id=?`, ts, id)
	}
	return err
}

// ClearAccountCooldown clears cooldown_until so the account can be dispatched again.
func (s *Store) ClearAccountCooldown(id int64) error {
	res, err := s.db.Exec(`UPDATE accounts SET cooldown_until='', updated_at=? WHERE id=?`, now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteAccount(id int64) error {
	acc, err := s.GetAccount(id)
	if err != nil {
		return err
	}
	acc.decoratePublic(time.Now().UTC())
	if acc.Busy {
		return ErrAccountBusy
	}
	res, err := s.db.Exec(`DELETE FROM accounts WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AdminView fills derived fields but keeps secrets for admin detail.
func (a *Account) AdminView() {
	a.decoratePublic(time.Now().UTC())
}

func (s *Store) UpdateAccountToken(id int64, accessToken string) error {
	return s.UpdateAccountAuthTokens(id, accessToken, "")
}

// UpdateAccountAuthTokens updates AT and/or RT (empty fields are left unchanged).
// When accessToken is set, at_expires_at is derived from the JWT exp claim.
func (s *Store) UpdateAccountAuthTokens(id int64, accessToken, refreshToken string) error {
	accessToken = strings.TrimSpace(accessToken)
	refreshToken = strings.TrimSpace(refreshToken)
	if accessToken == "" && refreshToken == "" {
		return nil
	}
	ts := now()
	if accessToken != "" && refreshToken != "" {
		_, err := s.db.Exec(`UPDATE accounts SET access_token=?, refresh_token=?, at_expires_at=?, updated_at=? WHERE id=?`,
			accessToken, refreshToken, jwtExpiresAtRFC3339(accessToken), ts, id)
		return err
	}
	if accessToken != "" {
		_, err := s.db.Exec(`UPDATE accounts SET access_token=?, at_expires_at=?, updated_at=? WHERE id=?`,
			accessToken, jwtExpiresAtRFC3339(accessToken), ts, id)
		return err
	}
	_, err := s.db.Exec(`UPDATE accounts SET refresh_token=?, updated_at=? WHERE id=?`, refreshToken, ts, id)
	return err
}

func (s *Store) UpdateAccountBalance(id int64, remainCoins, currentTaskCounts, apiType string) error {
	return s.UpdateAccountBalanceWithBilling(id, remainCoins, currentTaskCounts, apiType, "")
}

// UpdateAccountBalanceWithBilling writes one side of the wallet by billing_version.
// remain is RH币 when OLD/unknown, 积分 when NEW. The other column is preserved.
// Empty billingVersion keeps existing version.
func (s *Store) UpdateAccountBalanceWithBilling(id int64, remain, currentTaskCounts, apiType, billingVersion string) error {
	billingVersion = strings.TrimSpace(billingVersion)
	if billingVersion == "" {
		_ = s.db.QueryRow(`SELECT billing_version FROM accounts WHERE id=?`, id).Scan(&billingVersion)
		billingVersion = strings.TrimSpace(billingVersion)
	}
	if strings.EqualFold(billingVersion, "NEW") {
		return s.UpdateAccountWallet(id, "", remain, currentTaskCounts, apiType, "NEW")
	}
	bv := billingVersion
	if bv == "" {
		bv = "OLD"
	}
	return s.UpdateAccountWallet(id, remain, "", currentTaskCounts, apiType, bv)
}

// UpdateAccountWallet writes RH币 and/或积分. Empty coin/point strings leave that column unchanged.
// Disable only when BOTH balances are 0 after the write — webapp still spends RH币.
func (s *Store) UpdateAccountWallet(id int64, remainCoins, remainPoints, currentTaskCounts, apiType, billingVersion string) error {
	ts := now()
	billingVersion = strings.TrimSpace(billingVersion)
	remainCoins = strings.TrimSpace(remainCoins)
	remainPoints = strings.TrimSpace(remainPoints)
	currentTaskCounts = strings.TrimSpace(currentTaskCounts)
	apiType = strings.TrimSpace(apiType)

	var curCoins, curPoints, curBilling, curCounts, curAPIType string
	_ = s.db.QueryRow(
		`SELECT remain_coins, remain_points, billing_version, current_task_counts, api_type FROM accounts WHERE id=?`, id,
	).Scan(&curCoins, &curPoints, &curBilling, &curCounts, &curAPIType)
	if billingVersion == "" {
		billingVersion = strings.TrimSpace(curBilling)
	}
	if remainCoins == "" {
		remainCoins = curCoins
	}
	if remainPoints == "" {
		remainPoints = curPoints
	}
	if currentTaskCounts == "" {
		currentTaskCounts = curCounts
	}
	if apiType == "" {
		apiType = curAPIType
	}
	if billingVersion == "" {
		billingVersion = "OLD"
	}

	_, err := s.db.Exec(`UPDATE accounts SET remain_coins=?, remain_points=?, current_task_counts=?, api_type=?, billing_version=?, last_balance_at=?, updated_at=? WHERE id=?`,
		remainCoins, remainPoints, currentTaskCounts, apiType, billingVersion, ts, ts, id)
	if err != nil {
		return err
	}
	a := Account{RemainCoins: remainCoins, RemainPoints: remainPoints, BillingVersion: billingVersion}
	if a.SpendableBalance() <= 0 {
		return s.DisableAccount(id, fmt.Sprintf("NO_COINS: coins=%s points=%s", remainCoins, remainPoints))
	}
	_, err = s.db.Exec(`UPDATE accounts SET status='active', last_error='', updated_at=?
WHERE id=? AND status='disabled' AND last_error LIKE 'NO_COINS%'`, ts, id)
	return err
}

func (s *Store) UpdateAccountAPIKey(id int64, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil
	}
	_, err := s.db.Exec(`UPDATE accounts SET api_key=?, updated_at=? WHERE id=?`, apiKey, now(), id)
	return err
}

func (s *Store) UpdateAccountLoginCoinAt(id int64, day string) error {
	day = strings.TrimSpace(day)
	if day == "" {
		return nil
	}
	_, err := s.db.Exec(`UPDATE accounts SET last_login_coin_at=?, updated_at=? WHERE id=?`, day, now(), id)
	return err
}

func (s *Store) UpdateAccountRHUserID(id int64, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	_, err := s.db.Exec(`UPDATE accounts SET rh_user_id=?, updated_at=? WHERE id=?`, userID, now(), id)
	return err
}

func (s *Store) UpdateAccountNickName(id int64, nickName string) error {
	nickName = strings.TrimSpace(nickName)
	_, err := s.db.Exec(`UPDATE accounts SET nick_name=?, updated_at=? WHERE id=?`, nickName, now(), id)
	return err
}

// AccountEditable is a partial update for admin account detail edits.
// Pointer / special empty-means-clear fields: omit unchanged; empty string clears password/tokens/key when ClearSecrets set via explicit flags.
type AccountEditable struct {
	Email          *string
	Password       *string
	AccessToken    *string
	RefreshToken   *string
	APIKey         *string
	NickName       *string
	Status         *string
	Source         *string
	ExternalID     *string
	MaxConcurrency *int
	EgressID       *int64
}

// UpdateAccountEditable patches local account fields. Email uniqueness is case-insensitive.
func (s *Store) UpdateAccountEditable(id int64, in AccountEditable) (Account, error) {
	cur, err := s.GetAccount(id)
	if err != nil {
		return Account{}, err
	}
	email := cur.Email
	if in.Email != nil {
		email = strings.TrimSpace(*in.Email)
		if email == "" {
			return Account{}, fmt.Errorf("email required")
		}
		if !strings.EqualFold(email, cur.Email) {
			other, oerr := s.GetAccountByEmail(email)
			if oerr == nil && other.ID != id {
				return Account{}, fmt.Errorf("邮箱已被其他账号占用")
			}
			if oerr != nil && !errors.Is(oerr, ErrNotFound) {
				return Account{}, oerr
			}
		}
	}
	password := cur.Password
	if in.Password != nil {
		password = strings.TrimSpace(*in.Password)
	}
	accessToken := cur.AccessToken
	if in.AccessToken != nil {
		accessToken = strings.TrimSpace(*in.AccessToken)
	}
	refreshToken := cur.RefreshToken
	if in.RefreshToken != nil {
		refreshToken = strings.TrimSpace(*in.RefreshToken)
	}
	apiKey := cur.APIKey
	if in.APIKey != nil {
		apiKey = strings.TrimSpace(*in.APIKey)
	}
	nickName := cur.NickName
	if in.NickName != nil {
		nickName = strings.TrimSpace(*in.NickName)
	}
	status := cur.Status
	if in.Status != nil {
		st := strings.ToLower(strings.TrimSpace(*in.Status))
		if st != "active" && st != "disabled" {
			return Account{}, fmt.Errorf("status 仅支持 active/disabled")
		}
		status = st
	}
	source := cur.Source
	if in.Source != nil {
		source = strings.TrimSpace(*in.Source)
	}
	externalID := cur.ExternalID
	if in.ExternalID != nil {
		externalID = strings.TrimSpace(*in.ExternalID)
	}
	maxConc := cur.MaxConcurrency
	if maxConc < 1 {
		maxConc = 1
	}
	if in.MaxConcurrency != nil {
		maxConc = *in.MaxConcurrency
		if maxConc < 1 || maxConc > 64 {
			return Account{}, fmt.Errorf("max_concurrency 需在 1..64")
		}
	}
	egressID := cur.EgressID
	if in.EgressID != nil {
		egressID = *in.EgressID
		if egressID < 0 {
			egressID = 0
		}
	}
	atExp := cur.ATExpiresAt
	if in.AccessToken != nil {
		if accessToken == "" {
			atExp = ""
		} else if exp := jwtExpiresAtRFC3339(accessToken); exp != "" {
			atExp = exp
		}
	}
	ts := now()
	res, err := s.db.Exec(`
UPDATE accounts SET
  email=?, password=?, access_token=?, refresh_token=?, at_expires_at=?,
  api_key=?, nick_name=?, status=?, source=?, external_id=?,
  max_concurrency=?, egress_id=?, updated_at=?
WHERE id=?`,
		email, password, accessToken, refreshToken, atExp,
		apiKey, nickName, status, source, externalID,
		maxConc, egressID, ts, id,
	)
	if err != nil {
		return Account{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Account{}, ErrNotFound
	}
	return s.GetAccount(id)
}

// FillAccountIdentityEmpty writes anonymous_id / device_id / fingerprint_json
// only where the corresponding column is currently empty.
func (s *Store) FillAccountIdentityEmpty(id int64, anonymousID, deviceID, fingerprintJSON string) (filled bool, err error) {
	anonymousID = strings.TrimSpace(anonymousID)
	deviceID = strings.TrimSpace(deviceID)
	fingerprintJSON = strings.TrimSpace(fingerprintJSON)
	if anonymousID == "" && deviceID == "" && fingerprintJSON == "" {
		return false, nil
	}
	res, err := s.db.Exec(`
UPDATE accounts SET
  anonymous_id=CASE WHEN trim(ifnull(anonymous_id,''))='' AND ?!='' THEN ? ELSE anonymous_id END,
  device_id=CASE WHEN trim(ifnull(device_id,''))='' AND ?!='' THEN ? ELSE device_id END,
  fingerprint_json=CASE WHEN trim(ifnull(fingerprint_json,''))='' AND ?!='' THEN ? ELSE fingerprint_json END,
  updated_at=?
WHERE id=?
  AND (
    (trim(ifnull(anonymous_id,''))='' AND ?!='')
    OR (trim(ifnull(device_id,''))='' AND ?!='')
    OR (trim(ifnull(fingerprint_json,''))='' AND ?!='')
  )`,
		anonymousID, anonymousID,
		deviceID, deviceID,
		fingerprintJSON, fingerprintJSON,
		now(), id,
		anonymousID, deviceID, fingerprintJSON,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListAccountIDsMissingFingerprint returns account ids with empty fingerprint_json.
// userID>0 scopes to that owner; userID==0 = all.
func (s *Store) ListAccountIDsMissingFingerprint(userID int64) ([]int64, error) {
	q := `SELECT id FROM accounts WHERE trim(ifnull(fingerprint_json,''))=''`
	args := []any{}
	if userID > 0 {
		q += ` AND user_id=?`
		args = append(args, userID)
	}
	q += ` ORDER BY id ASC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) CreateTask(t VideoTask) error {
	_, err := s.db.Exec(`INSERT INTO video_tasks(id,user_id,status,prompt,image_path,webapp_id,template_id,duration,aspect_ratio,resolution,node_info_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.UserID, t.Status, t.Prompt, t.ImagePath, t.WebappID, t.TemplateID, t.Duration, t.AspectRatio, t.Resolution, t.NodeInfoJSON, t.CreatedAt, t.UpdatedAt)
	return err
}

func (s *Store) GetTask(id string) (VideoTask, error) {
	return s.scanTask(s.db.QueryRow(taskSelect+` WHERE id=?`, id))
}

// ListRunningWithRHTask returns in-flight tasks that already have an upstream RH task id.
func (s *Store) ListRunningWithRHTask() ([]VideoTask, error) {
	rows, err := s.db.Query(taskSelect + ` WHERE status='running' AND rh_task_id!='' ORDER BY updated_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]VideoTask, 0)
	for rows.Next() {
		t, err := s.scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListRunningWithoutRHTask returns running tasks stuck before RH submit returned an id
// (e.g. process restarted mid-submit). These need requeue, not poll resume.
func (s *Store) ListRunningWithoutRHTask() ([]VideoTask, error) {
	rows, err := s.db.Query(taskSelect + ` WHERE status='running' AND (rh_task_id='' OR rh_task_id IS NULL) ORDER BY updated_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]VideoTask, 0)
	for rows.Next() {
		t, err := s.scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListTasks returns tasks. userID=0 means all (admin).
func (s *Store) ListTasks(userID int64, limit int) ([]VideoTask, error) {
	return s.ListTasksPage(userID, "", "", limit, 0)
}

func taskListWhere(userID int64, q, status string) (string, []any) {
	where := make([]string, 0, 3)
	args := make([]any, 0, 5)
	if userID > 0 {
		where = append(where, "user_id=?")
		args = append(args, userID)
	}
	q = strings.TrimSpace(q)
	if q != "" {
		like := "%" + q + "%"
		where = append(where, `(id LIKE ? OR rh_task_id LIKE ?)`)
		args = append(args, like, like)
	}
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "queued", "running", "succeeded", "failed", "cancelled":
		where = append(where, "status=?")
		args = append(args, status)
	}
	if len(where) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

func (s *Store) CountTasks(userID int64, q, status string) (int, error) {
	clause, args := taskListWhere(userID, q, status)
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM video_tasks`+clause, args...).Scan(&n)
	return n, err
}

// ListTasksPage returns tasks ordered by created_at DESC. limit<=0 defaults to 50.
// q matches local task id or rh_task_id (substring). status filters exact status when set.
func (s *Store) ListTasksPage(userID int64, q, status string, limit, offset int) ([]VideoTask, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	clause, args := taskListWhere(userID, q, status)
	args = append(args, limit, offset)
	rows, err := s.db.Query(taskSelect+clause+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]VideoTask, 0)
	for rows.Next() {
		t, err := s.scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) ClaimQueuedTask() (VideoTask, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return VideoTask{}, err
	}
	defer tx.Rollback()
	row := tx.QueryRow(`SELECT id FROM video_tasks WHERE status='queued' ORDER BY created_at ASC LIMIT 1`)
	var id string
	if err := row.Scan(&id); err != nil {
		return VideoTask{}, err
	}
	ts := now()
	if _, err := tx.Exec(`UPDATE video_tasks SET status='running', updated_at=?, attempts=attempts+1 WHERE id=? AND status='queued'`, ts, id); err != nil {
		return VideoTask{}, err
	}
	if err := tx.Commit(); err != nil {
		return VideoTask{}, err
	}
	return s.GetTask(id)
}

func (s *Store) UpdateTaskRunning(id string, accountID int64, email, rhTaskID string) error {
	// Clear prior soft errors (e.g. BUSY/423) once RH accepted the create.
	_, err := s.db.Exec(`UPDATE video_tasks SET status='running', account_id=?, account_email=?, rh_task_id=?, error='', error_code='', updated_at=? WHERE id=?`,
		accountID, email, rhTaskID, now(), id)
	return err
}

func (s *Store) UpdateTaskWebappID(id, webappID string) error {
	webappID = strings.TrimSpace(webappID)
	if id == "" || webappID == "" {
		return nil
	}
	_, err := s.db.Exec(`UPDATE video_tasks SET webapp_id=?, updated_at=? WHERE id=?`, webappID, now(), id)
	return err
}

func (s *Store) UpdateTaskProgress(id, rhStatus, progressText string, queueIndex, queueCount int) error {
	_, err := s.db.Exec(`UPDATE video_tasks SET rh_status=?, queue_index=?, queue_count=?, progress_text=?, updated_at=? WHERE id=?`,
		rhStatus, queueIndex, queueCount, progressText, now(), id)
	return err
}

func (s *Store) FinishTaskOK(id, fileURL, previewURL string) error {
	ts := now()
	_, err := s.db.Exec(`UPDATE video_tasks SET status='succeeded', file_url=?, preview_url=?, rh_status='SUCCESS', progress_text='完成', error='', error_code='', updated_at=?, finished_at=? WHERE id=?`,
		fileURL, previewURL, ts, ts, id)
	return err
}

func (s *Store) FinishTaskFail(id, code, msg string) error {
	ts := now()
	_, err := s.db.Exec(`UPDATE video_tasks SET status='failed', error_code=?, error=?, updated_at=?, finished_at=? WHERE id=?`,
		code, truncate(msg, 1000), ts, ts, id)
	return err
}

func (s *Store) RequeueTask(id, code, msg string) error {
	_, err := s.db.Exec(`UPDATE video_tasks SET status='queued', error_code=?, error=?, account_id=0, account_email='', rh_task_id='', rh_status='', queue_index=0, queue_count=0, progress_text='', updated_at=? WHERE id=?`,
		code, truncate(msg, 1000), now(), id)
	return err
}

func (s *Store) CancelTask(id string) (bool, error) {
	res, err := s.db.Exec(`UPDATE video_tasks SET status='cancelled', updated_at=?, finished_at=? WHERE id=? AND status IN ('queued','running')`, now(), now(), id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListActiveUploadPaths returns image_path values still needed by queued/running tasks.
func (s *Store) ListActiveUploadPaths() (map[string]struct{}, error) {
	rows, err := s.db.Query(`SELECT image_path FROM video_tasks WHERE status IN ('queued','running') AND image_path!=''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		p = strings.TrimSpace(p)
		if p != "" {
			out[p] = struct{}{}
		}
	}
	return out, rows.Err()
}

// ListTerminalUploadsBefore returns finished tasks with local image_path older than cutoff (RFC3339 UTC).
func (s *Store) ListTerminalUploadsBefore(cutoffRFC3339 string) ([]VideoTask, error) {
	rows, err := s.db.Query(
		taskSelect+` WHERE status IN ('succeeded','failed','cancelled') AND image_path!='' AND (
			(finished_at!='' AND finished_at < ?) OR (finished_at='' AND updated_at < ?)
		) ORDER BY updated_at ASC LIMIT 500`,
		cutoffRFC3339, cutoffRFC3339,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]VideoTask, 0)
	for rows.Next() {
		t, err := s.scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) ClearTaskImagePath(id string) error {
	_, err := s.db.Exec(`UPDATE video_tasks SET image_path='', updated_at=? WHERE id=?`, now(), id)
	return err
}

// ListTerminalTasksBefore returns finished tasks older than cutoff (for retention purge).
func (s *Store) ListTerminalTasksBefore(cutoffRFC3339 string, limit int) ([]VideoTask, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.Query(
		taskSelect+` WHERE status IN ('succeeded','failed','cancelled') AND (
			(finished_at!='' AND finished_at < ?) OR (finished_at='' AND updated_at < ?)
		) ORDER BY updated_at ASC LIMIT ?`,
		cutoffRFC3339, cutoffRFC3339, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]VideoTask, 0)
	for rows.Next() {
		t, err := s.scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteTasksByIDs removes video_tasks by id. Returns deleted count.
func (s *Store) DeleteTasksByIDs(ids []string) (int, error) {
	deleted := 0
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		res, err := s.db.Exec(`DELETE FROM video_tasks WHERE id=?`, id)
		if err != nil {
			return deleted, err
		}
		n, _ := res.RowsAffected()
		deleted += int(n)
	}
	return deleted, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanAccount(row rowScanner) (Account, error) {
	var a Account
	var paused int
	err := row.Scan(
		&a.ID, &a.UserID, &a.Email, &a.Password, &a.AccessToken, &a.RefreshToken, &a.ATExpiresAt, &a.APIKey, &a.Identify, &a.RHUserID,
		&a.RemainCoins, &a.RemainPoints,
		&a.FreeImageRemaining, &a.FreeImageTotal, &a.FreeVideoRemaining, &a.FreeVideoTotal, &a.LastFreeQuotaAt,
		&a.CurrentTaskCounts, &a.APIType, &a.BillingVersion, &a.LastBalanceAt, &a.LastLoginCoinAt,
		&a.AnonymousID, &a.DeviceID, &a.FingerprintJSON, &a.NickName, &a.EgressID, &a.MaxConcurrency, &a.Inflight,
		&paused, &a.Status, &a.Source, &a.ExternalID, &a.CooldownUntil, &a.LastSubmitAt, &a.FailCount, &a.LastError,
		&a.LeaseTaskID, &a.LeaseUntil, &a.CreatedAt, &a.UpdatedAt,
	)
	a.DispatchPaused = paused != 0
	if err == nil && a.MaxConcurrency < 1 {
		a.MaxConcurrency = 1
	}
	return a, err
}

// UpdateAccountFreeQuota writes daily free image/video remaining+total from welfare/summary.
func (s *Store) UpdateAccountFreeQuota(id int64, imageRem, imageTot, videoRem, videoTot int) error {
	if imageRem < 0 {
		imageRem = 0
	}
	if imageTot < 0 {
		imageTot = 0
	}
	if videoRem < 0 {
		videoRem = 0
	}
	if videoTot < 0 {
		videoTot = 0
	}
	ts := now()
	_, err := s.db.Exec(`UPDATE accounts SET free_image_remaining=?, free_image_total=?, free_video_remaining=?, free_video_total=?, last_free_quota_at=?, updated_at=? WHERE id=?`,
		imageRem, imageTot, videoRem, videoTot, ts, ts, id)
	return err
}

// SetAccountEgress binds account to an egress node (0 clears).
func (s *Store) SetAccountEgress(accountID, egressID int64) error {
	_, err := s.db.Exec(`UPDATE accounts SET egress_id=?, updated_at=? WHERE id=?`, egressID, now(), accountID)
	return err
}

func (s *Store) scanAccountRows(rows *sql.Rows) (Account, error) {
	return s.scanAccount(rows)
}

func (s *Store) scanTask(row rowScanner) (VideoTask, error) {
	var t VideoTask
	err := row.Scan(&t.ID, &t.UserID, &t.Status, &t.Prompt, &t.ImagePath, &t.WebappID, &t.TemplateID, &t.Duration, &t.AspectRatio, &t.Resolution, &t.NodeInfoJSON, &t.AccountID, &t.AccountEmail, &t.RHTaskID, &t.RHStatus, &t.QueueIndex, &t.QueueCount, &t.ProgressText, &t.FileURL, &t.PreviewURL, &t.ErrorCode, &t.Error, &t.Attempts, &t.CreatedAt, &t.UpdatedAt, &t.FinishedAt)
	return t, err
}

func (s *Store) scanTaskRows(rows *sql.Rows) (VideoTask, error) {
	return s.scanTask(rows)
}

const SettingPollAuth = "poll_auth"
const SettingUploadKeepHours = "upload_keep_hours"

func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key=?`, strings.TrimSpace(key)).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) SetSetting(key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("setting key required")
	}
	_, err := s.db.Exec(
		`INSERT INTO app_settings(key,value,updated_at) VALUES(?,?,?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, now(),
	)
	return err
}

// GetUserSetting returns a per-user setting; empty string if missing.
func (s *Store) GetUserSetting(userID int64, key string) (string, error) {
	if userID <= 0 {
		return "", nil
	}
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM user_settings WHERE user_id=? AND key=?`,
		userID, strings.TrimSpace(key),
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetUserSetting upserts a per-user setting.
func (s *Store) SetUserSetting(userID int64, key, value string) error {
	if userID <= 0 {
		return fmt.Errorf("user id required")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("setting key required")
	}
	_, err := s.db.Exec(
		`INSERT INTO user_settings(user_id,key,value,updated_at) VALUES(?,?,?,?)
		 ON CONFLICT(user_id,key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		userID, key, value, now(),
	)
	return err
}

// GetSettingPreferUser returns user value when non-empty, otherwise global app_settings.
func (s *Store) GetSettingPreferUser(userID int64, key string) (string, error) {
	if userID > 0 {
		v, err := s.GetUserSetting(userID, key)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(v) != "" {
			return v, nil
		}
	}
	return s.GetSetting(key)
}

// NormalizeUploadKeepHours clamps to [0, 720] (0 = clean ASAP after finish).
func NormalizeUploadKeepHours(h int) int {
	if h < 0 {
		return 0
	}
	if h > 720 {
		return 720
	}
	return h
}

// UploadKeepHours returns configured hours; empty DB setting falls back to defaultHours.
func (s *Store) UploadKeepHours(defaultHours int) int {
	defaultHours = NormalizeUploadKeepHours(defaultHours)
	v, err := s.GetSetting(SettingUploadKeepHours)
	if err != nil || strings.TrimSpace(v) == "" {
		return defaultHours
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return defaultHours
	}
	return NormalizeUploadKeepHours(n)
}

func (s *Store) SetUploadKeepHours(hours int) error {
	hours = NormalizeUploadKeepHours(hours)
	return s.SetSetting(SettingUploadKeepHours, strconv.Itoa(hours))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// jwtExpiresAtRFC3339 extracts JWT exp as UTC RFC3339; empty if not a JWT / no exp.
func jwtExpiresAtRFC3339(token string) string {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 {
		return ""
	}
	payload := parts[1]
	if m := len(payload) % 4; m != 0 {
		payload += strings.Repeat("=", 4-m)
	}
	raw, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims map[string]any
	if json.Unmarshal(raw, &claims) != nil {
		return ""
	}
	var unix int64
	switch v := claims["exp"].(type) {
	case float64:
		unix = int64(v)
	case string:
		n, e := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if e != nil {
			return ""
		}
		unix = n
	default:
		return ""
	}
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

func (s *Store) backfillATExpiresAt() {
	rows, err := s.db.Query(`SELECT id, access_token FROM accounts WHERE access_token!='' AND at_expires_at=''`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var at string
		if rows.Scan(&id, &at) != nil {
			continue
		}
		if exp := jwtExpiresAtRFC3339(at); exp != "" {
			_, _ = s.db.Exec(`UPDATE accounts SET at_expires_at=? WHERE id=?`, exp, id)
		}
	}
}
