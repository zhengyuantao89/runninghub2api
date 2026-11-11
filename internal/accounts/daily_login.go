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
	ID             int64  `json:"id"`
	Email          string `json:"email"`
	OK             bool   `json:"ok"`
	Skipped        bool   `json:"skipped,omitempty"`
	Gained         int    `json:"gained,omitempty"`
	Before         string `json:"before_coins,omitempty"`
	After          string `json:"after_coins,omitempty"`
	Triggered      any    `json:"login_coin_triggered,omitempty"`
	BillingVersion string `json:"billing_version,omitempty"`
	GiftAmount     string `json:"gift_amount,omitempty"`
	AlreadyGranted bool   `json:"already_granted,omitempty"`
	Day            string `json:"day,omitempty"`
	Error          string `json:"error,omitempty"`
	Relogged       bool   `json:"relogged,omitempty"`
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
	return c.ClaimScoped(0, ids, force)
}

// ClaimScoped claims daily login gifts. userID=0 means all users (admin/background).
func (c *DailyLoginClaimer) ClaimScoped(userID int64, ids []int64, force bool) ([]DailyClaimResult, error) {
	day := ShanghaiDate(time.Now())
	list, err := c.store.ListAccounts(userID)
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
		// Active accounts, or disabled only for zero balance (claim is how they refill).
		if a.Status != "active" && !claimableNoCoins(a) {
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

func claimableNoCoins(a store.Account) bool {
	if a.Status != "disabled" {
		return false
	}
	err := strings.TrimSpace(a.LastError)
	return strings.HasPrefix(err, "NO_COINS")
}

func (c *DailyLoginClaimer) ClaimOne(acc store.Account, day string, force bool) DailyClaimResult {
	res := DailyClaimResult{ID: acc.ID, Email: acc.Email, Day: day}
	if !force && strings.TrimSpace(acc.LastLoginCoinAt) == day {
		res.OK = true
		res.Skipped = true
		res.BillingVersion = strings.TrimSpace(acc.BillingVersion)
		res.Before = firstNonEmpty(acc.RemainPoints, acc.RemainCoins)
		// 今日已领过仍要刷新钱包，避免库里余额过期（如页面仍显示旧积分）
		if err := c.syncWalletOnly(acc, &res); err != nil {
			res.Error = "钱包同步失败: " + err.Error()
		}
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

		if at == "" || !runninghub.AccessTokenFresh(at, 24*time.Hour) {
			if err := ensureLogin(); err != nil {
				if at == "" {
					return err
				}
				client.SetAccessToken(at)
			}
		} else {
			client.SetAccessToken(at)
		}
		res.Relogged = relogged

		// Detect billing: NEW → welfare daily-gift; OLD → legacy getUserInfo login coin.
		if ps, e := client.PointSummary(); e == nil && runninghub.IsNewBilling(ps) {
			return c.claimNewBilling(acc, client, &res, day, ps)
		}
		return c.claimOldBilling(acc, client, &res, day, at, ensureLogin)
	})
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.OK = true
	return res
}

// syncWalletOnly refreshes RH币/积分 into DB without claiming the daily gift again.
// Prefer API Key accountStatus so we do not burn AT/login just to refresh balance.
func (c *DailyLoginClaimer) syncWalletOnly(acc store.Account, res *DailyClaimResult) error {
	return c.egress.Do(acc, c.baseURL, func(client *runninghub.Client) error {
		apiKey := strings.TrimSpace(acc.APIKey)
		at := strings.TrimSpace(acc.AccessToken)

		// Only attach existing fresh AT as fallback; never pwd-login here for wallet sync.
		if at != "" && runninghub.AccessTokenFresh(at, 24*time.Hour) {
			client.SetAccessToken(at)
		} else if apiKey == "" {
			if strings.TrimSpace(acc.Email) == "" || strings.TrimSpace(acc.Password) == "" {
				return fmt.Errorf("缺少 API Key / 登录凭据，无法同步钱包")
			}
			login, err := client.PwdLogin(acc.Email, acc.Password)
			if err != nil {
				return err
			}
			token := firstNonEmpty(
				asString(nested(login, "data", "access_token")),
				asString(nested(login, "data", "accessToken")),
			)
			if token == "" {
				return fmt.Errorf("login missing access_token")
			}
			rt := firstNonEmpty(
				asString(nested(login, "data", "refresh_token")),
				asString(nested(login, "data", "refreshToken")),
			)
			client.SetAccessToken(token)
			_ = c.store.UpdateAccountAuthTokens(acc.ID, token, rt)
		}

		wallet, err := client.FetchWallet(apiKey)
		if err != nil {
			return err
		}
		res.BillingVersion = wallet.BillingVersion
		bal := firstNonEmpty(wallet.RemainPoints, wallet.RemainCoins)
		if strings.TrimSpace(res.Before) == "" {
			res.Before = bal
		}
		res.After = bal
		if err := c.store.UpdateAccountWallet(acc.ID, wallet.RemainCoins, wallet.RemainPoints, wallet.CurrentTaskCounts, wallet.APIType, wallet.BillingVersion); err != nil {
			return err
		}
		// free quota still needs AT; only refresh when we already have one attached
		if strings.TrimSpace(client.AccessToken) != "" {
			RefreshFreeQuota(c.store, client, acc.ID)
		}
		return nil
	})
}

func (c *DailyLoginClaimer) claimNewBilling(acc store.Account, client *runninghub.Client, res *DailyClaimResult, day string, beforePS map[string]any) error {
	res.BillingVersion = runninghub.BillingNew
	beforePoint, _ := runninghub.PointRemain(beforePS)
	res.Before = beforePoint

	ws, err := client.WelfareSummary(false)
	if err != nil {
		return err
	}
	granted, amount, _, ok := runninghub.DailyGiftGranted(ws)
	if !ok {
		return fmt.Errorf("welfare/summary missing dailyGift")
	}
	if granted {
		res.AlreadyGranted = true
		res.GiftAmount = amount
	} else {
		if _, err := client.ClaimDailyGift(); err != nil {
			return err
		}
		time.Sleep(800 * time.Millisecond)
		if ws2, e := client.WelfareSummary(true); e == nil {
			if g, amt, _, ok2 := runninghub.DailyGiftGranted(ws2); ok2 {
				granted = g
				if amt != "" {
					amount = amt
				}
			}
		}
		res.GiftAmount = amount
		if !granted && amount != "" {
			res.GiftAmount = amount
		}
	}

	afterPoint := beforePoint
	if ps2, e := client.PointSummary(); e == nil {
		afterPoint, _ = runninghub.PointRemain(ps2)
	}
	res.After = afterPoint
	b, a := parseCoinInt(beforePoint), parseCoinInt(afterPoint)
	if a >= b {
		res.Gained = a - b
	}
	if res.Gained == 0 && res.GiftAmount != "" && !res.AlreadyGranted {
		res.Gained = parseCoinInt(res.GiftAmount)
	}

	wallet, _ := client.FetchWallet(acc.APIKey)
	points := afterPoint
	counts, apiType := acc.CurrentTaskCounts, acc.APIType
	coins := ""
	if wallet.BillingVersion == runninghub.BillingNew {
		if wallet.RemainPoints != "" {
			points = wallet.RemainPoints
			res.After = points
		}
		coins = wallet.RemainCoins
		if wallet.CurrentTaskCounts != "" {
			counts = wallet.CurrentTaskCounts
		}
		if wallet.APIType != "" {
			apiType = wallet.APIType
		}
	}
	_ = c.store.UpdateAccountWallet(acc.ID, coins, points, counts, apiType, runninghub.BillingNew)
	_ = c.store.UpdateAccountLoginCoinAt(acc.ID, day)
	RefreshFreeQuota(c.store, client, acc.ID)
	return nil
}

func (c *DailyLoginClaimer) claimOldBilling(acc store.Account, client *runninghub.Client, res *DailyClaimResult, day, at string, ensureLogin func() error) error {
	res.BillingVersion = runninghub.BillingOld
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
		uid = firstNonEmpty(jwtSub(client.AccessToken), uid)
		info, err = client.GetUserInfo(uid)
		if err != nil {
			return err
		}
	}

	before := coinFromUserInfo(info)
	triggered := loginTriggered(info)
	res.Before = before
	res.Triggered = triggered

	if triggered {
		time.Sleep(2 * time.Second)
	} else {
		time.Sleep(1200 * time.Millisecond)
	}
	if info2, e := client.GetUserInfo(uid); e == nil {
		info = info2
	}

	after := coinFromUserInfo(info)
	res.After = after
	res.Triggered = loginTriggered(info)
	b, a := parseCoinInt(before), parseCoinInt(after)
	if a >= b {
		res.Gained = a - b
	}
	if after != "" {
		_ = c.store.UpdateAccountBalanceWithBilling(acc.ID, after, acc.CurrentTaskCounts, acc.APIType, runninghub.BillingOld)
	} else if key := strings.TrimSpace(acc.APIKey); key != "" {
		if st, e := client.AccountStatus(key); e == nil {
			if data, _ := st["data"].(map[string]any); data != nil {
				if v, ok := data["remainCoins"]; ok {
					_ = c.store.UpdateAccountBalanceWithBilling(acc.ID, fmt.Sprint(v), acc.CurrentTaskCounts, acc.APIType, runninghub.BillingOld)
					res.After = fmt.Sprint(v)
				}
			}
		}
	} else {
		_ = c.store.UpdateAccountBalanceWithBilling(acc.ID, after, acc.CurrentTaskCounts, acc.APIType, runninghub.BillingOld)
	}
	_ = c.store.UpdateAccountLoginCoinAt(acc.ID, day)
	return nil
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
