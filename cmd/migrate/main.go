package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/config"
	"github.com/ciaabcdefg/gsb-salak-backend/migrations"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func main() {
	cfg := config.Load()

	if len(os.Args) < 2 {
		log.Fatal("usage: migrate <up|down|version|force <n>>")
	}
	cmd := os.Args[1]

	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		log.Fatalf("failed to load embedded migrations: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, cfg.MigrateDSN)
	if err != nil {
		log.Fatalf("failed to init migrate: %v", err)
	}

	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate up failed: %v", err)
		}
		log.Println("migrations applied")
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate down failed: %v", err)
		}
		log.Println("migrations rolled back")
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("failed to get version: %v", err)
		}
		log.Printf("version=%d dirty=%v", v, dirty)
	case "force":
		if len(os.Args) < 3 {
			log.Fatal("usage: migrate force <version>")
		}
		var version int
		if _, err := fmt.Sscanf(os.Args[2], "%d", &version); err != nil {
			log.Fatalf("invalid version: %v", err)
		}
		if err := m.Force(version); err != nil {
			log.Fatalf("migrate force failed: %v", err)
		}
	default:
		log.Fatalf("unknown command: %s", cmd)
	}
}
