package accounts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPickFingerprintStickyJSON(t *testing.T) {
	fp := PickFingerprint()
	if fp.ProfileID != "edge157-win" {
		t.Fatalf("profile %s", fp.ProfileID)
	}
	if !strings.Contains(fp.UserAgent, "Edg/157") {
		t.Fatalf("ua %s", fp.UserAgent)
	}
	raw := MustFingerprintJSON(fp)
	if !strings.Contains(raw, "sec_fetch_site") {
		t.Fatalf("missing sec_fetch: %s", raw)
	}
	got := ParseFingerprintJSON(raw)
	if got.UserAgent != fp.UserAgent || got.SecChUA == "" || got.SecFetchMode != "cors" {
		t.Fatalf("roundtrip %#v", got)
	}
}

func TestParseLegacyFingerprintMap(t *testing.T) {
	legacy, _ := json.Marshal(map[string]any{
		"user_agent": "UA-LEGACY",
		"lang":       "zh-CN",
		"sec_ch_ua":  `"Chromium";v="120"`,
		"source":     "legacy",
	})
	fp := ParseFingerprintJSON(string(legacy))
	if fp.UserAgent != "UA-LEGACY" {
		t.Fatalf("ua %s", fp.UserAgent)
	}
	if fp.SecFetchSite != "same-origin" {
		t.Fatalf("default sec-fetch %s", fp.SecFetchSite)
	}
}
