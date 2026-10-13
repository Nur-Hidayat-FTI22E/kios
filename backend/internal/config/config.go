package config

import (
	"os"
	"time"
)

type Config struct {
	Port              string
	DBDSN             string
	JWTSecret         string
	JWTTTL            time.Duration
	APPTimezone       string
	AllowedOrigin     string
	SeedOwnerUsername string
	SeedOwnerPassword string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() *Config {
	return &Config{
		Port:      env("PORT", "8080"),
		DBDSN:     env("DB_DSN", ""),
		JWTSecret: env("JWT_SECRET", ""),
		JWTTTL:    12 * time.Hour,
		APPTimezone: env("APP_TZ", "Asia/Makassar"),
		AllowedOrigin:     env("ALLOWED_ORIGIN", "http://localhost:5173"),
		SeedOwnerUsername: env("SEED_OWNER_USERNAME", "pemilik"),
		SeedOwnerPassword: env("SEED_OWNER_PASSWORD", "admin123"),
	}
}
