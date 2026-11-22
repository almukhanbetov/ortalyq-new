// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env            string // "local" or "production"
	HTTPAddr       string
	DatabaseURL    string
	AllowedOrigins []string
	// TrustedProxies: CIDRs/IPs whose X-Forwarded-For is believed (the reverse proxy). Empty keeps Gin's default.
	TrustedProxies   []string
	CookieName       string
	CookieSecure     bool
	CookieDomain     string
	SessionTTL       time.Duration
	UploadDir        string
	UploadMaxBytes   int64
	GiphyAPIKey      string
	GiphyAPIBase     string
	AutoMigrate      bool
	LoginMaxPerEmail int
	LoginMaxPerIP    int
	LoginWindow      time.Duration
	// Mail (stage 9): MAIL_DRIVER log (default, nothing is sent) or smtp; APP_URL builds links in letters.
	MailDriver   string
	MailSMTPAddr string
	MailFrom     string
	MailUser     string
	MailPassword string
	AppURL       string
	// Teacher sign-up limits; zero values use the defaults of the signup module (24 h link, 60 s pause, 5 letters per
	// address and 20 per IP an hour). Not read from the environment: tests override them.
	SignupLinkTTL     time.Duration
	SignupResendAfter time.Duration
	SignupMaxPerEmail int
	SignupMaxPerIP    int
	// Password reset limits; zero values use the defaults of the passwordreset module (1 h link, 60 s pause, 5 letters
	// per address and 20 per IP an hour). Not read from the environment: tests override them.
	ResetLinkTTL     time.Duration
	ResetResendAfter time.Duration
	ResetMaxPerEmail int
	ResetMaxPerIP    int
}

func (c Config) IsLocal() bool { return c.Env == "local" }

func Load() (Config, error) {
	c := Config{
		Env:              get("APP_ENV", "production"),
		HTTPAddr:         get("HTTP_ADDR", ":8448"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		AllowedOrigins:   splitList(get("CORS_ORIGINS", "http://localhost:3448")),
		TrustedProxies:   splitList(os.Getenv("TRUSTED_PROXIES")),
		CookieName:       get("SESSION_COOKIE_NAME", "ortalyq_session"),
		CookieSecure:     getBool("COOKIE_SECURE", true),
		CookieDomain:     os.Getenv("COOKIE_DOMAIN"),
		SessionTTL:       time.Duration(getInt("SESSION_TTL_HOURS", 24*14)) * time.Hour,
		UploadDir:        get("UPLOAD_DIR", "./data/uploads"),
		UploadMaxBytes:   int64(getInt("UPLOAD_MAX_MB", 10)) * 1024 * 1024,
		GiphyAPIKey:      os.Getenv("GIPHY_API_KEY"),
		GiphyAPIBase:     get("GIPHY_API_BASE", "https://api.giphy.com"),
		AutoMigrate:      getBool("AUTO_MIGRATE", false),
		LoginMaxPerEmail: getInt("LOGIN_MAX_PER_EMAIL", 5),
		LoginMaxPerIP:    getInt("LOGIN_MAX_PER_IP", 30),
		LoginWindow:      time.Duration(getInt("LOGIN_WINDOW_MINUTES", 15)) * time.Minute,
		MailDriver:       get("MAIL_DRIVER", "log"),
		MailSMTPAddr:     get("MAIL_SMTP_ADDR", ""),
		MailFrom:         get("MAIL_FROM", ""),
		MailUser:         get("MAIL_SMTP_USER", ""),
		MailPassword:     get("MAIL_SMTP_PASSWORD", ""),
		AppURL:           get("APP_URL", "http://localhost:3448"),
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if c.Env != "local" && c.Env != "production" {
		return c, fmt.Errorf("APP_ENV must be local or production")
	}
	if c.Env == "production" && !c.CookieSecure {
		return c, fmt.Errorf("COOKIE_SECURE must be true in production")
	}
	return c, nil
}

func get(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func getInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func getBool(k string, def bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
