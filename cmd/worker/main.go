package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	accountrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/account/repository"
	accountsvc "github.com/ciaabcdefg/gsb-salak-backend/internal/account/service"
	badgerepo "github.com/ciaabcdefg/gsb-salak-backend/internal/badge/repository"
	badgesvc "github.com/ciaabcdefg/gsb-salak-backend/internal/badge/service"
	kapookrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/repository"
	kapooksvc "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/service"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/worker"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/config"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/db"
	salakrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/repository"
	salaksvc "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/service"
	transactionrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/repository"
	transactionsvc "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/service"
)

// tickInterval is how often the worker polls for due goals - a fixed part
// of the design (see the worker package doc), not a config knob. Only the
// countdown's own length is configurable, via KAPOOK_COUNTDOWN_DURATION.
const tickInterval = time.Minute

func main() {
	cfg := config.Load()

	gdb, err := db.Open(cfg.DB)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	var clk clock.Clock = clock.Real{}
	if cfg.FixedClockRFC3339 != "" {
		fixed, err := time.Parse(time.RFC3339, cfg.FixedClockRFC3339)
		if err != nil {
			log.Fatalf("invalid FIXED_CLOCK_RFC3339: %v", err)
		}
		clk = clock.Fixed(fixed)
		log.Printf("business clock pinned to %s via FIXED_CLOCK_RFC3339 - do not set this outside local/test", fixed)
	}

	accountRepository := accountrepo.NewGormAccountRepository(gdb)
	productRepository := salakrepo.NewGormProductRepository(gdb)
	holdingRepository := salakrepo.NewGormHoldingRepository(gdb)
	drawDateRepository := salakrepo.NewGormDrawDateRepository(gdb)
	ledgerRepository := transactionrepo.NewGormLedgerRepository(gdb)
	badgeRepository := badgerepo.NewGormBadgeRepository(gdb)
	termsRepository := kapookrepo.NewGormTermsRepository(gdb)
	goalRepository := kapookrepo.NewGormGoalRepository(gdb)
	kapookTransactionRepository := kapookrepo.NewGormTransactionRepository(gdb)

	accountService := accountsvc.NewAccountService(accountRepository)
	salakService := salaksvc.NewSalakService(productRepository, holdingRepository, accountService, drawDateRepository, clk)
	badgeService := badgesvc.NewBadgeService(badgeRepository)
	buySalakService := transactionsvc.NewBuySalakService(gdb, accountService, salakService, ledgerRepository, badgeService, clk)
	kapookService := kapooksvc.NewKapookService(termsRepository, goalRepository, salakService, accountService, gdb, ledgerRepository, kapookTransactionRepository, clk, buySalakService)

	w := worker.New(gdb, goalRepository, accountService, kapookService, clk, cfg.KapookCountdownDuration)

	log.Printf("kapook worker starting: countdown=%s, tick=%s", cfg.KapookCountdownDuration, tickInterval)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	w.Run(ctx, tickInterval)

	log.Println("kapook worker stopped")
}
