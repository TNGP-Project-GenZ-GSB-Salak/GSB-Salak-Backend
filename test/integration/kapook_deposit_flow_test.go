//go:build integration

package integration

import (
	"context"
	"testing"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	accountrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/account/repository"
	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	txrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKapookTransactionRepo_Create_InvalidTypeRejectedByCheckConstraint
// proves the five-value type CHECK is real, not just app-level discipline.
func TestKapookTransactionRepo_Create_InvalidTypeRejectedByCheckConstraint(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)

	txn := &kapookdomain.Transaction{
		ID:              uuid.New(),
		Type:            kapookdomain.TransactionType("interest"), // not one of the five agreed values
		Amount:          decimal.RequireFromString("100"),
		KapookAccountID: kapookAcc.ID,
	}
	err := tx.Create(txn).Error
	requirePgErrorCode(t, err, sqlStateCheckViolation)
}

func TestKapookGoalFlow_Deposit_HappyPath_UpdatesBalancesGoalAndLedger(t *testing.T) {
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

	got, err := kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("1500"))
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("1500").Equal(got.SavingAmount))

	accountRepository := accountrepo.NewGormAccountRepository(tx)
	gotSavings, err := accountRepository.FindByID(ctx, savingsAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("8500").Equal(gotSavings.Balance))

	gotKapook, err := accountRepository.FindByID(ctx, kapookAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("1500").Equal(gotKapook.Balance))

	fetchedGoal, err := kapookSvc.GetActiveGoal(ctx, user.ID, kapookAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("1500").Equal(fetchedGoal.SavingAmount))

	// Both ledger entries share one reference_id, with the reference type
	// naming a kapook transaction.
	ledgerRepository := txrepo.NewGormLedgerRepository(tx)
	savingsEntries, err := ledgerRepository.FindByAccountID(ctx, savingsAcc.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, savingsEntries, 1)
	kapookEntries, err := ledgerRepository.FindByAccountID(ctx, kapookAcc.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, kapookEntries, 1)

	debitEntry, creditEntry := savingsEntries[0], kapookEntries[0]
	assert.Equal(t, "debit", string(debitEntry.Type))
	assert.Equal(t, "credit", string(creditEntry.Type))
	assert.Equal(t, "kapook_transaction", debitEntry.ReferenceType)
	assert.Equal(t, "kapook_transaction", creditEntry.ReferenceType)
	assert.Equal(t, debitEntry.ReferenceID, creditEntry.ReferenceID, "debit and credit must share one reference_id")
}

func TestKapookGoalFlow_Deposit_ExceedingTarget_Rejected(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	savingsAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))
	_, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("1000"))
	require.NoError(t, err)

	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("1500"))
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindValidation, appErr.Kind)

	accountRepository := accountrepo.NewGormAccountRepository(tx)
	gotSavings, err := accountRepository.FindByID(ctx, savingsAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("10000").Equal(gotSavings.Balance), "rejected deposit must not touch the funding account's balance")
}

// TestKapookGoalFlow_Deposit_InsufficientFunds_RollsBackEverything proves
// the whole deposit - goal lock, debit, credit, kapook_transaction, ledger
// pair - is one atomic unit: a debit failure partway through must leave no
// partial trace anywhere.
func TestKapookGoalFlow_Deposit_InsufficientFunds_RollsBackEverything(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	savingsAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("100"))
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))
	_, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)

	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("500"))
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindValidation, appErr.Kind)

	accountRepository := accountrepo.NewGormAccountRepository(tx)
	gotSavings, err := accountRepository.FindByID(ctx, savingsAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("100").Equal(gotSavings.Balance))

	gotKapook, err := accountRepository.FindByID(ctx, kapookAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.Zero.Equal(gotKapook.Balance))

	fetchedGoal, err := kapookSvc.GetActiveGoal(ctx, user.ID, kapookAcc.ID)
	require.NoError(t, err)
	assert.True(t, decimal.Zero.Equal(fetchedGoal.SavingAmount), "goal's saved amount must be untouched")

	ledgerRepository := txrepo.NewGormLedgerRepository(tx)
	entries, err := ledgerRepository.FindByAccountID(ctx, savingsAcc.ID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, entries, "no ledger entry should have been persisted")
}
