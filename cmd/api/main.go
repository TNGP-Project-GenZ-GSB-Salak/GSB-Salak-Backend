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

	adminhttp "github.com/ciaabcdefg/gsb-salak-backend/internal/admin/http"
	adminrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/admin/repository"
	adminsvc "github.com/ciaabcdefg/gsb-salak-backend/internal/admin/service"

	badgerepo "github.com/ciaabcdefg/gsb-salak-backend/internal/badge/repository"
	badgesvc "github.com/ciaabcdefg/gsb-salak-backend/internal/badge/service"

	kapookhttp "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/http"
	kapookrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/repository"
	kapooksvc "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/service"

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
	adminSigner := jwtutil.NewAdminSigner(cfg.AdminJWTSecret, cfg.JWTExpiryMins)

	var clk clock.Clock = clock.Real{}

	// Repositories
	userRepository := userrepo.NewGormUserRepository(gdb)
	adminRepository := adminrepo.NewGormAdminRepository(gdb)
	accountRepository := accountrepo.NewGormAccountRepository(gdb)
	productRepository := salakrepo.NewGormProductRepository(gdb)
	holdingRepository := salakrepo.NewGormHoldingRepository(gdb)
	drawDateRepository := salakrepo.NewGormDrawDateRepository(gdb)
	ledgerRepository := transactionrepo.NewGormLedgerRepository(gdb)
	badgeRepository := badgerepo.NewGormBadgeRepository(gdb)
	termsRepository := kapookrepo.NewGormTermsRepository(gdb)
	goalRepository := kapookrepo.NewGormGoalRepository(gdb)
	kapookTransactionRepository := kapookrepo.NewGormTransactionRepository(gdb)

	// Services (composition root wires concrete services behind each domain's interface)
	accountService := accountsvc.NewAccountService(accountRepository)
	authService := usersvc.NewAuthService(userRepository, signer, accountService, gdb, cfg.RegistrationSavingsStartingBalance)
	salakService := salaksvc.NewSalakService(productRepository, holdingRepository, accountService, drawDateRepository, clk)
	badgeService := badgesvc.NewBadgeService(badgeRepository)
	buySalakService := transactionsvc.NewBuySalakService(gdb, accountService, salakService, ledgerRepository, badgeService, clk)
	kapookService := kapooksvc.NewKapookService(termsRepository, goalRepository, salakService, accountService, gdb, ledgerRepository, kapookTransactionRepository, clk, buySalakService, cfg.KapookCountdownDuration)
	adminService := adminsvc.NewAdminService(adminRepository, adminSigner)

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
	kapookHandler := kapookhttp.NewHandler(kapookService)
	adminHandler := adminhttp.NewHandler(adminService, kapookService)

	router := httpserver.NewRouter()

	router.Route("/api/v1", func(r chi.Router) {
		userHandler.RegisterRoutes(r)
		adminHandler.RegisterPublicRoutes(r)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(signer))
			accountHandler.RegisterRoutes(r)
			salakHandler.RegisterRoutes(r)
			transactionHandler.RegisterRoutes(r)
			kapookHandler.RegisterRoutes(r)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.AdminAuth(adminSigner))
			adminHandler.RegisterAdminRoutes(r)
		})
	})

	log.Printf("starting server on :%s", cfg.HTTPPort)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, router); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
