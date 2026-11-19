package register

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/trae/runninghub2api/internal/captcha"
	"github.com/trae/runninghub2api/internal/egress"
	"github.com/trae/runninghub2api/internal/mailinbox"
	"github.com/trae/runninghub2api/internal/provider/runninghub"
	"github.com/trae/runninghub2api/internal/store"
)

type FlowOpts struct {
	BaseURL     string
	Email       string
	PickupURL   string
	Password    string
	InviteCode  string
	Captcha     captcha.Config
	EgressNode  *store.EgressNode
	OTPTimeout  time.Duration
	Log         func(msg, level string)
	Cancelled   func() bool
}

type Result struct {
	Email             string
	Password          string
	AccessToken       string
	RefreshToken      string
	Identify          string
	RHUserID          string
	APIKey            string
	RemainCoins       string
	CurrentTaskCounts string
	APIType           string
	AnonymousID       string
	DeviceID          string
	FingerprintJSON   string
	NickName          string
	EgressID          int64
}

func RegisterOne(opts FlowOpts) (Result, error) {
	email := strings.TrimSpace(opts.Email)
	if !store.IsICloudEmail(email) {
		return Result{}, fmt.Errorf("仅支持 @icloud.com：%s", email)
	}
	password := strings.TrimSpace(opts.Password)
	if password == "" {
		password = RandomPassword(14)
	}
	log := opts.Log
	if log == nil {
		log = func(string, string) {}
	}
	cancelled := opts.Cancelled
	if cancelled == nil {
		cancelled = func() bool { return false }
	}
	assertAlive := func() error {
		if cancelled() {
			return fmt.Errorf("任务已取消")
		}
		return nil
	}

	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = "https://www.runninghub.ai"
	}
	httpClient, err := egress.BuildHTTPClient(opts.EgressNode, 120*time.Second)
	if err != nil {
		return Result{}, err
	}
	client := runninghub.NewClient(baseURL)
	client.HTTP = httpClient
	fp := PickFingerprint()
	fpJSON := MustFingerprintJSON(fp)
	client.ApplyIdentity(client.AnonymousID, fpJSON)
	deviceID := RandomDeviceID()
	log(fmt.Sprintf("指纹=%s (%s) anonymous=%s device=%s",
		fp.ProfileID, firstNonEmpty(fp.LabelZH, fp.ProfileID), shortID(client.AnonymousID), shortID(deviceID)), "info")

	if err := assertAlive(); err != nil {
		return Result{}, err
	}
	log("getConfig", "info")
	cfgRes, err := client.GetConfig()
	if err != nil {
		return Result{}, fmt.Errorf("getConfig: %w", err)
	}
	siteKey := "0x4AAAAAAFgUPnToO_F2W9Dd"
	if data, ok := cfgRes["data"].(map[string]any); ok {
		if v := strings.TrimSpace(fmt.Sprint(data["turnstileSiteKey"])); v != "" && v != "<nil>" {
			siteKey = v
		}
	}

	// Snapshot whatever OTP is already on the mailbox *before* sendCode,
	// so WaitOTP will not reuse a stale code from a previous attempt.
	otpBaseline := ""
	if b, _, err := mailinbox.Probe(opts.PickupURL); err == nil {
		otpBaseline = strings.TrimSpace(b)
	}
	if otpBaseline != "" {
		log(fmt.Sprintf("发码前取件基线 len=%d（等待时将忽略）", len(otpBaseline)), "info")
	} else {
		log("发码前取件为空，等待全新验证码", "info")
	}

	// At most one successful sendCode per email. 1662 (bad turnstile) may
	// refresh the token once; 1653/114 fail out so the job can switch mailbox
	// instead of burning another captcha on the same address.
	maxTurnstile := 2
	sendOK := false
	for attempt := 1; attempt <= maxTurnstile; attempt++ {
		if err := assertAlive(); err != nil {
			return Result{}, err
		}
		log(fmt.Sprintf("Turnstile + sendCode (%d/%d)", attempt, maxTurnstile), "info")
		cfToken, err := captcha.SolveTurnstile(captcha.SolveOpts{
			SiteKey: siteKey,
			PageURL: client.PageURL("/zh-cn"),
			Action:  captcha.RHRegisterAction,
			Config:  opts.Captcha,
			HTTP:    &http.Client{Timeout: 30 * time.Second},
			Log:     log,
		})
		if err != nil {
			return Result{}, err
		}
		if err := assertAlive(); err != nil {
			return Result{}, err
		}
		sent, err := client.SendRegisterCode(email, cfToken)
		if err != nil {
			var apiErr *runninghub.APIError
			if errors.As(err, &apiErr) && fmt.Sprint(apiErr.Code) == "1654" {
				return Result{}, fmt.Errorf("RH 发信失败(1654)：仅支持 @icloud.com：%s", apiErr.Msg)
			}
			return Result{}, err
		}
		code := asInt(sent["code"])
		msg := asString(sent["msg"])
		switch code {
		case 0:
			sendOK = true
		case 114:
			return Result{}, fmt.Errorf("RH 频控(114)：%s（换邮箱）", msg)
		case 1653:
			return Result{}, fmt.Errorf("发码过快(1653)：%s（换邮箱，不重发）", msg)
		case 1662:
			if attempt >= maxTurnstile {
				return Result{}, fmt.Errorf("TURNSTILE_REQUIRED(1662)：%s（换邮箱）", msg)
			}
			log("TURNSTILE_REQUIRED(1662)，换新 token 再试一次", "warning")
			continue
		case 1656:
			return Result{}, fmt.Errorf("邮箱已注册: %s", email)
		case 1654:
			return Result{}, fmt.Errorf("RH 发信失败(1654)：仅支持 @icloud.com：%s", msg)
		default:
			return Result{}, fmt.Errorf("sendCode 失败: %d %s", code, msg)
		}
		if sendOK {
			break
		}
	}
	if !sendOK {
		return Result{}, fmt.Errorf("sendCode 失败（换邮箱）")
	}

	if err := assertAlive(); err != nil {
		return Result{}, err
	}
	// Re-probe right after sendCode: catch OTPs that arrived during Turnstile
	// so WaitOTP will not treat a late stale code as "new".
	baselines := []string{}
	if otpBaseline != "" {
		baselines = append(baselines, otpBaseline)
	}
	if b, _, err := mailinbox.Probe(opts.PickupURL); err == nil {
		b = strings.TrimSpace(b)
		if b != "" {
			baselines = append(baselines, b)
			if b != otpBaseline {
				log(fmt.Sprintf("发码后仍见旧码 len=%d，继续等待新码", len(b)), "info")
			}
		}
	}
	log("等待邮箱验证码", "info")
	otpTimeout := opts.OTPTimeout
	if otpTimeout <= 0 {
		otpTimeout = 3 * time.Minute
	}
	otp, err := mailinbox.WaitOTP(mailinbox.WaitOpts{
		PickupURL: opts.PickupURL,
		Timeout:   otpTimeout,
		Interval:  3 * time.Second,
		Log:       log,
		Cancelled: cancelled,
		Baselines: baselines,
	})
	if err != nil {
		return Result{}, err
	}
	otp = strings.TrimSpace(otp)
	if len(otp) < 4 || len(otp) > 8 {
		return Result{}, fmt.Errorf("验证码格式异常: len=%d", len(otp))
	}
	for _, b := range baselines {
		if b != "" && otp == b {
			return Result{}, fmt.Errorf("取到的仍是发码前旧验证码，已中止")
		}
	}

	if err := assertAlive(); err != nil {
		return Result{}, err
	}
	log(fmt.Sprintf("register（otp_len=%d device_len=%d channel=null）", len(otp), len(deviceID)), "info")
	var regErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if err := assertAlive(); err != nil {
			return Result{}, err
		}
		_, regErr = client.RegisterEmail(email, otp, password, opts.InviteCode, deviceID)
		if regErr == nil {
			break
		}
		var apiErr *runninghub.APIError
		if errors.As(regErr, &apiErr) && apiErr != nil {
			log(fmt.Sprintf("register 响应：http=%d code=%v msg=%s", apiErr.Status, apiErr.Code, apiErr.Msg), "warning")
			if fmt.Sprint(apiErr.Code) == "1656" {
				return Result{}, fmt.Errorf("邮箱已注册: %s", email)
			}
		} else {
			log(fmt.Sprintf("register 错误：%v", regErr), "warning")
		}
		if !runninghub.IsServerBusy(regErr) || attempt == 3 {
			return Result{}, fmt.Errorf("register: %w", regErr)
		}
		wait := time.Duration(attempt*2) * time.Second
		log(fmt.Sprintf("RH 判定繁忙，%s 后重试 register（%d/3）", wait, attempt), "warning")
		time.Sleep(wait)
	}

	if err := assertAlive(); err != nil {
		return Result{}, err
	}
	log("pwdLogin", "info")
	login, err := client.PwdLogin(email, password)
	if err != nil {
		return Result{}, fmt.Errorf("pwdLogin: %w", err)
	}
	data, _ := login["data"].(map[string]any)
	accessToken := firstNonEmpty(asString(data["access_token"]), asString(data["accessToken"]))
	if accessToken == "" {
		return Result{}, fmt.Errorf("登录成功但无 access_token")
	}
	refreshToken := firstNonEmpty(asString(data["refresh_token"]), asString(data["refreshToken"]))
	identify := asString(data["identify"])
	client.SetAccessToken(accessToken)
	rhUserID := runninghub.DecodeJWTSub(accessToken)
	if rhUserID == "" {
		rhUserID = identify
	}

	// Rename after login (needs JWT), before creating API key — soft-fail only.
	nickName := RandomNickName()
	log(fmt.Sprintf("改昵称 → %s", nickName), "info")
	if edited, err := client.EditUser(nickName, runninghub.DefaultHeadIcon, "null"); err != nil {
		log(fmt.Sprintf("改昵称失败（忽略继续拿 Key）：%v", err), "warning")
		nickName = ""
	} else if data, ok := edited["data"].(map[string]any); ok {
		if v := strings.TrimSpace(asString(data["nickName"])); v != "" {
			nickName = v
		}
	}

	apiKey := ""
	remainCoins := ""
	taskCounts := ""
	apiType := ""
	log("创建消费级 API Key", "info")
	if created, err := client.CreateConsumerApiKey(); err != nil {
		log(fmt.Sprintf("创建消费级 API Key 失败: %v", err), "warning")
	} else {
		apiKey = runninghub.ParseConsumerApiKey(created)
		if apiKey == "" {
			if reset, err2 := client.ResetConsumerApiKey(); err2 == nil {
				apiKey = runninghub.ParseConsumerApiKey(reset)
			}
		}
		if apiKey != "" {
			log("消费级 API Key 已创建", "ok")
			if st, err := client.AccountStatus(apiKey); err == nil {
				if d, ok := st["data"].(map[string]any); ok {
					remainCoins = asString(d["remainCoins"])
					taskCounts = asString(d["currentTaskCounts"])
					apiType = asString(d["apiType"])
				}
			}
		} else {
			log("createConsumer 未返回有效 apiKey", "warning")
		}
	}

	var egressID int64
	if opts.EgressNode != nil {
		egressID = opts.EgressNode.ID
	}
	return Result{
		Email:             email,
		Password:          password,
		AccessToken:       accessToken,
		RefreshToken:      refreshToken,
		Identify:          identify,
		RHUserID:          rhUserID,
		APIKey:            apiKey,
		RemainCoins:       remainCoins,
		CurrentTaskCounts: taskCounts,
		APIType:           apiType,
		AnonymousID:       client.AnonymousID,
		DeviceID:          deviceID,
		FingerprintJSON:   fpJSON,
		NickName:          nickName,
		EgressID:          egressID,
	}, nil
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		s := fmt.Sprint(t)
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		var i int
		fmt.Sscanf(t, "%d", &i)
		return i
	default:
		return -1
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func shortID(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 12 {
		return s
	}
	return s[:8] + "…"
}
