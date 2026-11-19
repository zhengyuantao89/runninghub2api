package store

import (
	"path/filepath"
	"testing"
)

func TestUserSettingsPreferUserThenGlobal(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.SetSetting("captcha.provider", "yescaptcha"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUserSetting(7, "captcha.provider", "2captcha"); err != nil {
		t.Fatal(err)
	}

	v, err := s.GetSettingPreferUser(7, "captcha.provider")
	if err != nil {
		t.Fatal(err)
	}
	if v != "2captcha" {
		t.Fatalf("prefer user: got %q", v)
	}

	v, err = s.GetSettingPreferUser(8, "captcha.provider")
	if err != nil {
		t.Fatal(err)
	}
	if v != "yescaptcha" {
		t.Fatalf("fallback global: got %q", v)
	}

	v, err = s.GetUserSetting(8, "captcha.provider")
	if err != nil {
		t.Fatal(err)
	}
	if v != "" {
		t.Fatalf("user-only empty: got %q", v)
	}
}
