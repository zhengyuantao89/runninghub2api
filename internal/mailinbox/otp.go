package mailinbox

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	digitRun   = regexp.MustCompile(`^\d{4,8}$`)
	digitAny   = regexp.MustCompile(`\d{4,8}`)
	rhHint     = regexp.MustCompile(`(?i)runninghub|xiaoyaoyou|verification|otp|验证码|one[- ]?time`)
	authFailRe = regexp.MustCompile(`(?i)unauthorized|forbidden|invalid.?token|auth|鉴权|过期|expired|401|403`)
)

type WaitOpts struct {
	PickupURL string
	Timeout   time.Duration
	Interval  time.Duration
	HTTP      *http.Client
	Log       func(msg, level string)
	Cancelled func() bool
	// Baseline / Baselines are OTP values already on the mailbox before/around
	// sendCode. WaitOTP ignores these until a different 4–8 digit code appears.
	Baseline  string
	Baselines []string
}

// WaitOTP polls the Inbox pickup URL until a new 4–8 digit code appears
// (different from any Baseline / Baselines entry).
func WaitOTP(opts WaitOpts) (string, error) {
	url := strings.TrimSpace(opts.PickupURL)
	if url == "" {
		return "", fmt.Errorf("缺少取件链接")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Minute
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = 3 * time.Second
	}
	client := opts.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	log := opts.Log
	if log == nil {
		log = func(string, string) {}
	}
	cancelled := opts.Cancelled
	if cancelled == nil {
		cancelled = func() bool { return false }
	}

	ignore := map[string]struct{}{}
	addIgnore := func(v string) {
		v = strings.TrimSpace(v)
		if v != "" {
			ignore[v] = struct{}{}
		}
	}
	addIgnore(opts.Baseline)
	for _, b := range opts.Baselines {
		addIgnore(b)
	}
	if len(ignore) > 0 {
		log(fmt.Sprintf("取件基线已记录（忽略旧码 %d 个）", len(ignore)), "info")
	}

	deadline := time.Now().Add(timeout)
	seenEmpty := 0
	seenStale := 0
	for time.Now().Before(deadline) {
		if cancelled() {
			return "", fmt.Errorf("任务已取消")
		}
		code, terminal, err := fetchOTPOnce(client, url)
		if err != nil {
			if terminal {
				return "", err
			}
			log(fmt.Sprintf("取件暂失败：%v", err), "warning")
		} else if code != "" {
			if _, stale := ignore[code]; stale {
				seenStale++
				if seenStale == 1 || seenStale%5 == 0 {
					log(fmt.Sprintf("取件仍为发码前旧验证码，继续等待新码 n=%d", seenStale), "info")
				}
			} else {
				log(fmt.Sprintf("已取到新验证码（len=%d）", len(code)), "info")
				return code, nil
			}
		} else {
			seenEmpty++
			if seenEmpty == 1 || seenEmpty%5 == 0 {
				log(fmt.Sprintf("取件轮询中（暂无验证码）n=%d", seenEmpty), "info")
			}
		}
		time.Sleep(interval)
	}
	return "", fmt.Errorf("等待邮箱验证码超时（%s）", timeout)
}

// Probe does a single GET on the pickup URL and returns any OTP found.
func Probe(pickupURL string) (code string, terminal bool, err error) {
	client := &http.Client{Timeout: 20 * time.Second}
	return fetchOTPOnce(client, strings.TrimSpace(pickupURL))
}

func fetchOTPOnce(client *http.Client, pickupURL string) (code string, terminal bool, err error) {
	req, err := http.NewRequest(http.MethodGet, pickupURL, nil)
	if err != nil {
		return "", true, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	res, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	text := string(raw)
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return "", true, fmt.Errorf("取件链接鉴权失效 HTTP %d", res.StatusCode)
	}
	if res.StatusCode >= 400 {
		if authFailRe.MatchString(text) {
			return "", true, fmt.Errorf("取件链接鉴权失效 HTTP %d", res.StatusCode)
		}
		return "", false, fmt.Errorf("取件 HTTP %d", res.StatusCode)
	}
	if authFailRe.MatchString(text) && !digitAny.MatchString(text) {
		return "", true, fmt.Errorf("取件链接鉴权失效")
	}

	// Preferred: structured Inbox JSON {success, code, ...}
	if c, ok := parseInboxCodeJSON(raw); ok {
		return c, false, nil
	}

	blob := text
	var parsed any
	if json.Unmarshal(raw, &parsed) == nil {
		blob = flattenJSON(parsed) + "\n" + text
	}
	blob = stripHTML(blob)
	return extractOTP(blob), false, nil
}

// parseInboxCodeJSON handles tibosb-style pickup responses:
//
//	{"success":false,"code":"no_code",...} → empty
//	{"success":true,"code":"123456",...}   → "123456"
//
// Requires boolean success==true before accepting digits.
// ok=false means the body is not this shape; fall back to extractOTP.
func parseInboxCodeJSON(raw []byte) (code string, ok bool) {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return "", false
	}
	success, hasSuccess := m["success"].(bool)
	_, hasCode := m["code"]
	if !hasSuccess || !hasCode {
		return "", false
	}
	if !success {
		return "", true
	}
	rawCode := strings.TrimSpace(fmt.Sprint(m["code"]))
	if rawCode == "" || strings.EqualFold(rawCode, "no_code") || rawCode == "<nil>" {
		return "", true
	}
	if !digitRun.MatchString(rawCode) {
		return "", true
	}
	return rawCode, true
}

func extractOTP(blob string) string {
	if blob == "" {
		return ""
	}
	lower := strings.ToLower(blob)
	if loc := rhHint.FindStringIndex(lower); loc != nil {
		window := blob[max(0, loc[0]-80):min(len(blob), loc[1]+120)]
		if m := digitAny.FindString(window); m != "" {
			return m
		}
	}
	for _, key := range []string{`"code"`, `"otp"`, `"verification_code"`, `"verifyCode"`, `"验证码"`} {
		i := strings.Index(strings.ToLower(blob), strings.ToLower(key))
		if i < 0 {
			continue
		}
		chunk := blob[i:min(len(blob), i+64)]
		// Skip known non-OTP tokens like "no_code"
		if strings.Contains(strings.ToLower(chunk), "no_code") {
			continue
		}
		if m := digitAny.FindString(chunk); m != "" {
			return m
		}
	}
	if m := digitAny.FindString(blob); m != "" {
		return m
	}
	return ""
}

func flattenJSON(v any) string {
	var b strings.Builder
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for k, vv := range t {
				b.WriteString(k)
				b.WriteByte(' ')
				walk(vv)
			}
		case []any:
			for _, vv := range t {
				walk(vv)
			}
		case string:
			b.WriteString(t)
			b.WriteByte(' ')
		case float64:
			b.WriteString(fmt.Sprintf("%.0f ", t))
		case bool:
			b.WriteString(fmt.Sprintf("%v ", t))
		case nil:
		default:
			b.WriteString(fmt.Sprint(t))
			b.WriteByte(' ')
		}
	}
	walk(v)
	return b.String()
}

func stripHTML(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(s, " ")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
