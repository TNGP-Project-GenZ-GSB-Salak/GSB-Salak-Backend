package main

import (
	"context"
	"errors"
	"log"
	"time"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/config"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/db"
	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	salakrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/repository"
	userdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Fixed IDs keep local seeding idempotent: re-running always references the
// same demo user/accounts instead of generating duplicates or orphan rows.
var (
	demoUserID           = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	demoSavingsAccountID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	demoSalakAccountID   = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	demoKapookAccountID  = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	// demoFallbackGoalID backs the "already elapsed" Kapook goal seeded
	// below - a demo fallback in case the live create-goal-and-wait path
	// misbehaves during a presentation (see the ticket's own note on why:
	// a real 24h countdown can't be demonstrated live within the time
	// available, and a pre-elapsed one guarantees something to show).
	demoFallbackGoalID = uuid.MustParse("55555555-5555-5555-5555-555555555555")
)

// demoFallbackGoalAmount is both the fallback goal's target and the demo
// Kapook account's seeded balance - already fully saved, so the worker can
// buy it in full on its very first tick after seeding.
var demoFallbackGoalAmount = decimal.NewFromInt(5000)

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
	// One Kapook account per user, opened once and reused for every goal.
	// Seeded already funded to demoFallbackGoalAmount - see the fallback
	// goal below, which needs this balance to make sense as "already
	// saved in full."
	kapookAccount := accountdomain.Account{
		ID:            demoKapookAccountID,
		UserID:        demoUserID,
		AccountNumber: "5001000111",
		Type:          accountdomain.TypeKapook,
		Balance:       demoFallbackGoalAmount,
		Currency:      "THB",
	}

	kapookAccountIsFresh := false
	for _, acc := range []accountdomain.Account{savingsAccount, salakAccount, kapookAccount} {
		acc := acc
		result := gdb.WithContext(ctx).
			Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "account_number"}}, DoNothing: true}).
			Create(&acc)
		if result.Error != nil {
			log.Fatalf("failed to seed account %s: %v", acc.AccountNumber, result.Error)
		}
		if acc.ID == demoKapookAccountID {
			kapookAccountIsFresh = result.RowsAffected > 0
		}
		log.Printf("seeded demo account: %s (%s)", acc.AccountNumber, acc.Type)
	}

	// A demo fallback: a goal that already reached its target 48 hours ago
	// - comfortably overdue under any configured KAPOOK_COUNTDOWN_DURATION
	// - so the worker's very next tick buys it, without needing anyone to
	// create a goal and wait live during a presentation.
	//
	// Only attempted when the kapook account was freshly created just now:
	// the account's seeded balance (demoFallbackGoalAmount) is what makes
	// this goal's SavingAmount honest. DoNothing above means a pre-existing
	// account keeps whatever balance it already had - seeding a goal on top
	// of that would claim a SavingAmount the account can't actually back,
	// and the worker would fail to buy it on every tick.
	//
	// idx_kapook_goals_account_active only allows one ACTIVE goal per
	// account, and it's a partial index - a plain ON CONFLICT(id) doesn't
	// target it, so a manually-created active goal left over from testing
	// would make this INSERT fail outright. Checked for explicitly instead
	// of fighting the partial index's upsert syntax: skip seeding (and say
	// why) rather than erroring, since this row is a nice-to-have demo
	// affordance, not something the rest of seeding depends on.
	var existingActiveGoal kapookdomain.Goal
	err = gdb.WithContext(ctx).
		Where("account_id = ? AND is_active", demoKapookAccountID).
		First(&existingActiveGoal).Error
	switch {
	case !kapookAccountIsFresh:
		log.Println("demo kapook account already existed - skipping the fallback goal (its balance may no longer match a fresh seed)")
	case err == nil:
		log.Printf("demo kapook account already has an active goal (%s) - skipping the fallback goal", existingActiveGoal.ID)
	case errors.Is(err, gorm.ErrRecordNotFound):
		oneYearProduct, err := productRepo.FindByCode(ctx, "SALAK_1Y")
		if err != nil {
			log.Fatalf("failed to look up SALAK_1Y for the fallback goal: %v", err)
		}
		reachedAt := time.Now().UTC().Add(-48 * time.Hour)
		fallbackGoal := kapookdomain.Goal{
			ID:            demoFallbackGoalID,
			AccountID:     demoKapookAccountID,
			ProductID:     oneYearProduct.ID,
			GoalAmount:    demoFallbackGoalAmount,
			SavingAmount:  demoFallbackGoalAmount,
			SalakAmount:   decimal.Zero,
			IsActive:      true,
			GoalReachedAt: &reachedAt,
		}
		if err := gdb.WithContext(ctx).
			Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).
			Create(&fallbackGoal).Error; err != nil {
			log.Fatalf("failed to seed fallback kapook goal: %v", err)
		}
		log.Printf("seeded fallback kapook goal: reached %s ago, %s THB ready to auto-purchase", time.Since(reachedAt).Round(time.Hour), demoFallbackGoalAmount)
	default:
		log.Fatalf("failed to check for an existing active kapook goal: %v", err)
	}
}
