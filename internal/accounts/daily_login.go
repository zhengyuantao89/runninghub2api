package accounts

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/trae/runninghub2api/internal/egress"
	"github.com/trae/runninghub2api/internal/provider/runninghub"
	"github.com/trae/runninghub2api/internal/store"
)

// ShanghaiDate returns YYYY-MM-DD in Asia/Shanghai.
func ShanghaiDate(t time.Time) string {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return t.In(loc).Format("2006-01-02")
}

type DailyClaimResult struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	OK        bool   `json:"ok"`
	Skipped   bool   `json:"skipped,omitempty"`
	Gained    int    `json:"gained,omitempty"`
	Before    string `json:"before_coins,omitempty"`
	After     string `json:"after_coins,omitempty"`
	Triggered any    `json:"login_coin_triggered,omitempty"`
	Day       string `json:"day,omitempty"`
	Error     string `json:"error,omitempty"`
	Relogged  bool   `json:"relogged,omitempty"`
}

type DailyLoginClaimer struct {
	store    *store.Store
	egress   *egress.Manager
	baseURL  string
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	running  bool
}

func NewDailyLoginClaimer(st *store.Store, baseURL string, interval time.Duration) *DailyLoginClaimer {
	if interval < time.Hour {
		interval = 12 * time.Hour
	}
	return &DailyLoginClaimer{
		store:    st,
		egress:   egress.NewManager(st),
		baseURL:  baseURL,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (c *DailyLoginClaimer) Start() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		// startup pass after short delay
		select {
		case <-time.After(20 * time.Second):
			c.safeRunAll()
		case <-c.stopCh:
			return
		}
		t := time.NewTicker(c.interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				c.safeRunAll()
			case <-c.stopCh:
				return
			}
		}
	}()
}

func (c *DailyLoginClaimer) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

func (c *DailyLoginClaimer) safeRunAll() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
	}()

	day := ShanghaiDate(time.Now())
	results, err := c.ClaimAll(nil, false)
	if err != nil {
		log.Printf("[daily-login] claim failed: %v", err)
		return
	}
	ok, skip, fail, gained := 0, 0, 0, 0
	for _, r := range results {
		if r.Skipped {
			skip++
		} else if r.OK {
			ok++
			gained += r.Gained
		} else {
			fail++
		}
	}
	log.Printf("[daily-login] day=%s ok=%d skipped=%d failed=%d gained=%d", day, ok, skip, fail, gained)
}

// ClaimAll claims daily login coin for active accounts.
// ids=nil means all active; force ignores last_login_coin_at skip.
func (c *DailyLoginClaimer) ClaimAll(ids []int64, force bool) ([]DailyClaimResult, error) {
	day := ShanghaiDate(time.Now())
	list, err := c.store.ListAccounts()
	if err != nil {
		return nil, err
	}
	want := map[int64]struct{}{}
	for _, id := range ids {
		want[id] = struct{}{}
	}
	out := make([]DailyClaimResult, 0)
	for _, a := range list {
		if len(want) > 0 {
			if _, ok := want[a.ID]; !ok {
				continue
			}
		}
		if a.Status != "active" {
			continue
		}
		// ListAccounts strips secrets — reload full account
		full, err := c.store.GetAccount(a.ID)
		if err != nil {
			out = append(out, DailyClaimResult{ID: a.ID, Email: a.Email, Error: err.Error()})
			continue
		}
		res := c.ClaimOne(full, day, force)
		out = append(out, res)
		time.Sleep(400 * time.Millisecond)
	}
	return out, nil
}

