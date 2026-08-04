//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	accountrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/account/repository"
	accountservice "github.com/ciaabcdefg/gsb-salak-backend/internal/account/service"
	badgerepo "github.com/ciaabcdefg/gsb-salak-backend/internal/badge/repository"
	badgeservice "github.com/ciaabcdefg/gsb-salak-backend/internal/badge/service"
	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	kapookrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/repository"
	kapookservice "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/service"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/worker"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	salakrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/repository"
	salakservice "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/service"
	txrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/repository"
	txservice "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newWorkerOn wires a *worker.Worker the same way cmd/worker/main.go does,
// with every repo/service built directly on db - unlike newKapookService's
// tx parameter, db here is meant to be sharedDB itself (or a real,
// independently-committing connection), since the worker's RunOnce opens
// its own top-level transaction rather than joining one a caller supplies.
func newWorkerOn(db *gorm.DB, countdownDuration time.Duration) *worker.Worker {
	accountRepository := accountrepo.NewGormAccountRepository(db)
	accountSvc := accountservice.NewAccountService(accountRepository)
	salakSvc := salakservice.NewSalakService(
		salakrepo.NewGormProductRepository(db),
		salakrepo.NewGormHoldingRepository(db),
		accountSvc,
		salakrepo.NewGormDrawDateRepository(db),
		clock.Real{},
	)
	ledgerRepo := txrepo.NewGormLedgerRepository(db)
	badgeSvc := badgeservice.NewBadgeService(badgerepo.NewGormBadgeRepository(db))
	buySalakSvc := txservice.NewBuySalakService(db, accountSvc, salakSvc, ledgerRepo, badgeSvc, clock.Real{})
	termsRepo := kapookrepo.NewGormTermsRepository(db)
	goalRepo := kapookrepo.NewGormGoalRepository(db)
	transactionRepo := kapookrepo.NewGormTransactionRepository(db)
	kapookSvc := kapookservice.NewKapookService(termsRepo, goalRepo, salakSvc, accountSvc, db, ledgerRepo, transactionRepo, clock.Real{}, buySalakSvc)
	return worker.New(db, goalRepo, accountSvc, kapookSvc, clock.Real{}, countdownDuration)
}

func TestKapookWorker_RunOnce_HappyPath_BuysTheFullAvailableBalance(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.RequireFromString("5000"))
	salakAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	reachedAt := time.Now().UTC().Add(-48 * time.Hour)
	goal := &kapookdomain.Goal{
		ID: uuid.New(), AccountID: kapookAcc.ID, ProductID: product.ID,
		GoalAmount: decimal.RequireFromString("5000"), SavingAmount: decimal.RequireFromString("5000"),
		IsActive: true, GoalReachedAt: &reachedAt,
	}
	require.NoError(t, tx.Create(goal).Error)

	w := newWorkerOn(tx, 24*time.Hour)
	summary, err := w.RunOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Claimed)
	require.Len(t, summary.Results, 1)
	assert.Equal(t, worker.OutcomePurchased, summary.Results[0].Outcome)

	var gotGoal kapookdomain.Goal
	require.NoError(t, tx.Where("id = ?", goal.ID).First(&gotGoal).Error)
	assert.False(t, gotGoal.IsActive, "a fully-satisfying auto-purchase closes the goal")
	assert.True(t, decimal.RequireFromString("5000").Equal(gotGoal.SalakAmount))

	holdings, err := salakrepo.NewGormHoldingRepository(tx).FindByAccountID(ctx, salakAcc.ID)
	require.NoError(t, err)
	require.Len(t, holdings, 1)
	assert.True(t, decimal.RequireFromString("5000").Equal(holdings[0].PurchaseAmount))
}

func TestKapookWorker_RunOnce_NotYetDue_LeavesTheGoalAlone(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.RequireFromString("5000"))
	mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	reachedAt := time.Now().UTC().Add(-1 * time.Hour) // countdown is 24h - nowhere near due
	goal := &kapookdomain.Goal{
		ID: uuid.New(), AccountID: kapookAcc.ID, ProductID: product.ID,
		GoalAmount: decimal.RequireFromString("5000"), SavingAmount: decimal.RequireFromString("5000"),
		IsActive: true, GoalReachedAt: &reachedAt,
	}
	require.NoError(t, tx.Create(goal).Error)

	w := newWorkerOn(tx, 24*time.Hour)
	summary, err := w.RunOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.Claimed)

	var gotGoal kapookdomain.Goal
	require.NoError(t, tx.Where("id = ?", goal.ID).First(&gotGoal).Error)
	assert.True(t, gotGoal.IsActive)
}

