package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/config"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/db"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"

	"github.com/go-chi/chi/v5"

	accounthttp "github.com/ciaabcdefg/gsb-salak-backend/internal/account/http"
	accountrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/account/repository"
	accountsvc "github.com/ciaabcdefg/gsb-salak-backend/internal/account/service"

	badgerepo "github.com/ciaabcdefg/gsb-salak-backend/internal/badge/repository"
	badgesvc "github.com/ciaabcdefg/gsb-salak-backend/internal/badge/service"

	salakhttp "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/http"
	salakrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/repository"
	salaksvc "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/service"

	transactionhttp "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/http"
	transactionrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/repository"
	transactionsvc "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/service"

	userhttp "github.com/ciaabcdefg/gsb-salak-backend/internal/user/http"
	userrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/user/repository"
	usersvc "github.com/ciaabcdefg/gsb-salak-backend/internal/user/service"
)

// @title           GSB Digital Salak API
// @version         1.0
// @description     Backend API mimicking the "Digital Salak" feature of GSB's MyMo mobile banking app.
// @host            localhost:8080
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Type "Bearer" followed by a space and the JWT token from POST /api/v1/auth/login.
func main() {
	cfg := config.Load()

	gdb, err := db.Open(cfg.DB)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	signer := jwtutil.NewSigner(cfg.JWTSecret, cfg.JWTExpiryMins)

	var clk clock.Clock = clock.Real{}
	if cfg.FixedClockRFC3339 != "" {
		fixed, err := time.Parse(time.RFC3339, cfg.FixedClockRFC3339)
		if err != nil {
			log.Fatalf("invalid FIXED_CLOCK_RFC3339: %v", err)
		}
		clk = clock.Fixed(fixed)
		log.Printf("business clock pinned to %s via FIXED_CLOCK_RFC3339 - do not set this outside local/test", fixed)
	}

	// Repositories
	userRepository := userrepo.NewGormUserRepository(gdb)
	accountRepository := accountrepo.NewGormAccountRepository(gdb)
	productRepository := salakrepo.NewGormProductRepository(gdb)
	holdingRepository := salakrepo.NewGormHoldingRepository(gdb)
	drawDateRepository := salakrepo.NewGormDrawDateRepository(gdb)
	ledgerRepository := transactionrepo.NewGormLedgerRepository(gdb)
	badgeRepository := badgerepo.NewGormBadgeRepository(gdb)

	// Services (composition root wires concrete services behind each domain's interface)
	authService := usersvc.NewAuthService(userRepository, signer)
	accountService := accountsvc.NewAccountService(accountRepository)
	salakService := salaksvc.NewSalakService(productRepository, holdingRepository, accountService, drawDateRepository, clk)
	badgeService := badgesvc.NewBadgeService(badgeRepository)
	buySalakService := transactionsvc.NewBuySalakService(gdb, accountService, salakService, ledgerRepository, badgeService, clk)

	if activeProducts, err := productRepository.ListActive(context.Background()); err != nil {
		log.Printf("WARNING: failed to check draw-date calendar coverage: %v", err)
	} else {
		for _, w := range drawDateCoverageWarnings(context.Background(), activeProducts, drawDateRepository, clk.Now(), 60*24*time.Hour) {
			log.Printf("WARNING: %s", w)
		}
	}

	// HTTP handlers
	userHandler := userhttp.NewHandler(authService)
	accountHandler := accounthttp.NewHandler(accountService)
	salakHandler := salakhttp.NewHandler(salakService)
	transactionHandler := transactionhttp.NewHandler(buySalakService)

	router := httpserver.NewRouter()

	router.Route("/api/v1", func(r chi.Router) {
		userHandler.RegisterRoutes(r)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(signer))
			accountHandler.RegisterRoutes(r)
			salakHandler.RegisterRoutes(r)
			transactionHandler.RegisterRoutes(r)
		})
	})

	log.Printf("starting server on :%s", cfg.HTTPPort)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, router); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
