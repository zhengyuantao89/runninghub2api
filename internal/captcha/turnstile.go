package captcha

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	ProviderYesCaptcha = "yescaptcha"
	Provider2Captcha   = "2captcha"
	ProviderCapSolver  = "capsolver"
	RHRegisterAction   = "reg_send_code"
)

var defaultBase = map[string]string{
	ProviderYesCaptcha: "https://api.yescaptcha.com",
	Provider2Captcha:   "https://api.2captcha.com",
	ProviderCapSolver:  "https://api.capsolver.com",
}

// ProvidersWithCreds are platforms that store their own api_key / api_base.
var ProvidersWithCreds = []string{ProviderYesCaptcha, Provider2Captcha, ProviderCapSolver}

type Config struct {
	Provider        string
	APIKey          string
	APIBase         string
	TimeoutSec      int
	PollIntervalSec int
}

func NormalizeProvider(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "yes", "yes-captcha", "yescaptcha":
		return ProviderYesCaptcha
	case "anti-captcha", "anticaptcha", "2captcha":
		return Provider2Captcha
	case "capsolver":
		return ProviderCapSolver
	case "off", "manual", "":
		return "off"
	default:
		return strings.ToLower(strings.TrimSpace(v))
	}
}

// DefaultAPIBase returns the built-in endpoint for a provider (empty if unknown/off).
func DefaultAPIBase(provider string) string {
	return defaultBase[NormalizeProvider(provider)]
}

// SettingAPIKey is the per-provider app_settings key for API key.
func SettingAPIKey(provider string) string {
	p := NormalizeProvider(provider)
	if p == "off" || p == "" {
		return ""
	}
	return "captcha." + p + ".api_key"
}

// SettingAPIBase is the per-provider app_settings key for API base override.
func SettingAPIBase(provider string) string {
	p := NormalizeProvider(provider)
	if p == "off" || p == "" {
		return ""
	}
	return "captcha." + p + ".api_base"
}

func (c Config) Resolved() Config {
	out := c
	out.Provider = NormalizeProvider(out.Provider)
	out.APIKey = strings.TrimSpace(out.APIKey)
	out.APIBase = strings.TrimRight(strings.TrimSpace(out.APIBase), "/")
	if out.APIBase == "" {
		out.APIBase = defaultBase[out.Provider]
	}
	if out.TimeoutSec < 30 {
		out.TimeoutSec = 180
	}
	if out.TimeoutSec > 600 {
		out.TimeoutSec = 600
	}
	if out.PollIntervalSec < 1 {
		out.PollIntervalSec = 3
	}
	if out.PollIntervalSec > 15 {
		out.PollIntervalSec = 15
	}
	return out
}

type SolveOpts struct {
	SiteKey  string
	PageURL  string
	Action   string
	Config   Config
	HTTP     *http.Client
	Log      func(msg, level string)
}

func SolveTurnstile(opts SolveOpts) (string, error) {
	cfg := opts.Config.Resolved()
	siteKey := strings.TrimSpace(opts.SiteKey)
	if siteKey == "" {
		return "", fmt.Errorf("缺少 Turnstile siteKey")
	}
	if cfg.Provider == "off" || cfg.Provider == "" {
		return "", fmt.Errorf("未配置打码：请在设置中选择 YesCaptcha / 2Captcha / CapSolver 并填写 Key")
	}
	if cfg.APIKey == "" {
		return "", fmt.Errorf("打码平台 %s 缺少 API Key", cfg.Provider)
	}
	if cfg.APIBase == "" {
		return "", fmt.Errorf("打码平台 %s 缺少 API Base", cfg.Provider)
	}
	pageURL := strings.TrimSpace(opts.PageURL)
	if pageURL == "" {
		pageURL = "https://www.runninghub.ai/zh-cn"
	}
	action := strings.TrimSpace(opts.Action)
	if action == "" {
		action = RHRegisterAction
	}
	log := opts.Log
	if log == nil {
		log = func(string, string) {}
	}
	client := opts.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	taskType := "TurnstileTaskProxyless"
	if cfg.Provider == ProviderCapSolver {
		taskType = "AntiTurnstileTaskProxyLess"
	}
	task := map[string]any{
		"type":       taskType,
		"websiteURL": pageURL,
		"websiteKey": siteKey,
	}
	switch cfg.Provider {
	case ProviderCapSolver:
		task["metadata"] = map[string]any{"action": action}
	case ProviderYesCaptcha:
		task["pageAction"] = action
	default:
		task["action"] = action
	}

	log(fmt.Sprintf("Turnstile 打码开始：provider=%s type=%s action=%s", cfg.Provider, taskType, action), "info")
	create, err := postJSON(client, cfg.APIBase+"/createTask", map[string]any{
		"clientKey": cfg.APIKey,
		"task":      task,
	})
	if err != nil {
		return "", err
	}
	if asInt(create["errorId"]) > 0 {
		return "", fmt.Errorf("打码 createTask 失败：%v", firstNonEmpty(asString(create["errorDescription"]), asString(create["errorCode"])))
	}
	taskID := create["taskId"]
	if taskID == nil || asString(taskID) == "" {
		// some APIs return numeric taskId
		if n := asInt(taskID); n > 0 {
			taskID = n
		} else {
			return "", fmt.Errorf("打码 createTask 未返回 taskId")
		}
	}
	log(fmt.Sprintf("Turnstile 任务已创建：taskId=%v，轮询中（超时 %ds）", taskID, cfg.TimeoutSec), "info")

	deadline := time.Now().Add(time.Duration(cfg.TimeoutSec) * time.Second)
	polls := 0
	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(cfg.PollIntervalSec) * time.Second)
		polls++
		poll, err := postJSON(client, cfg.APIBase+"/getTaskResult", map[string]any{
			"clientKey": cfg.APIKey,
			"taskId":    taskID,
		})
		if err != nil {
			return "", err
		}
		if asInt(poll["errorId"]) > 0 {
			return "", fmt.Errorf("打码 getTaskResult 失败：%v", firstNonEmpty(asString(poll["errorDescription"]), asString(poll["errorCode"])))
		}
		if asString(poll["status"]) == "ready" {
			sol, _ := poll["solution"].(map[string]any)
			token := firstNonEmpty(
				asString(sol["token"]),
				asString(sol["cfToken"]),
				asString(sol["gRecaptchaResponse"]),
				asString(sol["response"]),
			)
			if token == "" {
				return "", fmt.Errorf("打码 ready 但无 token")
			}
			log(fmt.Sprintf("Turnstile 打码成功：provider=%s polls=%d", cfg.Provider, polls), "ok")
			return token, nil
		}
		if polls == 1 || polls%5 == 0 {
			log(fmt.Sprintf("Turnstile 等待中：taskId=%v status=%s polls=%d", taskID, asString(poll["status"]), polls), "info")
		}
	}
	return "", fmt.Errorf("打码超时（%ds） taskId=%v", cfg.TimeoutSec, taskID)
}

func postJSON(client *http.Client, url string, body any) (map[string]any, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		return nil, fmt.Errorf("打码平台非 JSON：%s", truncate(string(b), 200))
	}
	return decoded, nil
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	case json.Number:
		return t.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	case string:
		var i int
		fmt.Sscanf(t, "%d", &i)
		return i
	default:
		return 0
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
