//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	accountrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/account/repository"
	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	kapookrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/repository"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	txrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/repository"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKapookGoalFlow_Withdraw_HappyPath_FirstTwoFreeThirdCharged(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	savingsAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))
	_, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)
	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("3000"))
	require.NoError(t, err)

	accountRepository := accountrepo.NewGormAccountRepository(tx)

	first, err := kapookSvc.Withdraw(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("500"))
	require.NoError(t, err)
	assert.False(t, first.FeeCharged, "1st withdrawal is free")
	assert.True(t, decimal.RequireFromString("2500").Equal(first.Goal.SavingAmount))

	second, err := kapookSvc.Withdraw(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("500"))
	require.NoError(t, err)
	assert.False(t, second.FeeCharged, "2nd withdrawal is free")

	third, err := kapookSvc.Withdraw(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("500"))
	require.NoError(t, err)
	assert.True(t, third.FeeCharged, "3rd withdrawal in the same window carries the fee")
	assert.True(t, decimal.RequireFromString("10").Equal(third.FeeAmount), "2%% of 500")
	assert.True(t, decimal.RequireFromString("490").Equal(third.NetCredited))
	assert.True(t, decimal.RequireFromString("1500").Equal(third.Goal.SavingAmount), "the kapook side drops by the full pre-fee amount each time")

	gotSavings, err := accountRepository.FindByID(ctx, savingsAcc.ID)
	require.NoError(t, err)
	// 10000 - 3000 (deposit) + 500 + 500 + 490 (net of the 3rd withdrawal's fee)
	assert.True(t, decimal.RequireFromString("8490").Equal(gotSavings.Balance))

	gotKapook, err := accountRepository.FindByID(ctx, kapookAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("1500").Equal(gotKapook.Balance))

	transactionRepository := kapookrepo.NewGormTransactionRepository(tx)
	used, err := transactionRepository.CountByGoalAndTypesInWindow(ctx, tx, first.Goal.ID, []kapookdomain.TransactionType{kapookdomain.TransactionWithdraw, kapookdomain.TransactionWithdrawWithFee}, first.Goal.CreatedAt, first.Goal.CreatedAt.AddDate(1, 0, 0))
	require.NoError(t, err)
	assert.Equal(t, 3, used)
}

func TestKapookGoalFlow_Withdraw_FullWithdrawal_LeavesGoalActive(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	savingsAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))
	_, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)
	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("1000"))
	require.NoError(t, err)

	result, err := kapookSvc.Withdraw(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("1000"))
	require.NoError(t, err)
	assert.True(t, decimal.Zero.Equal(result.Goal.SavingAmount))
	assert.True(t, result.Goal.IsActive, "emptying the kapook must not close the goal")

	fetchedGoal, err := kapookSvc.GetActiveGoal(ctx, user.ID, kapookAcc.ID)
	require.NoError(t, err, "the goal must still be findable as the account's active goal")
	assert.True(t, fetchedGoal.IsActive)
}

func TestKapookGoalFlow_Withdraw_ExceedingBalance_RejectedWithoutChangingBalances(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	savingsAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))
	_, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)
	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("300"))
	require.NoError(t, err)

	_, err = kapookSvc.Withdraw(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("301"))
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindValidation, appErr.Kind)

	accountRepository := accountrepo.NewGormAccountRepository(tx)
	gotKapook, err := accountRepository.FindByID(ctx, kapookAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("300").Equal(gotKapook.Balance), "rejected withdrawal must not touch the kapook balance")

	ledgerRepository := txrepo.NewGormLedgerRepository(tx)
	entries, err := ledgerRepository.FindByAccountID(ctx, kapookAcc.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "only the earlier deposit's credit entry, nothing from the rejected withdrawal")
}

