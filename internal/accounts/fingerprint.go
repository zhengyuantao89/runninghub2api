package accounts

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// BrowserFingerprint is a sticky browser persona for one RH signup/account session.
// Shape mirrors muse's HAR-derived profiles, but Origin/Referer stay owned by the RH client BaseURL.
type BrowserFingerprint struct {
	ProfileID               string `json:"profile_id"`
	UserAgent               string `json:"user_agent"`
	SecChUA                 string `json:"sec_ch_ua"`
	SecChUAMobile           string `json:"sec_ch_ua_mobile"`
	SecChUAPlatform         string `json:"sec_ch_ua_platform"`
	SecChUAFullVersionList  string `json:"sec_ch_ua_full_version_list"`
	SecChPrefersColorScheme string `json:"sec_ch_prefers_color_scheme"`
	Lang                    string `json:"lang"`
	LangFull                string `json:"lang_full"`
	Accept                  string `json:"accept,omitempty"`
	SecFetchSite            string `json:"sec_fetch_site"`
	SecFetchMode            string `json:"sec_fetch_mode"`
	SecFetchDest            string `json:"sec_fetch_dest"`
	LabelZH                 string `json:"label_zh,omitempty"`
	Source                  string `json:"source,omitempty"`
}

// edge157Win matches 参考/runninghub11注册.devtools successful /uc/email/register.
var edge157Win = BrowserFingerprint{
	ProfileID: "edge157-win",
	UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.0.0 Safari/537.36 Edg/157.0.0.0",
	SecChUA:   `"Not-A.Brand";v="99", "Microsoft Edge";v="157", "Chromium";v="157"`,
	SecChUAMobile:   "?0",
	SecChUAPlatform: `"Windows"`,
	SecChUAFullVersionList: `"Not-A.Brand";v="99.0.0.0", "Microsoft Edge";v="157.0.0.0", "Chromium";v="157.0.0.0"`,
	SecChPrefersColorScheme: "light",
	Lang:         "zh-CN",
	LangFull:     "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
	Accept:       "application/json, text/plain, */*",
	SecFetchSite: "same-origin",
	SecFetchMode: "cors",
	SecFetchDest: "empty",
	LabelZH:      "Windows Edge 157",
	Source:       "har-runninghub11",
}

// chrome156Win kept as alias for older stored fingerprints / tests.
var chrome156Win = edge157Win

// PickFingerprint returns the registration-default persona (cloned via JSON).
func PickFingerprint() BrowserFingerprint {
	return chrome156Win
}

// DefaultFingerprintJSON returns sticky Chrome-Win fingerprint JSON for RH headers.
func DefaultFingerprintJSON() string {
	return MustFingerprintJSON(PickFingerprint())
}

// MustFingerprintJSON marshals fp; empty on error.
func MustFingerprintJSON(fp BrowserFingerprint) string {
	if fp.ProfileID == "" {
		fp = PickFingerprint()
	}
	if fp.Source == "" {
		fp.Source = "builtin"
	}
	b, err := json.Marshal(fp)
	if err != nil {
		return ""
	}
	return string(b)
}

// ParseFingerprintJSON restores a persona; falls back to default when invalid.
func ParseFingerprintJSON(raw string) BrowserFingerprint {
	raw = stringTrim(raw)
	if raw == "" {
		return PickFingerprint()
	}
	var fp BrowserFingerprint
	if err := json.Unmarshal([]byte(raw), &fp); err != nil || fp.UserAgent == "" {
		// Legacy map shape from older DefaultFingerprintJSON.
		var m map[string]any
		if err2 := json.Unmarshal([]byte(raw), &m); err2 != nil {
			return PickFingerprint()
		}
		fp = BrowserFingerprint{
			ProfileID:              strMap(m, "profile_id", "chrome156-win"),
			UserAgent:              strMap(m, "user_agent", ""),
			SecChUA:                strMap(m, "sec_ch_ua", ""),
			SecChUAMobile:          strMap(m, "sec_ch_ua_mobile", "?0"),
			SecChUAPlatform:        strMap(m, "sec_ch_ua_platform", `"Windows"`),
			SecChUAFullVersionList: strMap(m, "sec_ch_ua_full_version_list", ""),
			SecChPrefersColorScheme: strMap(m, "sec_ch_prefers_color_scheme", "light"),
			Lang:                   strMap(m, "lang", "zh-CN"),
			LangFull:               firstStr(strMap(m, "lang_full", ""), strMap(m, "accept_language", ""), "zh-CN,zh;q=0.9,en;q=0.8"),
			Accept:                 strMap(m, "accept", "application/json, text/plain, */*"),
			SecFetchSite:           strMap(m, "sec_fetch_site", "same-origin"),
			SecFetchMode:           strMap(m, "sec_fetch_mode", "cors"),
			SecFetchDest:           strMap(m, "sec_fetch_dest", "empty"),
			LabelZH:                strMap(m, "label_zh", ""),
			Source:                 strMap(m, "source", "legacy"),
		}
		if fp.UserAgent == "" {
			return PickFingerprint()
		}
		return fp
	}
	if fp.SecChUAMobile == "" {
		fp.SecChUAMobile = "?0"
	}
	if fp.SecFetchSite == "" {
		fp.SecFetchSite = "same-origin"
	}
	if fp.SecFetchMode == "" {
		fp.SecFetchMode = "cors"
	}
	if fp.SecFetchDest == "" {
		fp.SecFetchDest = "empty"
	}
	return fp
}

func stringTrim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c != ' ' && c != '\n' && c != '\t' && c != '\r' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

func strMap(m map[string]any, key, fallback string) string {
	if m == nil {
		return fallback
	}
	v, ok := m[key]
	if !ok || v == nil {
		return fallback
	}
	switch t := v.(type) {
	case string:
		if t == "" {
			return fallback
		}
		return t
	default:
		s := fmt.Sprint(t)
		if s == "" || s == "<nil>" {
			return fallback
		}
		return s
	}
}

func firstStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func RandomPassword(n int) string {
	if n < 8 {
		n = 12
	}
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%"
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(out)
}

// RandomDeviceID returns a 64-hex deviceId like the RH SPA (FingerprintJS/hash style).
func RandomDeviceID() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RandomAnonymousID returns an x-rh-anonymous-id style UUID.
func RandomAnonymousID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

var nickFirst = []string{
	"Daver", "Mira", "Kelan", "Soren", "Nova", "Riven", "Elara", "Joris", "Tamsin", "Orin",
	"Liora", "Casen", "Vesper", "Harlan", "Nyra", "Quill", "Aspen", "Bram", "Cleo", "Dax",
}
var nickLast = []string{
	"yse", "vale", "ford", "lane", "beck", "moss", "reed", "wynn", "hale", "croft",
	"ash", "beck", "bloom", "pike", "sol", "glen", "brook", "crest", "dawn", "field",
}

// RandomNickName returns a short "First last" display name like the RH SPA sample.
func RandomNickName() string {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return nickFirst[int(b[0])%len(nickFirst)] + " " + nickLast[int(b[1])%len(nickLast)]
}
