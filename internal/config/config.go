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

	// Legacy aliases kept so old .env / gpt-plus tokens still work until rotated.
	AuthKey   string
	AdminKey  string
	SyncToken string
}

func Load() Config {
	workers, _ := strconv.Atoi(env("RH2API_WORKERS", "2"))
	if workers < 1 {
		workers = 1
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
