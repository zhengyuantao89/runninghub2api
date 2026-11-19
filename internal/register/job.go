package register

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/trae/runninghub2api/internal/captcha"
	"github.com/trae/runninghub2api/internal/egress"
	"github.com/trae/runninghub2api/internal/provider/runninghub"
	"github.com/trae/runninghub2api/internal/store"
)

const (
	SettingCaptchaProvider   = "captcha.provider"
	SettingCaptchaAPIKey     = "captcha.api_key"  // legacy shared; migrated into per-provider keys
	SettingCaptchaAPIBase    = "captcha.api_base" // legacy shared
	SettingCaptchaTimeoutSec = "captcha.timeout_sec"
	SettingRegisterInvite    = "register.invite_code"
	SettingRegisterConc      = "register.concurrency"
	SettingRegisterInterval  = "register.interval_ms" // legacy ms; prefer sec min/max
	SettingRegisterIntervalMinSec = "register.interval_sec_min"
	SettingRegisterIntervalMaxSec = "register.interval_sec_max"
	SettingRegisterOTPSec    = "register.otp_timeout_sec"
)

type Service struct {
	Store   *store.Store
	BaseURL string

	mu      sync.Mutex
	running map[int64]chan struct{} // cancel signals
}

func NewService(st *store.Store, baseURL string) *Service {
	return &Service{
		Store:   st,
		BaseURL: baseURL,
		running: map[int64]chan struct{}{},
	}
}

func (s *Service) LoadCaptchaConfig() captcha.Config {
	return s.LoadCaptchaConfigForUser(0)
}

// LoadCaptchaConfigForUser prefers the user's captcha settings, then global.
func (s *Service) LoadCaptchaConfigForUser(userID int64) captcha.Config {
	provider := captcha.NormalizeProvider(preferSetting(s.Store, userID, SettingCaptchaProvider))
	key, base := LoadCaptchaCredsForUser(s.Store, userID, provider, userID <= 0)
	timeout, _ := strconv.Atoi(strings.TrimSpace(preferSetting(s.Store, userID, SettingCaptchaTimeoutSec)))
	return captcha.Config{
		Provider:   provider,
		APIKey:     key,
		APIBase:    base,
		TimeoutSec: timeout,
	}.Resolved()
}

func preferSetting(st *store.Store, userID int64, key string) string {
	if st == nil {
		return ""
	}
	v, _ := st.GetSettingPreferUser(userID, key)
	return v
}

// LoadCaptchaCreds returns global per-provider key/base.
// When migrateLegacy is true and the slot is empty, copies legacy shared captcha.api_key/base into this provider only.
func LoadCaptchaCreds(st *store.Store, provider string, migrateLegacy bool) (key, base string) {
	return LoadCaptchaCredsForUser(st, 0, provider, migrateLegacy)
}

// LoadCaptchaCredsForUser returns per-provider key/base for a user (fallback global when empty).
// Legacy shared-key migration only runs for global (userID<=0).
func LoadCaptchaCredsForUser(st *store.Store, userID int64, provider string, migrateLegacy bool) (key, base string) {
	if st == nil {
		return "", ""
	}
	provider = captcha.NormalizeProvider(provider)
	if sk := captcha.SettingAPIKey(provider); sk != "" {
		key = preferSetting(st, userID, sk)
	}
	if sb := captcha.SettingAPIBase(provider); sb != "" {
		base = preferSetting(st, userID, sb)
	}
	if strings.TrimSpace(key) != "" || strings.TrimSpace(base) != "" || !migrateLegacy || userID > 0 {
		return key, base
	}
	legacyKey, _ := st.GetSetting(SettingCaptchaAPIKey)
	legacyBase, _ := st.GetSetting(SettingCaptchaAPIBase)
	if strings.TrimSpace(legacyKey) == "" && strings.TrimSpace(legacyBase) == "" {
		return "", ""
	}
	key, base = legacyKey, legacyBase
	if sk := captcha.SettingAPIKey(provider); sk != "" && strings.TrimSpace(legacyKey) != "" {
		_ = st.SetSetting(sk, strings.TrimSpace(legacyKey))
	}
	if sb := captcha.SettingAPIBase(provider); sb != "" && strings.TrimSpace(legacyBase) != "" {
		_ = st.SetSetting(sb, strings.TrimSpace(legacyBase))
	}
	return key, base
}

