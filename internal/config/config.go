package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	ContentDir  string
	JWTSecret   string
	AdminEmail  string
	AdminPasswd string

	// Theme
	ThemeDir      string
	ThemeBuildCmd string
	ThemeService  string // optional: systemd service to restart after build

	// Webhook
	WebhookSecret string // optional: if set, enables POST /api/webhook/rebuild

	// Media
	SiteURL      string // base URL for absolute media file links, e.g. https://example.com
	MediaStorage string // "local" (default) or "s3"
	MediaDir     string // derived: parent of ContentDir + "/media"

	// S3 (used when MediaStorage = "s3")
	S3Endpoint  string // S3-compatible API endpoint, e.g. https://s3.nevaobjects.id
	S3Bucket    string // bucket name
	S3Region    string // region (use "auto" for providers that don't require one)
	S3AccessKey string // access key ID
	S3SecretKey string // secret access key
	S3PublicURL string // base URL for public file access, e.g. https://s3.nevaobjects.id/my-bucket

	// Demo mode
	DemoMode   bool   // if true, seed demo content on startup and enable reset endpoint
	DemoEmail  string // demo login email (default: demo@foliocms.com)
	DemoPasswd string // demo login password (default: demo1234)

	// SMTP (newsletter)
	SMTPHost     string // e.g. smtp.mailgun.org
	SMTPPort     string // e.g. 587
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string // e.g. newsletter@example.com
}

func Load(envFile string) (*Config, error) {
	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			return nil, fmt.Errorf("loading env file: %w", err)
		}
	}

	port := getEnv("PORT", "8090")
	if _, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("PORT must be a number, got %q", port)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters (got %d)", len(jwtSecret))
	}

	contentDir := getEnv("CONTENT_DIR", "content/blog")
	mediaDir := filepath.Join(filepath.Dir(contentDir), "media")

	return &Config{
		Port:          port,
		DatabaseURL:   getEnv("DATABASE_URL", "data/folio.db"),
		ContentDir:    contentDir,
		JWTSecret:     jwtSecret,
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminPasswd:   os.Getenv("ADMIN_PASSWORD"),
		ThemeDir:      getEnv("THEME_DIR", "theme"),
		ThemeBuildCmd: getEnv("THEME_BUILD_CMD", "npm run build"),
		ThemeService:  os.Getenv("THEME_SERVICE"),
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),
		SiteURL:       getEnv("SITE_URL", "http://localhost:8090"),
		MediaStorage:  getEnv("MEDIA_STORAGE", "local"),
		MediaDir:      mediaDir,
		S3Endpoint:    os.Getenv("S3_ENDPOINT"),
		S3Bucket:      os.Getenv("S3_BUCKET"),
		S3Region:      getEnv("S3_REGION", "auto"),
		S3AccessKey:   os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:   os.Getenv("S3_SECRET_KEY"),
		S3PublicURL:   os.Getenv("S3_PUBLIC_URL"),
		DemoMode:      os.Getenv("DEMO_MODE") == "true",
		DemoEmail:     getEnv("DEMO_EMAIL", "demo@foliocms.com"),
		DemoPasswd:    getEnv("DEMO_PASSWD", "demo1234"),
		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPPort:      getEnv("SMTP_PORT", "587"),
		SMTPUsername:  os.Getenv("SMTP_USERNAME"),
		SMTPPassword:  os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:      os.Getenv("SMTP_FROM"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