// TestKapookGoalFlow_Withdraw_ConcurrentWithdrawals_OnlyOneConsumesLastFreeSlot
// is this slice's answer to the ticket's required concurrency test: it
// proves the goal row lock actually serializes the free-withdrawal count
// check under real contention, a guarantee the sqlmock-based unit tests
// cannot verify (a fake's "lock" is just Go program order, unrelated to
// whether the real SELECT ... FOR UPDATE works).
//
// This deliberately breaks the rollback-per-test pattern used everywhere
// else in this suite - see holding_repo_test.go's
// TestReserveTicketRange_ConcurrentCallersGetDisjointContiguousRanges and
// account_repo_test.go's TestAccountRepo_Debit_NoLostUpdateAcrossSequentialTransactions
// for the same reasoning: a *gorm.DB bound to one *sql.Tx isn't safe for
// concurrent goroutines, so the fixture is committed directly against
// sharedDB and cleaned up with raw DELETEs in t.Cleanup.
func TestKapookGoalFlow_Withdraw_ConcurrentWithdrawals_OnlyOneConsumesLastFreeSlot(t *testing.T) {
	if sharedDB == nil {
		t.Skip("integration DB unreachable; run `make docker-up migrate-up` first")
	}
	ctx := context.Background()

	setupTx := sharedDB.Begin()
	require.NoError(t, setupTx.Error)
	user := mustCreateUser(t, setupTx, "")
	kapookAcc := mustCreateAccount(t, setupTx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	savingsAcc := mustCreateAccount(t, setupTx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	product := mustCreateProduct(t, setupTx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	setupKapookSvc, termsRepo := newKapookService(setupTx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))
	goal, err := setupKapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)
	_, err = setupKapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("3000"))
	require.NoError(t, err)
	// Consume the first of the two free withdrawals up front, so exactly one
	// free slot remains for the two concurrent withdrawals below to race over.
	_, err = setupKapookSvc.Withdraw(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("100"))
	require.NoError(t, err)
	require.NoError(t, setupTx.Commit().Error)
	t.Cleanup(func() {
		sharedDB.Exec(`DELETE FROM kapook.kapook_transactions WHERE kapook_account_id = ?`, kapookAcc.ID)
		sharedDB.Exec(`DELETE FROM transaction.ledger_entries WHERE account_id IN (?, ?)`, kapookAcc.ID, savingsAcc.ID)
		sharedDB.Exec(`DELETE FROM kapook.kapook_goals WHERE account_id = ?`, kapookAcc.ID)
		sharedDB.Exec(`DELETE FROM kapook.terms_acceptances WHERE user_id = ?`, user.ID)
		sharedDB.Exec(`DELETE FROM account.accounts WHERE id IN (?, ?)`, kapookAcc.ID, savingsAcc.ID)
		sharedDB.Exec(`DELETE FROM salak.products WHERE id = ?`, product.ID)
		sharedDB.Exec(`DELETE FROM "user".users WHERE id = ?`, user.ID)
	})

	const n = 2
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx := sharedDB.Begin() // separate connection per goroutine - the point
			kapookSvc, _ := newKapookService(tx)
			_, err := kapookSvc.Withdraw(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("50"))
			if err != nil {
				tx.Rollback()
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			if commitErr := tx.Commit().Error; commitErr != nil {
				mu.Lock()
				errs = append(errs, commitErr)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	require.Empty(t, errs)

	transactionRepository := kapookrepo.NewGormTransactionRepository(sharedDB)
	windowStart, windowEnd := goal.CreatedAt, goal.CreatedAt.AddDate(1, 0, 0)
	freeCount, err := transactionRepository.CountByGoalAndTypesInWindow(ctx, nil, goal.ID, []kapookdomain.TransactionType{kapookdomain.TransactionWithdraw}, windowStart, windowEnd)
	require.NoError(t, err)
	feeCount, err := transactionRepository.CountByGoalAndTypesInWindow(ctx, nil, goal.ID, []kapookdomain.TransactionType{kapookdomain.TransactionWithdrawWithFee}, windowStart, windowEnd)
	require.NoError(t, err)

	// One free withdrawal was already consumed in setup, so the two
	// concurrent calls have exactly one free slot to race over: without the
	// row lock serializing the count check, both could read "1 used" and
	// both would go free, producing free=3/fee=0 instead of free=2/fee=1.
	assert.Equal(t, 2, freeCount, "the setup withdrawal plus exactly one of the two concurrent ones")
	assert.Equal(t, 1, feeCount, "exactly one concurrent withdrawal must be charged the fee, never zero or two")
}
