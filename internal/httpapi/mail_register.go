package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/trae/runninghub2api/internal/captcha"
	"github.com/trae/runninghub2api/internal/egress"
	"github.com/trae/runninghub2api/internal/mailinbox"
	"github.com/trae/runninghub2api/internal/register"
	"github.com/trae/runninghub2api/internal/store"
)

func (s *Server) handleMailInboxes(w http.ResponseWriter, r *http.Request) {
	a, _ := actorFromRequest(r)
	userID := a.UserID
	if a.IsAdmin() && r.URL.Query().Get("all") == "1" {
		userID = 0
	}
	switch r.Method {
	case http.MethodGet:
		status := r.URL.Query().Get("status")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit <= 0 {
			limit = 100
		}
		list, err := s.store.ListMailInboxes(userID, status, limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		total, _ := s.store.CountMailInboxes(userID, status)
		unused, _ := s.store.CountMailInboxes(userID, store.MailUnused)
		leased, _ := s.store.CountMailInboxes(userID, store.MailLeased)
		used, _ := s.store.CountMailInboxes(userID, store.MailUsed)
		dead, _ := s.store.CountMailInboxes(userID, store.MailDead)
		all, _ := s.store.CountMailInboxes(userID, "")
		writeJSON(w, http.StatusOK, map[string]any{
			"items":  list,
			"total":  total,
			"unused": unused,
			"stats": map[string]any{
				"all":    all,
				"unused": unused,
				"leased": leased,
				"used":   used,
				"dead":   dead,
			},
		})
	case http.MethodPost:
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		imported, updated, skipped, errs := s.store.ImportMailInboxes(a.UserID, body.Text)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"imported": imported,
			"updated":  updated,
			"skipped":  skipped,
			"errors":   errs,
			"hint":     "仅接受 @icloud.com，格式：邮箱----取件链接",
		})
	case http.MethodDelete:
		var body struct {
			IDs []int64 `json:"ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		uid := a.UserID
		if a.IsAdmin() {
			uid = 0
		}
		n, err := s.store.DeleteMailInboxes(body.IDs, uid)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": n})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMailInboxByID(w http.ResponseWriter, r *http.Request) {
	a, _ := actorFromRequest(r)
	path := strings.TrimPrefix(r.URL.Path, "/admin/mail-inboxes/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	if len(parts) == 2 && parts[1] == "test" && r.Method == http.MethodPost {
		list, err := s.store.ListMailInboxes(0, "", 8000, 0)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		var target *store.MailInbox
		for i := range list {
			if list[i].ID == id {
				if !a.IsAdmin() && list[i].UserID != a.UserID {
					writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
					return
				}
				target = &list[i]
				break
			}
		}
		if target == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		code, terminal, err2 := mailinbox.Probe(target.PickupURL)
		if err2 != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err2.Error(), "terminal": terminal})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "code_preview": code, "has_code": code != ""})
		return
	}
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	uid := a.UserID
	if a.IsAdmin() {
		uid = 0
	}
	if err := s.store.DeleteMailInbox(id, uid); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRegisterJobs(w http.ResponseWriter, r *http.Request) {
	a, _ := actorFromRequest(r)
	switch r.Method {
	case http.MethodGet:
		uid := a.UserID
		if a.IsAdmin() && r.URL.Query().Get("all") == "1" {
			uid = 0
		}
		list, err := s.store.ListRegisterJobs(uid, 50)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": list})
	case http.MethodPost:
		var body struct {
			Quantity   int    `json:"quantity"`
			InviteCode string `json:"invite_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		if body.Quantity < 1 {
			body.Quantity = 1
		}
		if body.Quantity > 50 {
			body.Quantity = 50
		}
		unused, _ := s.store.CountMailInboxes(a.UserID, store.MailUnused)
		if unused < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "邮箱池没有可用的 @icloud.com"})
			return
		}
		if body.Quantity > unused {
			body.Quantity = unused
		}
		job, err := s.store.CreateRegisterJob(a.UserID, body.Quantity, body.InviteCode)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		if s.register == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "register service unavailable"})
			return
		}
		s.register.StartJob(job)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "job": job})
	case http.MethodDelete:
		uid := a.UserID
		if a.IsAdmin() && r.URL.Query().Get("all") == "1" {
			uid = 0
		}
		n, err := s.store.ClearFinishedRegisterJobs(uid)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": n})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRegisterJobByID(w http.ResponseWriter, r *http.Request) {
	a, _ := actorFromRequest(r)
	path := strings.TrimPrefix(r.URL.Path, "/admin/register-jobs/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	job, err := s.store.GetRegisterJob(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !a.IsAdmin() && job.UserID != a.UserID {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	if len(parts) >= 2 && parts[1] == "logs" && r.Method == http.MethodGet {
		after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		logs, err := s.store.ListRegisterJobLogs(id, after, 300)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"job": job, "logs": logs})
		return
	}
	if len(parts) >= 2 && parts[1] == "cancel" && r.Method == http.MethodPost {
		ok := false
		if s.register != nil {
			ok = s.register.CancelJob(id)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": ok})
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

// captchaSettingsView returns captcha/register/egress settings.
// For non-admin (userID>0): returns that user's own captcha/register values (no global api_key leak),
// plus effective_captcha_configured from user-then-global resolution.
// For admin (userID<=0): returns global settings.
func (s *Server) captchaSettingsView() map[string]any {
	return s.captchaSettingsViewFor(0, true)
}

func (s *Server) captchaSettingsViewFor(userID int64, isAdmin bool) map[string]any {
	scopeUID := int64(0)
	if !isAdmin && userID > 0 {
		scopeUID = userID
	}

	readOwn := func(key string) string {
		if scopeUID > 0 {
			v, _ := s.store.GetUserSetting(scopeUID, key)
			return v
		}
		v, _ := s.store.GetSetting(key)
		return v
	}

	provider := captcha.NormalizeProvider(readOwn(register.SettingCaptchaProvider))
	var key, base string
	if scopeUID > 0 {
		key, base = register.LoadCaptchaCredsUserOnly(s.store, scopeUID, provider)
	} else {
		key, base = register.LoadCaptchaCreds(s.store, provider, true)
	}
	timeout := readOwn(register.SettingCaptchaTimeoutSec)
	cfg := captcha.Config{Provider: provider, APIKey: key, APIBase: base}.Resolved()
	to, _ := strconv.Atoi(strings.TrimSpace(timeout))
	if to > 0 {
		cfg.TimeoutSec = to
	} else if scopeUID > 0 {
		// Show resolved timeout when user hasn't set one.
		eff := s.register.LoadCaptchaConfigForUser(scopeUID)
		cfg.TimeoutSec = eff.TimeoutSec
	}
	invite := readOwn(register.SettingRegisterInvite)
	conc := readOwn(register.SettingRegisterConc)
	intervalMin := readOwn(register.SettingRegisterIntervalMinSec)
	intervalMax := readOwn(register.SettingRegisterIntervalMaxSec)
	intervalLegacy := readOwn(register.SettingRegisterInterval)
	otpSec := readOwn(register.SettingRegisterOTPSec)
	if strings.TrimSpace(intervalMin) == "" && strings.TrimSpace(intervalMax) == "" && strings.TrimSpace(intervalLegacy) != "" {
		if ms, err := strconv.Atoi(strings.TrimSpace(intervalLegacy)); err == nil && ms > 0 {
			sec := ms / 1000
			if sec < 1 {
				sec = 1
			}
			intervalMin = strconv.Itoa(sec)
			intervalMax = intervalMin
		}
	}
	if strings.TrimSpace(intervalMin) == "" {
		intervalMin = "8"
	}
	if strings.TrimSpace(intervalMax) == "" {
		intervalMax = "15"
	}
	if strings.TrimSpace(conc) == "" {
		conc = "1"
	}
	if strings.TrimSpace(otpSec) == "" {
		otpSec = "180"
	}

	regMode, _ := s.store.GetSetting(egress.SettingRegisterMode)
	genMode, _ := s.store.GetSetting(egress.SettingGenerateMode)
	proxies, _ := s.store.GetSetting(egress.SettingProxies)

	providers := map[string]any{}
	for _, p := range captcha.ProvidersWithCreds {
		var pk, pb string
		if scopeUID > 0 {
			pk, pb = register.LoadCaptchaCredsUserOnly(s.store, scopeUID, p)
		} else {
			pk, pb = register.LoadCaptchaCreds(s.store, p, false)
		}
		providers[p] = map[string]any{
			"api_key":          pk,
			"api_base":         pb,
			"default_api_base": captcha.DefaultAPIBase(p),
			"configured":       strings.TrimSpace(pk) != "",
		}
	}

	resolveUID := scopeUID
	if isAdmin {
		resolveUID = 0
	} else if userID > 0 {
		resolveUID = userID
	}
	eff := s.register.LoadCaptchaConfigForUser(resolveUID)
	effConfigured := eff.Provider != "off" && strings.TrimSpace(eff.APIKey) != ""

	out := map[string]any{
		"captcha": map[string]any{
			"provider":                     provider,
			"api_key_configured":           strings.TrimSpace(key) != "",
			"api_key":                      key,
			"api_base":                     base,
			"default_api_base":             captcha.DefaultAPIBase(provider),
			"timeout_sec":                  cfg.TimeoutSec,
			"providers":                    providers,
			"effective_captcha_configured": effConfigured,
			"scope":                        map[bool]string{true: "global", false: "user"}[scopeUID == 0],
		},
		"register": map[string]any{
			"invite_code":      invite,
			"concurrency":      conc,
			"interval_sec_min": intervalMin,
			"interval_sec_max": intervalMax,
			"otp_timeout_sec":  otpSec,
		},
	}
	if isAdmin {
		out["egress"] = map[string]any{
			"register_mode": egress.NormalizeMode(regMode),
			"generate_mode": egress.NormalizeMode(genMode),
			"proxies":       proxies,
			"proxy_lines":   len(egress.SplitProxyLines(proxies)),
		}
	}
	return out
}

func (s *Server) applyCaptchaRegisterSettings(body map[string]json.RawMessage) error {
	return s.applyCaptchaRegisterSettingsFor(0, body, true)
}

func (s *Server) setScopedSetting(userID int64, key, value string) error {
	if userID > 0 {
		return s.store.SetUserSetting(userID, key, value)
	}
	return s.store.SetSetting(key, value)
}

func (s *Server) getScopedSetting(userID int64, key string) string {
	if userID > 0 {
		v, _ := s.store.GetUserSetting(userID, key)
		return v
	}
	v, _ := s.store.GetSetting(key)
	return v
}

func (s *Server) applyCaptchaRegisterSettingsFor(userID int64, body map[string]json.RawMessage, isAdmin bool) error {
	scopeUID := int64(0)
	if !isAdmin && userID > 0 {
		scopeUID = userID
	}
	if raw, ok := body["captcha"]; ok {
		var capRaw map[string]any
		if err := json.Unmarshal(raw, &capRaw); err == nil {
			provider := ""
			if v, ok := capRaw["provider"].(string); ok {
				provider = captcha.NormalizeProvider(v)
				_ = s.setScopedSetting(scopeUID, register.SettingCaptchaProvider, provider)
			} else {
				provider = captcha.NormalizeProvider(s.getScopedSetting(scopeUID, register.SettingCaptchaProvider))
			}
			_, hasKey := capRaw["api_key"]
			_, hasBase := capRaw["api_base"]
			if hasKey || hasBase {
				key := ""
				base := ""
				if v, ok := capRaw["api_key"].(string); ok {
					key = v
				} else if scopeUID > 0 {
					key, _ = register.LoadCaptchaCredsUserOnly(s.store, scopeUID, provider)
				} else {
					key, _ = register.LoadCaptchaCreds(s.store, provider, false)
				}
				if v, ok := capRaw["api_base"].(string); ok {
					base = v
				} else if scopeUID > 0 {
					_, base = register.LoadCaptchaCredsUserOnly(s.store, scopeUID, provider)
				} else {
					_, base = register.LoadCaptchaCreds(s.store, provider, false)
				}
				_ = register.SetCaptchaCredsForUser(s.store, scopeUID, provider, key, base)
			}
			if v, ok := capRaw["timeout_sec"]; ok {
				_ = s.setScopedSetting(scopeUID, register.SettingCaptchaTimeoutSec, strings.TrimSpace(asAnyString(v)))
			}
		}
	}
	if raw, ok := body["register"]; ok {
		var regRaw map[string]any
		if err := json.Unmarshal(raw, &regRaw); err == nil {
			if v, ok := regRaw["invite_code"].(string); ok {
				_ = s.setScopedSetting(scopeUID, register.SettingRegisterInvite, strings.TrimSpace(v))
			}
			if v, ok := regRaw["concurrency"]; ok {
				_ = s.setScopedSetting(scopeUID, register.SettingRegisterConc, strings.TrimSpace(asAnyString(v)))
			}
			if v, ok := regRaw["interval_sec_min"]; ok {
				_ = s.setScopedSetting(scopeUID, register.SettingRegisterIntervalMinSec, strings.TrimSpace(asAnyString(v)))
			}
			if v, ok := regRaw["interval_sec_max"]; ok {
				_ = s.setScopedSetting(scopeUID, register.SettingRegisterIntervalMaxSec, strings.TrimSpace(asAnyString(v)))
			}
			// Legacy ms field still accepted → convert to sec min=max.
			if v, ok := regRaw["interval_ms"]; ok {
				ms, _ := strconv.Atoi(strings.TrimSpace(asAnyString(v)))
				if ms > 0 {
					sec := ms / 1000
					if sec < 1 {
						sec = 1
					}
					_ = s.setScopedSetting(scopeUID, register.SettingRegisterIntervalMinSec, strconv.Itoa(sec))
					_ = s.setScopedSetting(scopeUID, register.SettingRegisterIntervalMaxSec, strconv.Itoa(sec))
				}
				_ = s.setScopedSetting(scopeUID, register.SettingRegisterInterval, strings.TrimSpace(asAnyString(v)))
			}
			if v, ok := regRaw["otp_timeout_sec"]; ok {
				_ = s.setScopedSetting(scopeUID, register.SettingRegisterOTPSec, strings.TrimSpace(asAnyString(v)))
			}
		}
	}
	if raw, ok := body["egress"]; ok {
		if !isAdmin {
			return errors.New("仅管理员可改出口模式")
		}
		var egRaw map[string]any
		if err := json.Unmarshal(raw, &egRaw); err == nil {
			if v, ok := egRaw["register_mode"].(string); ok {
				_ = s.store.SetSetting(egress.SettingRegisterMode, egress.NormalizeMode(v))
			}
			if v, ok := egRaw["generate_mode"].(string); ok {
				_ = s.store.SetSetting(egress.SettingGenerateMode, egress.NormalizeMode(v))
			}
			if v, ok := egRaw["proxies"].(string); ok {
				_ = s.store.SetSetting(egress.SettingProxies, v)
			}
		}
	}
	return nil
}

func asAnyString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
