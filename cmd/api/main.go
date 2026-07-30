package main

import (
	"log"
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/config"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/db"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"

	"github.com/go-chi/chi/v5"

	accounthttp "github.com/ciaabcdefg/gsb-salak-backend/internal/account/http"
	accountrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/account/repository"
	accountsvc "github.com/ciaabcdefg/gsb-salak-backend/internal/account/service"

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

func main() {
	cfg := config.Load()

	gdb, err := db.Open(cfg.DB)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	signer := jwtutil.NewSigner(cfg.JWTSecret, cfg.JWTExpiryMins)

	// Repositories
	userRepository := userrepo.NewGormUserRepository(gdb)
	accountRepository := accountrepo.NewGormAccountRepository(gdb)
	productRepository := salakrepo.NewGormProductRepository(gdb)
	holdingRepository := salakrepo.NewGormHoldingRepository(gdb)
	ledgerRepository := transactionrepo.NewGormLedgerRepository(gdb)

	// Services (composition root wires concrete services behind each domain's interface)
	authService := usersvc.NewAuthService(userRepository, signer)
	accountService := accountsvc.NewAccountService(accountRepository)
	salakService := salaksvc.NewSalakService(productRepository, holdingRepository, accountService)
	buySalakService := transactionsvc.NewBuySalakService(gdb, accountService, salakService, ledgerRepository)

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