// LoadCaptchaCredsUserOnly returns only the user's stored creds (no global fallback).
func LoadCaptchaCredsUserOnly(st *store.Store, userID int64, provider string) (key, base string) {
	if st == nil || userID <= 0 {
		return "", ""
	}
	provider = captcha.NormalizeProvider(provider)
	if sk := captcha.SettingAPIKey(provider); sk != "" {
		key, _ = st.GetUserSetting(userID, sk)
	}
	if sb := captcha.SettingAPIBase(provider); sb != "" {
		base, _ = st.GetUserSetting(userID, sb)
	}
	return key, base
}

// SetCaptchaCreds writes api_key/api_base into the given provider's global slot.
func SetCaptchaCreds(st *store.Store, provider, key, base string) error {
	return SetCaptchaCredsForUser(st, 0, provider, key, base)
}

// SetCaptchaCredsForUser writes creds globally (userID<=0) or into user_settings.
func SetCaptchaCredsForUser(st *store.Store, userID int64, provider, key, base string) error {
	if st == nil {
		return fmt.Errorf("store required")
	}
	provider = captcha.NormalizeProvider(provider)
	if provider == "off" || provider == "" {
		return nil
	}
	write := func(k, v string) error {
		if userID > 0 {
			return st.SetUserSetting(userID, k, strings.TrimSpace(v))
		}
		return st.SetSetting(k, strings.TrimSpace(v))
	}
	if sk := captcha.SettingAPIKey(provider); sk != "" {
		if err := write(sk, key); err != nil {
			return err
		}
	}
	if sb := captcha.SettingAPIBase(provider); sb != "" {
		if err := write(sb, base); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) DefaultInviteCode() string {
	return s.DefaultInviteCodeForUser(0)
}

func (s *Service) DefaultInviteCodeForUser(userID int64) string {
	return strings.TrimSpace(preferSetting(s.Store, userID, SettingRegisterInvite))
}

func (s *Service) concurrency() int {
	return s.concurrencyForUser(0)
}

func (s *Service) concurrencyForUser(userID int64) int {
	n, _ := strconv.Atoi(strings.TrimSpace(preferSetting(s.Store, userID, SettingRegisterConc)))
	if n < 1 {
		n = 1
	}
	if n > 5 {
		n = 5
	}
	return n
}

// intervalRangeSec returns [min,max] seconds between accounts (after each
// finish, before the next slot can start). Defaults 8–15s.
func (s *Service) intervalRangeSec() (minSec, maxSec int) {
	return s.intervalRangeSecForUser(0)
}

func (s *Service) intervalRangeSecForUser(userID int64) (minSec, maxSec int) {
	minSec, _ = strconv.Atoi(strings.TrimSpace(preferSetting(s.Store, userID, SettingRegisterIntervalMinSec)))
	maxSec, _ = strconv.Atoi(strings.TrimSpace(preferSetting(s.Store, userID, SettingRegisterIntervalMaxSec)))
	if minSec <= 0 && maxSec <= 0 {
		// Migrate legacy interval_ms once if sec range unset.
		ms, _ := strconv.Atoi(strings.TrimSpace(preferSetting(s.Store, userID, SettingRegisterInterval)))
		if ms > 0 {
			sec := ms / 1000
			if sec < 1 {
				sec = 1
			}
			minSec, maxSec = sec, sec
		} else {
			minSec, maxSec = 8, 15
		}
	}
	if minSec < 0 {
		minSec = 0
	}
	if maxSec < minSec {
		maxSec = minSec
	}
	if maxSec > 600 {
		maxSec = 600
	}
	if minSec > 600 {
		minSec = 600
	}
	return minSec, maxSec
}

func (s *Service) randomIntervalForUser(userID int64) time.Duration {
	minSec, maxSec := s.intervalRangeSecForUser(userID)
	if maxSec <= 0 {
		return 0
	}
	n := minSec
	if maxSec > minSec {
		n = minSec + rand.Intn(maxSec-minSec+1)
	}
	return time.Duration(n) * time.Second
}

func (s *Service) otpTimeout() time.Duration {
	return s.otpTimeoutForUser(0)
}

func (s *Service) otpTimeoutForUser(userID int64) time.Duration {
	sec, _ := strconv.Atoi(strings.TrimSpace(preferSetting(s.Store, userID, SettingRegisterOTPSec)))
	if sec < 30 {
		sec = 180
	}
	if sec > 600 {
		sec = 600
	}
	return time.Duration(sec) * time.Second
}

func (s *Service) StartJob(job store.RegisterJob) {
	s.mu.Lock()
	if _, ok := s.running[job.ID]; ok {
		s.mu.Unlock()
		return
	}
	cancel := make(chan struct{})
	s.running[job.ID] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.running, job.ID)
			s.mu.Unlock()
		}()
		s.runJob(job, cancel)
	}()
}

