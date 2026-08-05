//go:build integration

package integration

import (
	"context"
	"testing"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKapookGoalFlow_History_ScopedToOneGoal proves the ticket's central
// requirement against a real database, not a fake: a closed goal's history
// must never appear under a new goal on the same kapook account, even
// though both goals' rows live in the same kapook_transactions table.
func TestKapookGoalFlow_History_ScopedToOneGoal(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	savingsAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	mustSetPrimaryAccount(t, tx, savingsAcc.ID)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))

	// First goal: reach its (small, one-step) target, then withdraw the
	// full balance during the live countdown - the only way a goal closes
	// without a completing purchase, per Withdraw's own all-or-nothing rule
	// - so a second goal can be opened on the same account afterward.
	firstGoal, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("1000"))
	require.NoError(t, err)
	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("1000"))
	require.NoError(t, err)
	firstWithdraw, err := kapookSvc.Withdraw(ctx, user.ID, kapookAcc.ID, decimal.RequireFromString("1000"))
	require.NoError(t, err)
	require.True(t, firstWithdraw.GoalClosed, "the first goal must be closed before a second can be created on the same account")

	// Second goal on the SAME kapook account, with its own deposit.
	secondGoal, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)
	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("2000"))
	require.NoError(t, err)

	secondHistory, err := kapookSvc.GetGoalHistory(ctx, user.ID, secondGoal.ID, 20, 0)
	require.NoError(t, err)
	require.Len(t, secondHistory, 1, "the first goal's deposit+withdrawal must not leak into the second goal's history")
	assert.Equal(t, kapookdomain.TransactionDeposit, secondHistory[0].Transaction.Type)
	assert.True(t, decimal.RequireFromString("2000").Equal(secondHistory[0].Transaction.Amount))

	firstHistory, err := kapookSvc.GetGoalHistory(ctx, user.ID, firstGoal.ID, 20, 0)
	require.NoError(t, err)
	require.Len(t, firstHistory, 2, "the first goal keeps its own two rows, unaffected by the second goal existing")
}

// TestKapookGoalFlow_History_FeeDerivedServerSide proves the fee/net
// returned for a withdraw_with_fee row is computed fresh at read time
// (matching Withdraw's own rounding) rather than ever being a stored value.
func TestKapookGoalFlow_History_FeeDerivedServerSide(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	savingsAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	mustSetPrimaryAccount(t, tx, savingsAcc.ID)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))
	goal, err := kapookSvc.CreateGoal(ctx, user.ID, kapookAcc.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)
	_, err = kapookSvc.Deposit(ctx, user.ID, kapookAcc.ID, savingsAcc.ID, decimal.RequireFromString("3000"))
	require.NoError(t, err)

	// Consume the two free withdrawals so the third is fee-charged.
	_, err = kapookSvc.Withdraw(ctx, user.ID, kapookAcc.ID, decimal.RequireFromString("100"))
	require.NoError(t, err)
	_, err = kapookSvc.Withdraw(ctx, user.ID, kapookAcc.ID, decimal.RequireFromString("100"))
	require.NoError(t, err)
	third, err := kapookSvc.Withdraw(ctx, user.ID, kapookAcc.ID, decimal.RequireFromString("1000.01"))
	require.NoError(t, err)
	require.True(t, third.FeeCharged)

	history, err := kapookSvc.GetGoalHistory(ctx, user.ID, goal.ID, 20, 0)
	require.NoError(t, err)
	require.Len(t, history, 4)

	var feeRow *kapookdomain.Transaction
	var feeAmount, netAmount decimal.Decimal
	for _, e := range history {
		if e.Transaction.Type == kapookdomain.TransactionWithdrawWithFee {
			feeRow = &e.Transaction
			feeAmount, netAmount = e.Fee, e.Net
		}
	}
	require.NotNil(t, feeRow, "the third withdrawal must be recorded as withdraw_with_fee")
	assert.True(t, decimal.RequireFromString("20.00").Equal(feeAmount), "matches Withdraw's own rounded fee")
	assert.True(t, decimal.RequireFromString("980.01").Equal(netAmount))
	assert.Nil(t, feeRow.IsAutomaticPurchase, "unset until ticket 10's worker populates it")
}

// TestKapookGoalFlow_History_ForeignGoal_MaskedAsNotFound proves a goal ID
// that exists but belongs to a different customer is indistinguishable from
// one that doesn't exist at all - the same ownership convention every other
// goal method uses.
func TestKapookGoalFlow_History_ForeignGoal_MaskedAsNotFound(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()

	owner := mustCreateUser(t, tx, "")
	ownerKapookAcc := mustCreateAccount(t, tx, owner.ID, accountdomain.TypeKapook, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, owner.ID))
	goal, err := kapookSvc.CreateGoal(ctx, owner.ID, ownerKapookAcc.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)

	intruder := mustCreateUser(t, tx, "")

	_, err = kapookSvc.GetGoalHistory(ctx, intruder.ID, goal.ID, 20, 0)
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindNotFound, appErr.Kind)

	_, err = kapookSvc.GetGoalHistory(ctx, intruder.ID, uuid.New(), 20, 0)
	require.Error(t, err)
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindNotFound, appErr.Kind, "a genuinely missing goal fails the same way as a foreign one")
}
