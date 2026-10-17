package app

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"ricrac/github"
	"ricrac/mail"
)

type Config struct {
	Port        int
	DatabaseURL string
	// External base URL, e.g. https://ricrac.example.com. Used for sign-in, OAuth metadata, MCP
	// host checks and the links the API returns.
	PublicURL string
	// Signs session cookies. Changing it signs everyone out.
	SecretKey string
	// Sign-in with Google. Without them nobody can sign in, but tokens keep working.
	GoogleClientID     string
	GoogleClientSecret string
	MaxUploadSize      int64
	// Forces the Secure flag on cookies. When nil it is derived from X-Forwarded-Proto.
	SecureCookies *bool
	// Where emails go out; nil when email is off.
	Mail *mail.Config
	// The GitHub App; nil when the GitHub integration is off.
	GitHub *github.Config
}

func ConfigFromEnv() (Config, error) {
	publicURL := strings.TrimRight(env("PUBLIC_URL"), "/")
	if publicURL == "" {
		return Config{}, errors.New("PUBLIC_URL must be set, e.g. https://ricrac.example.com")
	}
	if !strings.HasPrefix(publicURL, "https://") && !strings.HasPrefix(publicURL, "http://") {
		return Config{}, errors.New("PUBLIC_URL must start with https:// or http://")
	}
	secret := env("SECRET_KEY")
	if secret == "" {
		return Config{}, errors.New("SECRET_KEY must be set to a long random string")
	}
	if len(secret) < 32 {
		return Config{}, errors.New("SECRET_KEY must be at least 32 characters")
	}
	port, err := number("PORT", 3000)
	if err != nil {
		return Config{}, err
	}
	uploadMB, err := number("MAX_UPLOAD_MB", 25)
	if err != nil {
		return Config{}, err
	}
	databaseURL := env("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL must be set")
	}
	config := Config{
		Port:               port,
		DatabaseURL:        databaseURL,
		PublicURL:          publicURL,
		SecretKey:          secret,
		GoogleClientID:     env("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: env("GOOGLE_CLIENT_SECRET"),
		MaxUploadSize:      int64(uploadMB) * 1024 * 1024,
	}
	// Email and GitHub are optional: set wrong, they stay off rather than keeping Ricrac down.
	if config.Mail, err = mail.ParseConfig(env("SMTP_URL"), env("EMAIL_FROM")); err != nil {
		slog.Warn("email is off: " + err.Error())
	}
	config.GitHub, err = github.ParseConfig(env("GITHUB_APP_ID"), env("GITHUB_APP_SLUG"), env("GITHUB_APP_PRIVATE_KEY"),
		env("GITHUB_WEBHOOK_SECRET"), env("GITHUB_CLIENT_ID"), env("GITHUB_CLIENT_SECRET"), env("GITHUB_API_URL"), env("GITHUB_WEB_URL"))
	if err != nil {
		slog.Warn("GitHub is off: " + err.Error())
	}
	switch env("SECURE_COOKIES") {
	case "true", "1":
		config.SecureCookies = new(true)
	case "false", "0":
		config.SecureCookies = new(false)
	}
	return config, nil
}

func (c Config) GoogleConfigured() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
}

// CookieName gives cookies on https the __Host- prefix, so no other host can set them.
func (c Config) CookieName(name string) string {
	if strings.HasPrefix(c.PublicURL, "https://") {
		return "__Host-" + name
	}
	return name
}

func (c Config) MCPURL() string {
	return c.PublicURL + "/mcp"
}

func env(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func number(name string, fallback int) (int, error) {
	value := env(name)
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.New(name + " must be a number")
	}
	return n, nil
}