// TestKapookWorker_RunOnce_ConcurrentPasses_MintExactlyOneHolding is this
// slice's required idempotency proof: two concurrent single-pass runs over
// the same due goal must mint exactly one holding, never two, and never
// zero. This is the entire reason ClaimDueGoals uses SELECT ... FOR UPDATE
// SKIP LOCKED rather than a plain SELECT - a fake's "lock" is just Go
// program order and cannot demonstrate this, so it needs real Postgres.
//
// This deliberately breaks the rollback-per-test pattern used elsewhere in
// this suite, for the same reason
// TestReserveTicketRange_ConcurrentCallersGetDisjointContiguousRanges does:
// each "concurrent worker pass" must be a genuinely separate connection
// against sharedDB, which a single *sql.Tx cannot provide. The fixture is
// committed directly and cleaned up with raw DELETEs in t.Cleanup.
func TestKapookWorker_RunOnce_ConcurrentPasses_MintExactlyOneHolding(t *testing.T) {
	if sharedDB == nil {
		t.Skip("integration DB unreachable; run `make docker-up migrate-up` first")
	}
	ctx := context.Background()

	setupTx := sharedDB.Begin()
	require.NoError(t, setupTx.Error)
	user := mustCreateUser(t, setupTx, "")
	kapookAcc := mustCreateAccount(t, setupTx, user.ID, accountdomain.TypeKapook, decimal.RequireFromString("5000"))
	salakAcc := mustCreateAccount(t, setupTx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	product := mustCreateProduct(t, setupTx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	reachedAt := time.Now().UTC().Add(-48 * time.Hour)
	goal := &kapookdomain.Goal{
		ID: uuid.New(), AccountID: kapookAcc.ID, ProductID: product.ID,
		GoalAmount: decimal.RequireFromString("5000"), SavingAmount: decimal.RequireFromString("5000"),
		IsActive: true, GoalReachedAt: &reachedAt,
	}
	require.NoError(t, setupTx.Create(goal).Error)
	require.NoError(t, setupTx.Commit().Error)
	t.Cleanup(func() {
		sharedDB.Exec(`DELETE FROM transaction.ledger_entries WHERE account_id IN (?, ?)`, kapookAcc.ID, salakAcc.ID)
		sharedDB.Exec(`DELETE FROM kapook.kapook_transactions WHERE kapook_account_id = ?`, kapookAcc.ID)
		sharedDB.Exec(`DELETE FROM salak.holdings WHERE account_id = ?`, salakAcc.ID)
		sharedDB.Exec(`DELETE FROM kapook.kapook_goals WHERE id = ?`, goal.ID)
		sharedDB.Exec(`DELETE FROM account.accounts WHERE id IN (?, ?)`, kapookAcc.ID, salakAcc.ID)
		sharedDB.Exec(`DELETE FROM salak.products WHERE id = ?`, product.ID)
		sharedDB.Exec(`DELETE FROM "user".users WHERE id = ?`, user.ID)
	})

	const n = 2
	var wg sync.WaitGroup
	var mu sync.Mutex
	var summaries []worker.Summary
	var errs []error

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// A separate *gorm.DB bound to the same underlying sharedDB
			// connection pool per goroutine - each RunOnce opens its own
			// real Begin()/Commit(), the point of this test.
			w := newWorkerOn(sharedDB, 24*time.Hour)
			summary, err := w.RunOnce(ctx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			summaries = append(summaries, summary)
		}()
	}
	wg.Wait()

	require.Empty(t, errs)

	holdings, err := salakrepo.NewGormHoldingRepository(sharedDB).FindByAccountID(ctx, salakAcc.ID)
	require.NoError(t, err)
	require.Len(t, holdings, 1, "exactly one holding must be minted, never zero or two")
	assert.True(t, decimal.RequireFromString("5000").Equal(holdings[0].PurchaseAmount))

	totalPurchased := 0
	totalClaimed := 0
	for _, s := range summaries {
		totalPurchased += s.Purchased()
		totalClaimed += s.Claimed
	}
	assert.Equal(t, 1, totalPurchased, "exactly one of the two concurrent passes must be the one that bought it")
	assert.Equal(t, 1, totalClaimed, "SKIP LOCKED means only one pass ever sees the row at all - the other claims zero, not one-then-fails")

	var gotGoal kapookdomain.Goal
	require.NoError(t, sharedDB.Where("id = ?", goal.ID).First(&gotGoal).Error)
	assert.False(t, gotGoal.IsActive)
	assert.True(t, decimal.RequireFromString("5000").Equal(gotGoal.SalakAmount))
}
