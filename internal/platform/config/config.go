package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
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
	// AdminJWTSecret signs admin tokens - deliberately its own env var, not
	// JWTSecret, so the two are never accidentally the same value (see
	// internal/platform/jwtutil/admin.go's AdminSigner doc comment for why
	// that separation matters).
	AdminJWTSecret string
	// AdminUsername/AdminPassword are used only by cmd/seed to create or
	// rotate the seeded admin credential. Empty outside local means
	// cmd/seed skips admin creation with a warning, rather than falling
	// back to a guessable default - an admin credential is more sensitive
	// than the JWT secret.
	AdminUsername string
	AdminPassword string
	// KapookCountdownDuration is how long a goal's auto-purchase countdown
	// runs once its target is reached, defaulting to the real 24h. This is
	// a config knob, not a code constant, specifically so a demo/test run
	// can set it to something like 60s - a 24h countdown can't be shown in
	// a 20-30 minute presentation, and the worker's real code path runs
	// unchanged either way.
	KapookCountdownDuration time.Duration
	// KapookWorkerTickInterval is how often cmd/worker polls for due goals -
	// a config knob for the same reason KapookCountdownDuration is one: a
	// demo/test run with a short countdown wants a short tick too, or the
	// spinner can sit "stuck-looking" for up to a full tick after the
	// countdown legitimately expires. Defaults to the real 1-minute cadence.
	KapookWorkerTickInterval time.Duration
	// RegistrationSavingsStartingBalance funds a new user's savings account
	// at registration - in the spirit of SEED_DEMO_DATA/
	// KAPOOK_COUNTDOWN_DURATION, a demo/test knob tuned without a code
	// change. The committed default is ฿1,000,000, so every new customer
	// registers with money already in their savings account - cmd/seed's
	// demo user (฿50,000) is unaffected, since this governs registration
	// only.
	RegistrationSavingsStartingBalance decimal.Decimal
}

// loadDotenv loads .env from the current working directory into the process
// environment, for whichever key it doesn't already have a real value for -
// godotenv.Load never overrides an already-set OS env var, so a real
// deployment's actual environment always wins over a stray .env file. A
// missing file is expected and silent (every cmd/* works with zero env vars
// set, per this package's own local-defaults philosophy); a malformed one
// is logged but not fatal, since the OS environment and this file's
// defaults can still carry the process.
func loadDotenv() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("WARNING: failed to load .env: %v", err)
	}
}

func Load() Config {
	loadDotenv()

	startingBalance, _ := decimal.NewFromString("1000000.00")

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
		JWTSecret:                          getEnv("JWT_SECRET", ""),
		JWTExpiryMins:                      getEnvInt("JWT_EXPIRY_MINUTES", 60),
		SeedDemoData:                       getEnvBool("SEED_DEMO_DATA", false),
		AdminJWTSecret:                     getEnv("ADMIN_JWT_SECRET", ""),
		AdminUsername:                      getEnv("ADMIN_USERNAME", ""),
		AdminPassword:                      getEnv("ADMIN_PASSWORD", ""),
		KapookCountdownDuration:            getEnvDuration("KAPOOK_COUNTDOWN_DURATION", 24*time.Hour),
		KapookWorkerTickInterval:           getEnvDuration("KAPOOK_WORKER_TICK_INTERVAL", time.Minute),
		RegistrationSavingsStartingBalance: getEnvDecimal("REGISTRATION_SAVINGS_STARTING_BALANCE", startingBalance),
	}

	cfg.MigrateDSN = getEnv("MIGRATE_DATABASE_URL", cfg.DB.URL())

	if cfg.AppEnv != "local" && cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET must be set outside of local environment")
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "local-dev-insecure-secret"
	}

	if cfg.AppEnv != "local" && cfg.AdminJWTSecret == "" {
		log.Fatal("ADMIN_JWT_SECRET must be set outside of local environment")
	}
	if cfg.AdminJWTSecret == "" {
		cfg.AdminJWTSecret = "local-dev-insecure-admin-secret"
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

func getEnvDecimal(key string, fallback decimal.Decimal) decimal.Decimal {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := decimal.NewFromString(v); err == nil {
			return d
		}
	}
	return fallback
}
