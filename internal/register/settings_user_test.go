package register

import (
	"path/filepath"
	"testing"

	"github.com/trae/runninghub2api/internal/store"
)

func TestLoadCaptchaConfigForUserPreferThenFallback(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := NewService(st, "")
	_ = st.SetSetting(SettingCaptchaProvider, "yescaptcha")
	_ = SetCaptchaCreds(st, "yescaptcha", "global-key", "")
	_ = st.SetSetting(SettingRegisterInvite, "GINVITE")

	cfg := svc.LoadCaptchaConfigForUser(3)
	if cfg.Provider != "yescaptcha" || cfg.APIKey != "global-key" {
		t.Fatalf("fallback global: %+v", cfg)
	}
	if svc.DefaultInviteCodeForUser(3) != "GINVITE" {
		t.Fatalf("invite fallback: %q", svc.DefaultInviteCodeForUser(3))
	}

	_ = st.SetUserSetting(3, SettingCaptchaProvider, "2captcha")
	_ = SetCaptchaCredsForUser(st, 3, "2captcha", "user-key", "")
	_ = st.SetUserSetting(3, SettingRegisterInvite, "UINVITE")

	cfg = svc.LoadCaptchaConfigForUser(3)
	if cfg.Provider != "2captcha" || cfg.APIKey != "user-key" {
		t.Fatalf("prefer user: %+v", cfg)
	}
	if svc.DefaultInviteCodeForUser(3) != "UINVITE" {
		t.Fatalf("invite user: %q", svc.DefaultInviteCodeForUser(3))
	}

	key, _ := LoadCaptchaCredsUserOnly(st, 3, "yescaptcha")
	if key != "" {
		t.Fatalf("user-only must not leak other provider global key: %q", key)
	}
	gKey, _ := LoadCaptchaCreds(st, "yescaptcha", false)
	if gKey != "global-key" {
		t.Fatalf("global key intact: %q", gKey)
	}
}
