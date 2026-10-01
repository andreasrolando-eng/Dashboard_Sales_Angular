package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	AppEnv  string
	AppPort string
	// StaticDir, when set, makes the API also serve the built Angular app
	// (single-container deploys such as Railway). Empty = API only.
	StaticDir   string
	FrontendURL string
	DatabaseURL string

	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string

	SessionSecret string

	// Local email+password sign-in: how long a login lasts, and whether the
	// session cookie is HTTPS-only (default: true in production).
	SessionTTLHours int
	// First admin for hosts without a shell (Railway): only applied when that
	// account has no password yet. Remove the password variable after login.
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
	CookieSecure           bool

	ESBAPIBaseURL       string
	ESBAPIKey           string
	HealthchecksPingURL string

	// Nightly sync scheduler (runs inside `serve`). Enabled by default only
	// in production so dev machines do not auto-sync on every restart.
	SyncSchedulerEnabled bool
	SyncHourWIB          int
	SyncCatchupDays      int
	SyncRefreshDays      int
	SyncAlertWebhookURL  string
}

// Load reads .env (if present) then returns Config populated from the
// environment. Missing required values are left empty; callers that need
// them (auth, db) validate at the point of use.
//
// Tries the repo root .env whether the process runs from there or from
// api/ directly (e.g. `go run ./cmd/server` from inside api/), so local dev
// doesn't silently fall back to empty defaults depending on cwd.
func Load() Config {
	for _, path := range []string{".env", "../.env"} {
		if err := godotenv.Load(path); err == nil {
			break
		}
	}

	appEnv := getEnv("APP_ENV", "development")

	return Config{
		AppEnv: appEnv,
		// PORT is what PaaS hosts (Railway, Render) inject; APP_PORT is ours.
		AppPort:     getEnv("PORT", getEnv("APP_PORT", "8080")),
		StaticDir:   getEnv("STATIC_DIR", ""),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:4200"),
		DatabaseURL: getEnv("DATABASE_URL", ""),

		OIDCIssuer:       getEnv("OIDC_ISSUER", ""),
		OIDCClientID:     getEnv("OIDC_CLIENT_ID", ""),
		OIDCClientSecret: getEnv("OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:  getEnv("OIDC_REDIRECT_URL", ""),

		SessionSecret: getEnv("SESSION_SECRET", ""),

		SessionTTLHours:        getEnvInt("SESSION_TTL_HOURS", 12),
		BootstrapAdminEmail:    getEnv("BOOTSTRAP_ADMIN_EMAIL", ""),
		BootstrapAdminPassword: getEnv("BOOTSTRAP_ADMIN_PASSWORD", ""),
		CookieSecure:           getEnvBool("COOKIE_SECURE", appEnv == "production"),

		ESBAPIBaseURL:       getEnv("ESB_API_BASE_URL", ""),
		ESBAPIKey:           getEnv("ESB_API_KEY", ""),
		HealthchecksPingURL: getEnv("HEALTHCHECKS_PING_URL", ""),

		SyncSchedulerEnabled: getEnvBool("SYNC_SCHEDULER_ENABLED", appEnv == "production"),
		SyncHourWIB:          getEnvInt("SYNC_HOUR_WIB", 6),
		SyncCatchupDays:      getEnvInt("SYNC_CATCHUP_DAYS", 7),
		SyncRefreshDays:      getEnvInt("SYNC_REFRESH_DAYS", 3),
		SyncAlertWebhookURL:  getEnv("SYNC_ALERT_WEBHOOK_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(key)); err == nil {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return fallback
}
