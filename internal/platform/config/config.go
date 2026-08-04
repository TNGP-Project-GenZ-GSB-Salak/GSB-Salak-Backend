package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

func (c DBConfig) URL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

type Config struct {
	AppEnv        string
	HTTPPort      string
	DB            DBConfig
	MigrateDSN    string
	JWTSecret     string
	JWTExpiryMins int
	SeedDemoData  bool
	// KapookCountdownDuration is how long a goal's auto-purchase countdown
	// runs once its target is reached, defaulting to the real 24h. This is
	// a config knob, not a code constant, specifically so a demo/test run
	// can set it to something like 60s - a 24h countdown can't be shown in
	// a 20-30 minute presentation, and the worker's real code path runs
	// unchanged either way.
	KapookCountdownDuration time.Duration
}

func Load() Config {
	cfg := Config{
		AppEnv:   getEnv("APP_ENV", "local"),
		HTTPPort: getEnv("HTTP_PORT", "8080"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "example"),
			Name:     getEnv("DB_NAME", "gsb_salak"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWTSecret:               getEnv("JWT_SECRET", ""),
		JWTExpiryMins:           getEnvInt("JWT_EXPIRY_MINUTES", 60),
		SeedDemoData:            getEnvBool("SEED_DEMO_DATA", false),
		KapookCountdownDuration: getEnvDuration("KAPOOK_COUNTDOWN_DURATION", 24*time.Hour),
	}

	cfg.MigrateDSN = getEnv("MIGRATE_DATABASE_URL", cfg.DB.URL())

	if cfg.AppEnv != "local" && cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET must be set outside of local environment")
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "local-dev-insecure-secret"
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
