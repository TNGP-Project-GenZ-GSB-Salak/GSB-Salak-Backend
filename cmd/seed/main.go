package main

import (
	"context"
	"log"
	"time"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/config"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/db"
	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	salakrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/repository"
	userdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/clause"
)

// Fixed IDs keep local seeding idempotent: re-running always references the
// same demo user/accounts instead of generating duplicates or orphan rows.
var (
	demoUserID           = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	demoSavingsAccountID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	demoSalakAccountID   = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	demoKapookAccountID  = uuid.MustParse("44444444-4444-4444-4444-444444444444")
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	gdb, err := db.Open(cfg.DB)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	productRepo := salakrepo.NewGormProductRepository(gdb)

	products := []salakdomain.Product{
		{
			ID:          uuid.New(),
			Code:        "SALAK_1Y",
			Name:        "Digital Salak 1-Year",
			TermMonths:  12,
			UnitPrice:   decimal.NewFromInt(100),
			MinPurchase: decimal.NewFromInt(1000),
			MaxPurchase: decimal.NewFromInt(10_000_000),
			StepAmount:  decimal.NewFromInt(1000),
			IsActive:    true,
		},
		{
			ID:          uuid.New(),
			Code:        "SALAK_2Y",
			Name:        "Digital Salak 2-Year",
			TermMonths:  24,
			UnitPrice:   decimal.NewFromInt(100),
			MinPurchase: decimal.NewFromInt(1000),
			MaxPurchase: decimal.NewFromInt(10_000_000),
			StepAmount:  decimal.NewFromInt(1000),
			IsActive:    true,
		},
	}

	for i := range products {
		if err := productRepo.Upsert(ctx, &products[i]); err != nil {
			log.Fatalf("failed to seed product %s: %v", products[i].Code, err)
		}
		log.Printf("seeded salak product: %s", products[i].Code)
	}

	// Draw dates are a real business fact, not demo data, so they're seeded
	// unconditionally (like the products themselves) rather than gated
	// behind SEED_DEMO_DATA.
	drawDateRepo := salakrepo.NewGormDrawDateRepository(gdb)
	currentYear := time.Now().UTC().Year()
	years := []int{currentYear, currentYear + 1}

	for i := range products {
		product, err := productRepo.FindByCode(ctx, products[i].Code)
		if err != nil {
			log.Fatalf("failed to look up seeded product %s: %v", products[i].Code, err)
		}

		drawDates := generateDrawDates(product.TermMonths, years)
		for _, d := range drawDates {
			if err := drawDateRepo.Create(ctx, &salakdomain.DrawDate{ID: uuid.New(), ProductID: product.ID, DrawDate: d}); err != nil {
				log.Fatalf("failed to seed draw date %s for %s: %v", d.Format("2006-01-02"), product.Code, err)
			}
		}
		log.Printf("seeded %d draw dates for %s (%d-%d)", len(drawDates), product.Code, years[0], years[len(years)-1])
	}

	if !cfg.SeedDemoData {
		log.Println("SEED_DEMO_DATA is false, skipping demo user/accounts")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("demopass123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash demo password: %v", err)
	}

	demoUser := userdomain.User{
		ID:           demoUserID,
		Username:     "demo",
		PasswordHash: string(hash),
		FullName:     "Somsri Somjai",
	}
	if err := gdb.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "username"}}, DoNothing: true}).
		Create(&demoUser).Error; err != nil {
		log.Fatalf("failed to seed demo user: %v", err)
	}
	log.Println("seeded demo user: demo / demopass123")

	savingsAccount := accountdomain.Account{
		ID:            demoSavingsAccountID,
		UserID:        demoUserID,
		AccountNumber: "1234009012",
		Type:          accountdomain.TypeSavings,
		Balance:       decimal.NewFromInt(50_000),
		Currency:      "THB",
	}
	salakAccount := accountdomain.Account{
		ID:            demoSalakAccountID,
		UserID:        demoUserID,
		AccountNumber: "4001000111",
		Type:          accountdomain.TypeSalak,
		Balance:       decimal.Zero,
		Currency:      "THB",
	}
	// One Kapook account per user, opened once and reused for every goal -
	// it persists permanently and sits at zero between goals.
	kapookAccount := accountdomain.Account{
		ID:            demoKapookAccountID,
		UserID:        demoUserID,
		AccountNumber: "5001000111",
		Type:          accountdomain.TypeKapook,
		Balance:       decimal.Zero,
		Currency:      "THB",
	}

	for _, acc := range []accountdomain.Account{savingsAccount, salakAccount, kapookAccount} {
		acc := acc
		if err := gdb.WithContext(ctx).
			Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "account_number"}}, DoNothing: true}).
			Create(&acc).Error; err != nil {
			log.Fatalf("failed to seed account %s: %v", acc.AccountNumber, err)
		}
		log.Printf("seeded demo account: %s (%s)", acc.AccountNumber, acc.Type)
	}
}