func (c *DailyLoginClaimer) ClaimOne(acc store.Account, day string, force bool) DailyClaimResult {
	res := DailyClaimResult{ID: acc.ID, Email: acc.Email, Day: day}
	if !force && strings.TrimSpace(acc.LastLoginCoinAt) == day {
		res.OK = true
		res.Skipped = true
		return res
	}

	err := c.egress.Do(acc, c.baseURL, func(client *runninghub.Client) error {
		at := strings.TrimSpace(acc.AccessToken)
		relogged := false
		ensureLogin := func() error {
			if strings.TrimSpace(acc.Email) == "" || strings.TrimSpace(acc.Password) == "" {
				return fmt.Errorf("缺少邮箱或密码，无法重新登录")
			}
			login, err := client.PwdLogin(acc.Email, acc.Password)
			if err != nil {
				return err
			}
			token := asString(nested(login, "data", "access_token"))
			if token == "" {
				token = asString(nested(login, "data", "accessToken"))
			}
			if token == "" {
				return fmt.Errorf("login missing access_token")
			}
			rt := firstNonEmpty(
				asString(nested(login, "data", "refresh_token")),
				asString(nested(login, "data", "refreshToken")),
			)
			client.SetAccessToken(token)
			_ = c.store.UpdateAccountAuthTokens(acc.ID, token, rt)
			at = token
			relogged = true
			return nil
		}

		if at == "" {
			if err := ensureLogin(); err != nil {
				return err
			}
		} else {
			client.SetAccessToken(at)
		}

		uid := strings.TrimSpace(acc.RHUserID)
		if uid == "" {
			uid = jwtSub(at)
			if uid != "" {
				_ = c.store.UpdateAccountRHUserID(acc.ID, uid)
			}
		}
		if uid == "" {
			return fmt.Errorf("缺少 rh_user_id")
		}

		info, err := client.GetUserInfo(uid)
		if err != nil {
			if e2 := ensureLogin(); e2 != nil {
				return err
			}
			if uid == "" {
				uid = jwtSub(at)
			}
			info, err = client.GetUserInfo(uid)
			if err != nil {
				return err
			}
		}

		before := coinFromUserInfo(info)
		triggered := loginTriggered(info)
		res.Before = before
		res.Triggered = triggered
		res.Relogged = relogged

		if triggered {
			time.Sleep(2 * time.Second)
			if info2, e := client.GetUserInfo(uid); e == nil {
				info = info2
			}
		} else {
			time.Sleep(1200 * time.Millisecond)
			if info2, e := client.GetUserInfo(uid); e == nil {
				info = info2
			}
		}

		after := coinFromUserInfo(info)
		res.After = after
		res.Triggered = loginTriggered(info)
		b, a := parseCoinInt(before), parseCoinInt(after)
		if a >= b {
			res.Gained = a - b
		}
		if after != "" {
			_ = c.store.UpdateAccountBalance(acc.ID, after, acc.CurrentTaskCounts, acc.APIType)
		} else if key := strings.TrimSpace(acc.APIKey); key != "" {
			if st, e := client.AccountStatus(key); e == nil {
				if data, _ := st["data"].(map[string]any); data != nil {
					if v, ok := data["remainCoins"]; ok {
						_ = c.store.UpdateAccountBalance(acc.ID, fmt.Sprint(v), acc.CurrentTaskCounts, acc.APIType)
						res.After = fmt.Sprint(v)
					}
				}
			}
		}
		_ = c.store.UpdateAccountLoginCoinAt(acc.ID, day)
		return nil
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.OK = true
	return res
}

func coinFromUserInfo(info map[string]any) string {
	data, _ := info["data"].(map[string]any)
	if data == nil {
		return ""
	}
	if v := strings.TrimSpace(fmt.Sprint(data["totalCoin"])); v != "" && v != "<nil>" {
		return v
	}
	return ""
}

func loginTriggered(info map[string]any) bool {
	data, _ := info["data"].(map[string]any)
	if data == nil {
		return false
	}
	switch v := data["loginCoinTriggered"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true") || v == "1"
	default:
		return false
	}
}

func parseCoinInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "<nil>" {
		return 0
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	n, _ := strconv.Atoi(s)
	return n
}

func jwtSub(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	raw := parts[1]
	switch len(raw) % 4 {
	case 2:
		raw += "=="
	case 3:
		raw += "="
	}
	b, err := base64.URLEncoding.DecodeString(raw)
	if err != nil {
		b, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(payload["sub"]))
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

func nested(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok || obj == nil {
			return nil
		}
		cur = obj[k]
	}
	return cur
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
