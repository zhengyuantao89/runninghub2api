package runninghub

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	BaseURL      string
	AnonymousID  string
	AccessToken  string
	HTTP         *http.Client
	UserAgent    string
	ExtraHeaders map[string]string
}

type APIError struct {
	Path   string
	Code   any
	Msg    string
	Status int
	Data   any
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s failed: code=%v msg=%s", e.Path, e.Code, e.Msg)
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		AnonymousID:  randomUUIDLike(),
		HTTP:         &http.Client{Timeout: 120 * time.Second},
		UserAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
		ExtraHeaders: map[string]string{},
	}
}

// ApplyIdentity sets anonymous_id + browser fingerprint headers from push payload.
func (c *Client) ApplyIdentity(anonymousID, fingerprintJSON string) {
	if id := strings.TrimSpace(anonymousID); id != "" {
		c.AnonymousID = id
	}
	raw := strings.TrimSpace(fingerprintJSON)
	if raw == "" {
		return
	}
	var fp map[string]any
	if err := json.Unmarshal([]byte(raw), &fp); err != nil || fp == nil {
		return
	}
	if ua := strings.TrimSpace(asString(fp["user_agent"])); ua != "" {
		c.UserAgent = ua
	}
	if c.ExtraHeaders == nil {
		c.ExtraHeaders = map[string]string{}
	}
	lang := strings.TrimSpace(asString(fp["lang_full"]))
	if lang == "" {
		lang = strings.TrimSpace(asString(fp["lang"]))
	}
	if lang != "" {
		c.ExtraHeaders["Accept-Language"] = lang
	}
	if v := strings.TrimSpace(asString(fp["sec_ch_ua"])); v != "" {
		c.ExtraHeaders["sec-ch-ua"] = v
		c.ExtraHeaders["sec-ch-ua-mobile"] = firstNonEmpty(asString(fp["sec_ch_ua_mobile"]), "?0")
		c.ExtraHeaders["sec-ch-ua-platform"] = firstNonEmpty(asString(fp["sec_ch_ua_platform"]), `"Windows"`)
		if full := strings.TrimSpace(asString(fp["sec_ch_ua_full_version_list"])); full != "" {
			c.ExtraHeaders["sec-ch-ua-full-version-list"] = full
		}
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func MD5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (c *Client) SetAccessToken(token string) { c.AccessToken = token }

func (c *Client) headers(extra map[string]string) http.Header {
	h := http.Header{}
	h.Set("Accept", "application/json, text/plain, */*")
	h.Set("Content-Type", "application/json")
	h.Set("client", "WEB")
	h.Set("user-language", "zh_CN")
	h.Set("x-rh-anonymous-id", c.AnonymousID)
	h.Set("x-rh-lang", "zh")
	h.Set("Origin", c.BaseURL)
	h.Set("Referer", c.BaseURL+"/zh-cn")
	h.Set("User-Agent", c.UserAgent)
	for k, v := range c.ExtraHeaders {
		if strings.TrimSpace(v) != "" {
			h.Set(k, v)
		}
	}
	if c.AccessToken != "" {
		h.Set("Authorization", "Bearer "+c.AccessToken)
	}
	for k, v := range extra {
		h.Set(k, v)
	}
	return h
}

func (c *Client) postJSON(path string, body any, allowCodes ...int) (map[string]any, error) {
	if len(allowCodes) == 0 {
		allowCodes = []int{0}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header = c.headers(nil)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("non-json HTTP %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	if res.StatusCode >= 400 {
		return nil, &APIError{Path: path, Status: res.StatusCode, Code: decoded["code"], Msg: asString(decoded["msg"]), Data: decoded["data"]}
	}
	code := asInt(decoded["code"])
	for _, ok := range allowCodes {
		if code == ok {
			return decoded, nil
		}
	}
	return nil, &APIError{Path: path, Status: res.StatusCode, Code: decoded["code"], Msg: asString(decoded["msg"]), Data: decoded["data"]}
}

func (c *Client) PwdLogin(email, passwordPlain string) (map[string]any, error) {
	return c.postJSON("/uc/abroad/pwdLogin", map[string]any{
		"email":    email,
		"password": MD5Hex(passwordPlain),
	})
}

func (c *Client) GetUserInfo(userID string) (map[string]any, error) {
	return c.postJSON("/uc/getUserInfo", map[string]any{"userId": userID})
}

// PointSummary is the NEW billing wallet snapshot (billingVersion / balancePoint).
func (c *Client) PointSummary() (map[string]any, error) {
	return c.postJSON("/uc/point/summary", map[string]any{})
}

// WelfareSummary returns daily gift / free quota status. issue=true refreshes after claim.
func (c *Client) WelfareSummary(issue bool) (map[string]any, error) {
	return c.postJSON("/api/welfare/summary", map[string]any{"issue": issue})
}

// ClaimDailyGift claims NEW-system daily points (POST /api/welfare/daily-gift/claim).
func (c *Client) ClaimDailyGift() (map[string]any, error) {
	return c.postJSON("/api/welfare/daily-gift/claim", map[string]any{})
}

// CreateConsumerApiKey needs JWT; allowCodes includes 301 (already exists).
func (c *Client) CreateConsumerApiKey() (map[string]any, error) {
	return c.postJSON("/uc/apiKey/createConsumer", map[string]any{}, 0, 301)
}

// ResetConsumerApiKey returns a new plaintext consumer key (GET /uc/openapi/reset).
func (c *Client) ResetConsumerApiKey() (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/uc/openapi/reset", nil)
	if err != nil {
		return nil, err
	}
	req.Header = c.headers(nil)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("reset non-json HTTP %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	if res.StatusCode >= 400 || asInt(decoded["code"]) != 0 {
		return nil, &APIError{Path: "/uc/openapi/reset", Status: res.StatusCode, Code: decoded["code"], Msg: asString(decoded["msg"]), Data: decoded["data"]}
	}
	return decoded, nil
}

// ParseConsumerApiKey extracts a 32-hex consumer key from create/reset payloads.
func ParseConsumerApiKey(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	raw := payload["data"]
	switch t := raw.(type) {
	case string:
		key := strings.TrimSpace(t)
		if isConsumerKey(key) {
			return key
		}
	case map[string]any:
		for _, k := range []string{"apiKey", "api_key", "apikey"} {
			key := strings.TrimSpace(asString(t[k]))
			if isConsumerKey(key) {
				return key
			}
		}
	}
	key := strings.TrimSpace(asString(raw))
	if isConsumerKey(key) {
		return key
	}
	return ""
}

func isConsumerKey(key string) bool {
	if len(key) != 32 {
		return false
	}
	for _, ch := range key {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') && (ch < 'A' || ch > 'F') {
			return false
		}
	}
	return true
}

// EnsureKeyResult is the outcome of login + create/reset consumer api key.
type EnsureKeyResult struct {
	APIKey       string
	AccessToken  string
	RefreshToken string
	Source       string // create | reset
	Relogged     bool
}

// EnsureConsumerApiKey logs in if needed, createConsumer then reset fallback; returns plaintext key + tokens.
func (c *Client) EnsureConsumerApiKey(email, password, accessToken string) (EnsureKeyResult, error) {
	var out EnsureKeyResult
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)
	accessToken = strings.TrimSpace(accessToken)
	login := func() error {
		if email == "" || password == "" {
			return fmt.Errorf("email+password required to obtain api_key")
		}
		loginRes, e := c.PwdLogin(email, password)
		if e != nil {
			return e
		}
		token := asString(nested(loginRes, "data", "access_token"))
		if token == "" {
			token = asString(nested(loginRes, "data", "accessToken"))
		}
		if token == "" {
			return fmt.Errorf("login missing access_token")
		}
		c.SetAccessToken(token)
		out.AccessToken = token
		out.RefreshToken = firstNonEmpty(
			asString(nested(loginRes, "data", "refresh_token")),
			asString(nested(loginRes, "data", "refreshToken")),
		)
		out.Relogged = true
		return nil
	}
	if accessToken != "" {
		c.SetAccessToken(accessToken)
	} else if err := login(); err != nil {
		return out, err
	}

	obtain := func() (string, string, error) {
		created, e := c.CreateConsumerApiKey()
		if e == nil {
			if k := ParseConsumerApiKey(created); k != "" {
				return k, "create", nil
			}
		}
		reset, e2 := c.ResetConsumerApiKey()
		if e2 != nil {
			if e != nil {
				return "", "", fmt.Errorf("createConsumer: %v; reset: %v", e, e2)
			}
			return "", "", e2
		}
		k := ParseConsumerApiKey(reset)
		if k == "" {
			return "", "", fmt.Errorf("reset returned empty api_key")
		}
		return k, "reset", nil
	}

	key, source, err := obtain()
	if err != nil && password != "" {
		// token may be stale — re-login once
		if e := login(); e == nil {
			key, source, err = obtain()
		}
	}
	if err != nil {
		return out, err
	}
	out.APIKey = key
	out.Source = source
	return out, nil
}

func (c *Client) InstanceAccessAuth() (map[string]any, error) {
	return c.postJSON("/api/instance/access/auth", map[string]any{})
}

func (c *Client) WorkValid(webappID string) (map[string]any, error) {
	return c.postJSON("/api/user/task/work/valid", map[string]any{
		"taskType":     "WEBAPP",
		"id":           webappID,
		"taskCode":     nil,
		"instanceType": "default",
	})
}

// WebappDetail fetches AI-app metadata + inputNodes (public RH endpoint; login optional).
func (c *Client) WebappDetail(webappID string) (map[string]any, error) {
	webappID = strings.TrimSpace(webappID)
	if webappID == "" {
		return nil, fmt.Errorf("webappId required")
	}
	return c.postJSON("/api/webapp/detail", map[string]any{"webappId": webappID})
}

// ApiCallDemo is the official RH demo endpoint (requires consumer apiKey).
func (c *Client) ApiCallDemo(apiKey, webappID string) (map[string]any, error) {
	key := strings.TrimSpace(apiKey)
	webappID = strings.TrimSpace(webappID)
	if key == "" {
		return nil, fmt.Errorf("apiKey required")
	}
	if webappID == "" {
		return nil, fmt.Errorf("webappId required")
	}
	u := fmt.Sprintf("%s/api/webapp/apiCallDemo?apiKey=%s&webappId=%s",
		c.BaseURL, url.QueryEscape(key), url.QueryEscape(webappID))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header = c.headers(nil)
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("apiCallDemo non-json HTTP %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	if res.StatusCode >= 400 || asInt(decoded["code"]) != 0 {
		return nil, &APIError{Path: "/api/webapp/apiCallDemo", Status: res.StatusCode, Code: decoded["code"], Msg: asString(decoded["msg"]), Data: decoded["data"]}
	}
	return decoded, nil
}

func (c *Client) SensitiveCheck(prompt string) (map[string]any, error) {
	return c.postJSON("/api/prompt/sensitive/check", map[string]any{"prompt": prompt})
}

func (c *Client) WebappCreate(body map[string]any) (map[string]any, error) {
	return c.postJSON("/task/webapp/create", body)
}

func (c *Client) TaskList(body map[string]any) (map[string]any, error) {
	return c.postJSON("/task/list", body)
}

func (c *Client) OutputHistoryV2(body map[string]any) (map[string]any, error) {
	return c.postJSON("/api/output/v2/history", body)
}

// OpenAPITaskStatus queries RH official task status with consumer apiKey.
// POST /task/openapi/status → data is QUEUED|RUNNING|FAILED|SUCCESS.
func (c *Client) OpenAPITaskStatus(apiKey, taskID string) (string, error) {
	key := strings.TrimSpace(apiKey)
	taskID = strings.TrimSpace(taskID)
	if key == "" || taskID == "" {
		return "", fmt.Errorf("apiKey and taskId required")
	}
	payload, err := json.Marshal(map[string]any{"apiKey": key, "taskId": taskID})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/task/openapi/status", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header = c.headers(nil)
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("openapi/status non-json HTTP %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	if res.StatusCode >= 400 || asInt(decoded["code"]) != 0 {
		return "", &APIError{Path: "/task/openapi/status", Status: res.StatusCode, Code: decoded["code"], Msg: asString(decoded["msg"]), Data: decoded["data"]}
	}
	return asString(decoded["data"]), nil
}

// OpenAPITaskOutputs queries RH official task outputs with consumer apiKey.
func (c *Client) OpenAPITaskOutputs(apiKey, taskID string) (map[string]any, error) {
	key := strings.TrimSpace(apiKey)
	taskID = strings.TrimSpace(taskID)
	if key == "" || taskID == "" {
		return nil, fmt.Errorf("apiKey and taskId required")
	}
	payload, err := json.Marshal(map[string]any{"apiKey": key, "taskId": taskID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/task/openapi/outputs", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header = c.headers(nil)
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("openapi/outputs non-json HTTP %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	if res.StatusCode >= 400 || asInt(decoded["code"]) != 0 {
		return nil, &APIError{Path: "/task/openapi/outputs", Status: res.StatusCode, Code: decoded["code"], Msg: asString(decoded["msg"]), Data: decoded["data"]}
	}
	return decoded, nil
}

// AccountStatus uses consumer apiKey (Bearer + body.apikey) to query remainCoins / currentTaskCounts.
func (c *Client) AccountStatus(apiKey string) (map[string]any, error) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return nil, fmt.Errorf("apiKey required")
	}
	payload, err := json.Marshal(map[string]any{"apikey": key})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/uc/openapi/accountStatus", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header = c.headers(nil)
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("accountStatus non-json HTTP %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	if res.StatusCode >= 400 || asInt(decoded["code"]) != 0 {
		return nil, &APIError{Path: "/uc/openapi/accountStatus", Status: res.StatusCode, Code: decoded["code"], Msg: asString(decoded["msg"]), Data: decoded["data"]}
	}
	return decoded, nil
}

func (c *Client) UploadImage(data []byte, filename, webappID string) (map[string]any, error) {
	auth, err := c.InstanceAccessAuth()
	if err != nil {
		return nil, err
	}
	accessKey := asString(nested(auth, "data", "accessKey"))
	if accessKey == "" {
		return nil, fmt.Errorf("instanceAccessAuth missing accessKey")
	}
	identify := ""
	if raw, err := base64.StdEncoding.DecodeString(accessKey); err == nil {
		var decoded map[string]any
		if json.Unmarshal(raw, &decoded) == nil {
			identify = asString(decoded["userId"])
		}
	}
	if identify == "" {
		return nil, fmt.Errorf("cannot parse rh-identify from accessKey")
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("image", filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}
	_ = w.Close()

	referer := c.BaseURL + "/zh-cn"
	if webappID != "" {
		referer = c.BaseURL + "/zh-cn/ai-detail/" + webappID
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/upload/image", &buf)
	if err != nil {
		return nil, err
	}
	req.Header = c.headers(map[string]string{
		"Content-Type":  w.FormDataContentType(),
		"rh-comfy-auth": accessKey,
		"rh-identify":   identify,
		"Referer":       referer,
	})
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("upload non-json HTTP %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("upload HTTP %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	return decoded, nil
}

func decodeJWTClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	payload := parts[1]
	if m := len(payload) % 4; m != 0 {
		payload += strings.Repeat("=", 4-m)
	}
	raw, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil
		}
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

func DecodeJWTSub(token string) string {
	m := decodeJWTClaims(token)
	if m == nil {
		return ""
	}
	if s := asString(m["sub"]); s != "" {
		return s
	}
	return asString(m["userId"])
}

// DecodeJWTExp returns the JWT exp claim as UTC time, or zero if missing/invalid.
func DecodeJWTExp(token string) time.Time {
	m := decodeJWTClaims(token)
	if m == nil {
		return time.Time{}
	}
	switch v := m["exp"].(type) {
	case float64:
		if v <= 0 {
			return time.Time{}
		}
		return time.Unix(int64(v), 0).UTC()
	case json.Number:
		n, err := v.Int64()
		if err != nil || n <= 0 {
			return time.Time{}
		}
		return time.Unix(n, 0).UTC()
	case string:
		n, err := json.Number(v).Int64()
		if err != nil || n <= 0 {
			return time.Time{}
		}
		return time.Unix(n, 0).UTC()
	default:
		return time.Time{}
	}
}

// AccessTokenFresh is true when token is present and either has no exp claim
// (non-JWT / unknown) or expires after now+skew. Missing exp is treated as
// usable to avoid login storms on opaque tokens.
func AccessTokenFresh(token string, skew time.Duration) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	exp := DecodeJWTExp(token)
	if exp.IsZero() {
		return true
	}
	if skew < 0 {
		skew = 0
	}
	return time.Now().UTC().Add(skew).Before(exp)
}

func guessContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/png"
	}
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case float64:
		return fmt.Sprintf("%.0f", t)
	case json.Number:
		return t.String()
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
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
		return -1
	}
}

func nested(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = obj[k]
	}
	return cur
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func randomUUIDLike() string {
	sum := md5.Sum([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	h := hex.EncodeToString(sum[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}