func (s *Service) CancelJob(id int64) bool {
	s.mu.Lock()
	ch, ok := s.running[id]
	s.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case <-ch:
	default:
		close(ch)
	}
	_ = s.Store.UpdateRegisterJob(id, map[string]any{"status": "cancelling"})
	_ = s.Store.AppendRegisterJobLog(id, "warning", "收到停止指令")
	return true
}

func (s *Service) runJob(job store.RegisterJob, cancel <-chan struct{}) {
	_ = s.Store.UpdateRegisterJob(job.ID, map[string]any{"status": "running", "error": ""})
	_ = s.Store.AppendRegisterJobLog(job.ID, "info", fmt.Sprintf("开始注册 %d 个（仅 @icloud.com）", job.Quantity))

	uid := job.UserID
	invite := strings.TrimSpace(job.InviteCode)
	if invite == "" {
		invite = s.DefaultInviteCodeForUser(uid)
	}
	capCfg := s.LoadCaptchaConfigForUser(uid)
	if capCfg.Provider == "off" || capCfg.APIKey == "" {
		_ = s.Store.AppendRegisterJobLog(job.ID, "error", "打码未配置：请先在设置里填 YesCaptcha/2Captcha/CapSolver")
		_ = s.Store.UpdateRegisterJob(job.ID, map[string]any{
			"status":      "failed",
			"error":       "captcha not configured",
			"finished_at": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	success, failed := 0, 0
	conc := s.concurrencyForUser(uid)
	minSec, maxSec := s.intervalRangeSecForUser(uid)
	otpTO := s.otpTimeoutForUser(uid)
	_ = s.Store.AppendRegisterJobLog(job.ID, "info", fmt.Sprintf("账号间隔 %d–%ds（完成后生效），并发 %d；失败换邮箱不重发码", minSec, maxSec, conc))

	type slot struct{}
	sem := make(chan slot, conc)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < job.Quantity; i++ {
		select {
		case <-cancel:
			_ = s.Store.AppendRegisterJobLog(job.ID, "warning", "任务已停止")
			goto done
		default:
		}
		sem <- slot{}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s.oneAttempt(job, idx+1, invite, capCfg, otpTO, cancel, &mu, &success, &failed)
			// Cooldown after success/fail before freeing the slot → next account.
			wait := s.randomIntervalForUser(uid)
			if wait > 0 {
				_ = s.Store.AppendRegisterJobLog(job.ID, "info", fmt.Sprintf("#%d 账号间隔等待 %s", idx+1, wait))
				_ = sleepCancellable(wait, func() bool {
					select {
					case <-cancel:
						return true
					default:
						return false
					}
				})
			}
			<-sem
		}(i)
	}
	wg.Wait()

done:
	wg.Wait()
	status := "done"
	select {
	case <-cancel:
		status = "cancelled"
	default:
	}
	_ = s.Store.UpdateRegisterJob(job.ID, map[string]any{
		"status":      status,
		"success":     success,
		"failed":      failed,
		"finished_at": time.Now().UTC().Format(time.RFC3339),
	})
	_ = s.Store.AppendRegisterJobLog(job.ID, "info", fmt.Sprintf("结束：成功 %d / 失败 %d", success, failed))
}

func (s *Service) oneAttempt(
	job store.RegisterJob,
	idx int,
	invite string,
	capCfg captcha.Config,
	otpTO time.Duration,
	cancel <-chan struct{},
	mu *sync.Mutex,
	success, failed *int,
) {
	log := func(msg, level string) {
		_ = s.Store.AppendRegisterJobLog(job.ID, level, fmt.Sprintf("#%d %s", idx, msg))
	}
	cancelled := func() bool {
		select {
		case <-cancel:
			return true
		default:
			return false
		}
	}
	if cancelled() {
		return
	}

	inbox, err := s.Store.LeaseMailInbox(job.UserID)
	if err != nil {
		if err == store.ErrNotFound {
			log("邮箱池没有 unused @icloud.com 了", "error")
		} else {
			log(fmt.Sprintf("占邮箱失败：%v", err), "error")
		}
		mu.Lock()
		*failed++
		_ = s.Store.UpdateRegisterJob(job.ID, map[string]any{"failed": *failed})
		mu.Unlock()
		return
	}
	if !store.IsICloudEmail(inbox.Email) {
		_ = s.Store.ReleaseMailInbox(inbox.ID, store.MailDead, "非 @icloud.com")
		log(fmt.Sprintf("跳过非 icloud：%s", inbox.Email), "warning")
		mu.Lock()
		*failed++
		_ = s.Store.UpdateRegisterJob(job.ID, map[string]any{"failed": *failed})
		mu.Unlock()
		return
	}
	log(fmt.Sprintf("占用邮箱 %s", inbox.Email), "info")

	var node *store.EgressNode
	regMode := egress.ModePool
	if v, _ := s.Store.GetSetting(egress.SettingRegisterMode); v != "" {
		regMode = egress.NormalizeMode(v)
	}
	switch regMode {
	case egress.ModeDirect:
		log("出口=直连（设置：注册走直连）", "info")
	case egress.ModeList:
		proxies, _ := s.Store.GetSetting(egress.SettingProxies)
		if n, e := egress.PickListNode(proxies); e == nil {
			node = n
			log(fmt.Sprintf("出口=供应商列表 %s", n.Label()), "info")
		} else {
			log("出口=直连（供应商列表为空或无效；请在代理与结果填写代理行）", "warning")
		}
	default:
		if n, e := s.Store.PickAvailableEgress(0); e == nil {
			nn := n
			node = &nn
			_ = s.Store.MarkEgressUsed(n.ID)
			log(fmt.Sprintf("出口=%s", n.Label()), "info")
		} else {
			log("出口=直连（无可用出口节点；若 RH 被墙请先在出口池启用代理，或改用供应商列表/直连）", "warning")
		}
	}

	// One email = one RegisterOne. Failures release this mailbox and the next
	// job slot leases a new email — never resend code on the same address.
	res, regErr := RegisterOne(FlowOpts{
		BaseURL:    s.BaseURL,
		Email:      inbox.Email,
		PickupURL:  inbox.PickupURL,
		InviteCode: invite,
		Captcha:    capCfg,
		EgressNode: node,
		OTPTimeout: otpTO,
		Log:        log,
		Cancelled:  cancelled,
	})
	if regErr != nil {
		msg := regErr.Error()
		if runninghub.IsServerBusy(regErr) || runninghub.IsFrequentOperations(regErr) ||
			strings.Contains(msg, "发码过快") || strings.Contains(msg, "1653") {
			log(fmt.Sprintf("失败（换下一封邮箱，不重发）：%s", msg), "warning")
		}
		if node == nil && (strings.Contains(msg, "EOF") || strings.Contains(msg, "connection reset") || strings.Contains(msg, "Timeout") || strings.Contains(msg, "timeout")) {
			msg = msg + "；直连 RH 失败，请配置可用出口代理后重试"
		}
		toStatus := mailReleaseStatus(regErr, msg)
		_ = s.Store.ReleaseMailInbox(inbox.ID, toStatus, truncate(msg, 240))
		switch toStatus {
		case store.MailUsed:
			log(fmt.Sprintf("失败：%s → 邮箱池已标为「已注册」，不会再租用", msg), "error")
		case store.MailDead:
			log(fmt.Sprintf("失败：%s → 邮箱池已标为「失效」，不会再租用", msg), "error")
		default:
			log(fmt.Sprintf("失败：%s → 邮箱退回可用，供其它任务使用", msg), "error")
		}
		if node != nil && node.ID > 0 && !runninghub.IsServerBusy(regErr) && !runninghub.IsFrequentOperations(regErr) {
			_, _ = s.Store.MarkEgressFailure(node.ID, truncate(msg, 200))
		}
		mu.Lock()
		*failed++
		_ = s.Store.UpdateRegisterJob(job.ID, map[string]any{"failed": *failed, "success": *success})
		mu.Unlock()
		return
	}

	acc := store.Account{
		UserID:            job.UserID,
		Email:             res.Email,
		Password:          res.Password,
		AccessToken:       res.AccessToken,
		RefreshToken:      res.RefreshToken,
		APIKey:            res.APIKey,
		Identify:          res.Identify,
		RHUserID:          res.RHUserID,
		RemainCoins:       res.RemainCoins,
		CurrentTaskCounts: res.CurrentTaskCounts,
		APIType:           res.APIType,
		AnonymousID:       res.AnonymousID,
		DeviceID:          res.DeviceID,
		FingerprintJSON:   res.FingerprintJSON,
		NickName:          res.NickName,
		EgressID:          res.EgressID,
		Status:            "active",
		Source:            "register",
	}
	saved, err := s.Store.UpsertAccount(acc)
	if err != nil {
		_ = s.Store.ReleaseMailInbox(inbox.ID, store.MailUnused, "入库失败: "+err.Error())
		log(fmt.Sprintf("入库失败：%v", err), "error")
		mu.Lock()
		*failed++
		_ = s.Store.UpdateRegisterJob(job.ID, map[string]any{"failed": *failed})
		mu.Unlock()
		return
	}
	if res.EgressID > 0 {
		_ = s.Store.SetAccountEgress(saved.ID, res.EgressID)
	}
	if strings.TrimSpace(res.NickName) != "" {
		_ = s.Store.UpdateAccountNickName(saved.ID, res.NickName)
	}
	_ = s.Store.ReleaseMailInbox(inbox.ID, store.MailUsed, "")
	if node != nil && node.ID > 0 {
		_ = s.Store.MarkEgressSuccess(node.ID, 0)
	}
	if res.NickName != "" {
		log(fmt.Sprintf("成功入库 %s（昵称 %s）", res.Email, res.NickName), "ok")
	} else {
		log(fmt.Sprintf("成功入库 %s", res.Email), "ok")
	}
	mu.Lock()
	*success++
	_ = s.Store.UpdateRegisterJob(job.ID, map[string]any{"success": *success, "failed": *failed})
	mu.Unlock()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// mailReleaseStatus decides inbox status after a failed RegisterOne.
// Already-registered (1656) → used so LeaseMailInbox will never pick it again.
func mailReleaseStatus(err error, msg string) string {
	var apiErr *runninghub.APIError
	if errors.As(err, &apiErr) && apiErr != nil && fmt.Sprint(apiErr.Code) == "1656" {
		return store.MailUsed
	}
	lower := strings.ToLower(msg)
	if strings.Contains(msg, "已注册") || strings.Contains(msg, "1656") ||
		strings.Contains(lower, "already register") || strings.Contains(lower, "already exists") {
		return store.MailUsed
	}
	if strings.Contains(msg, "鉴权失效") || strings.Contains(msg, "401") || strings.Contains(msg, "403") {
		return store.MailDead
	}
	return store.MailUnused
}

// sleepCancellable sleeps up to d, returning early with error if cancelled.
func sleepCancellable(d time.Duration, cancelled func() bool) error {
	if d <= 0 {
		return nil
	}
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cancelled != nil && cancelled() {
			return fmt.Errorf("cancelled")
		}
		left := time.Until(deadline)
		step := 500 * time.Millisecond
		if left < step {
			step = left
		}
		if step <= 0 {
			break
		}
		time.Sleep(step)
	}
	return nil
}
