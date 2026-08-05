//go:build integration

package integration

import (
	"context"
	"testing"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	accountrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/account/repository"
	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	kapookrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/repository"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	salakrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/repository"
	txrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/repository"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKapookGoalFlow_BuyFromGoal_PartialPurchase_MintsHoldingAndLeavesGoalActive(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	salakAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	savingsAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))
	goal, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)
	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("3000"))
	require.NoError(t, err)

	result, err := kapookSvc.BuyFromGoal(ctx, user.ID, kapookAcc.ID, salakAcc.ID, decimal.RequireFromString("2000"))
	require.NoError(t, err)

	assert.False(t, result.GoalCompleted)
	assert.True(t, result.Goal.IsActive)
	assert.True(t, decimal.RequireFromString("2000").Equal(result.Goal.SalakAmount))
	assert.True(t, decimal.RequireFromString("3000").Equal(result.Goal.SavingAmount), "SavingAmount stays put - a purchase changes the form of the contribution, not its total")
	assert.EqualValues(t, 20, result.Receipt.Units) // 2000 / 100 unit price

	accountRepository := accountrepo.NewGormAccountRepository(tx)
	gotKapook, err := accountRepository.FindByID(ctx, kapookAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("1000").Equal(gotKapook.Balance), "3000 - 2000 spent")

	gotSalak, err := accountRepository.FindByID(ctx, salakAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("2000").Equal(gotSalak.Balance))

	holdings, err := salakrepo.NewGormHoldingRepository(tx).FindByAccountID(ctx, salakAcc.ID)
	require.NoError(t, err)
	require.Len(t, holdings, 1)

	// Exactly one debit+credit ledger pair - the Kapook side writes no
	// second pair of its own.
	ledgerRepository := txrepo.NewGormLedgerRepository(tx)
	kapookEntries, err := ledgerRepository.FindByAccountID(ctx, kapookAcc.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, kapookEntries, 2, "the earlier deposit's credit entry, plus this purchase's debit entry")
	salakEntries, err := ledgerRepository.FindByAccountID(ctx, salakAcc.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, salakEntries, 1)

	fetchedGoal, err := kapookSvc.GetActiveGoal(ctx, user.ID, kapookAcc.ID)
	require.NoError(t, err, "the goal must still be active and findable")
	assert.True(t, fetchedGoal.IsActive)

	transactionRepository := kapookrepo.NewGormTransactionRepository(tx)
	used, err := transactionRepository.CountByGoalAndTypesInWindow(ctx, tx, goal.ID, []kapookdomain.TransactionType{kapookdomain.TransactionBuySalak}, goal.CreatedAt, goal.CreatedAt.AddDate(1, 0, 0))
	require.NoError(t, err)
	assert.Equal(t, 1, used)

	// The units/count aggregation is a real cross-schema join
	// (kapook_transactions -> salak.holdings via holding_id) - worth
	// verifying against real Postgres, not just the fake repo in the unit
	// suite.
	units, count, err := transactionRepository.SumPurchasedUnitsAndCount(ctx, tx, goal.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 20, units)
	assert.Equal(t, 1, count)

	snap, err := kapookSvc.Snapshot(ctx, result.Goal)
	require.NoError(t, err)
	assert.EqualValues(t, 20, snap.PurchasedUnits)
	assert.Equal(t, 1, snap.PurchasedCount)
	assert.True(t, decimal.RequireFromString("1000").Equal(snap.AvailableBalance), "3000 saved - 2000 converted")
	assert.False(t, snap.TargetReached, "goal amount is 5000, only 3000 saved so far")
}

func TestKapookGoalFlow_BuyFromGoal_FullPurchase_DeactivatesGoal(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	salakAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	savingsAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))
	_, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("2000"))
	require.NoError(t, err)
	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("2000"))
	require.NoError(t, err)

	result, err := kapookSvc.BuyFromGoal(ctx, user.ID, kapookAcc.ID, salakAcc.ID, decimal.RequireFromString("2000"))
	require.NoError(t, err)

	assert.True(t, result.GoalCompleted)
	assert.False(t, result.Goal.IsActive)
	assert.True(t, decimal.RequireFromString("2000").Equal(result.Goal.SalakAmount))

	fetched, err := kapookSvc.GetActiveGoal(ctx, user.ID, kapookAcc.ID)
	require.NoError(t, err, "a fully-satisfied goal is no longer active, which is the null contract, not an error")
	assert.Nil(t, fetched)
}

// TestKapookGoalFlow_BuyFromGoal_DrawDay_RejectedWithoutChangingAnything
// proves the draw-day guard, shared via BuySalakForKapook with the public
// path, actually applies to a Kapook-funded purchase too.
func TestKapookGoalFlow_BuyFromGoal_DrawDay_RejectedWithoutChangingAnything(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	salakAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	savingsAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))
	mustCreateDrawDate(t, tx, product.ID, today())

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))
	_, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)
	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("2000"))
	require.NoError(t, err)

	_, err = kapookSvc.BuyFromGoal(ctx, user.ID, kapookAcc.ID, salakAcc.ID, decimal.RequireFromString("2000"))
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindValidation, appErr.Kind)

	accountRepository := accountrepo.NewGormAccountRepository(tx)
	gotKapook, err := accountRepository.FindByID(ctx, kapookAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("2000").Equal(gotKapook.Balance), "rejected purchase must not touch the kapook balance")

	holdings, err := salakrepo.NewGormHoldingRepository(tx).FindByAccountID(ctx, salakAcc.ID)
	require.NoError(t, err)
	assert.Empty(t, holdings)
}

func TestKapookGoalFlow_BuyFromGoal_AmountExceedingAvailableBalance_Rejected(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	salakAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	savingsAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))
	_, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)
	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("2000"))
	require.NoError(t, err)
	// Convert 1000 of it, leaving only 1000 available.
	_, err = kapookSvc.BuyFromGoal(ctx, user.ID, kapookAcc.ID, salakAcc.ID, decimal.RequireFromString("1000"))
	require.NoError(t, err)

	_, err = kapookSvc.BuyFromGoal(ctx, user.ID, kapookAcc.ID, salakAcc.ID, decimal.RequireFromString("2000"))
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindValidation, appErr.Kind)

	accountRepository := accountrepo.NewGormAccountRepository(tx)
	gotKapook, err := accountRepository.FindByID(ctx, kapookAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("1000").Equal(gotKapook.Balance), "rejected second purchase must not touch the balance again")
}
