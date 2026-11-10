package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr          string
	AdminUser     string
	AdminPassword string
	APIKey        string
	DataDir       string
	WebDir        string
	BaseURL       string
	WebappID      string
	Workers       int
	// SubmitGapSec: minimum seconds between two RH submits on the same account (default 3).
	SubmitGapSec int
	// PollAuth: "apikey" (RH openapi status/outputs) or "at" (Bearer access_token task/list).
	PollAuth string

	// Legacy aliases kept so old .env / gpt-plus tokens still work until rotated.
	AuthKey   string
	AdminKey  string
	SyncToken string
}

// NormalizePollAuth returns "apikey" or "at".
func NormalizePollAuth(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "at", "access_token", "token":
		return "at"
	default:
		return "apikey"
	}
}

func Load() Config {
	workers, _ := strconv.Atoi(env("RH2API_WORKERS", "2"))
	if workers < 1 {
		workers = 1
	}
	submitGapSec, _ := strconv.Atoi(env("RH2API_SUBMIT_GAP_SEC", "3"))
	if submitGapSec < 0 {
		submitGapSec = 0
	}

	adminKey := env("RH2API_ADMIN_KEY", "")
	authKey := env("RH2API_AUTH_KEY", "")
	syncToken := env("RH2API_SYNC_TOKEN", "")
	if syncToken == "" {
		syncToken = adminKey
	}
	apiKey := env("RH2API_API_KEY", "")
	if apiKey == "" {
		apiKey = authKey
	}
	if apiKey == "" {
		apiKey = syncToken
	}
	if apiKey == "" {
		apiKey = adminKey
	}

	adminPassword := env("RH2API_ADMIN_PASSWORD", "admin123")

	return Config{
		Addr:          env("RH2API_ADDR", ":8088"),
		AdminUser:     env("RH2API_ADMIN_USER", "admin"),
		AdminPassword: adminPassword,
		APIKey:        apiKey,
		DataDir:       env("RH2API_DATA_DIR", "./data"),
		WebDir:        env("RH2API_WEB_DIR", "./web/dist"),
		BaseURL:       strings.TrimRight(env("RH2API_BASE_URL", "https://www.runninghub.ai"), "/"),
		WebappID:      env("RH2API_WEBAPP_ID", "2099427470494662657"),
		Workers:       workers,
		SubmitGapSec:  submitGapSec,
		PollAuth:      NormalizePollAuth(env("RH2API_POLL_AUTH", "apikey")),
		AuthKey:       authKey,
		AdminKey:      adminKey,
		SyncToken:     syncToken,
	}
}

// ExternalKeys returns the single API key plus any legacy env aliases.
func (c Config) ExternalKeys() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	for _, k := range []string{c.APIKey, c.AuthKey, c.AdminKey, c.SyncToken} {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
