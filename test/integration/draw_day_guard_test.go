//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// today is truncated to a UTC date the same way SalakService does, so a
// draw_dates row inserted for "today" always collides with whatever day the
// test actually runs on - this makes the test deterministic regardless of
// the real calendar date, rather than only passing on days that happen not
// to be the 16th/1st/2nd.
func today() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// TestBuySalakFlow_DrawDay_RejectsPurchase proves the guard rejects a
// purchase on a product's draw day, before any state changes, and that the
// rejection is recognisable as retryable via errors.Is(err, salak.ErrDrawDay)
// - not just "some validation error", which insufficient funds also is.
func TestBuySalakFlow_DrawDay_RejectsPurchase(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()

	user := mustCreateUser(t, tx, "")
	funding := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	salakAccount := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))
	mustCreateDrawDate(t, tx, product.ID, today())

	buySvc, accountRepository := newBuySalakService(tx)

	_, err := buySvc.BuySalak(ctx, user.ID, funding.ID, salakAccount.ID, product.ID, nil, decimal.RequireFromString("2000"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, salak.ErrDrawDay), "expected errors.Is(err, salak.ErrDrawDay), got: %v", err)

	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindValidation, appErr.Kind)

	gotFunding, findErr := accountRepository.FindByID(ctx, funding.ID)
	require.NoError(t, findErr)
	assert.True(t, decimal.RequireFromString("10000").Equal(gotFunding.Balance), "rejected before any debit could happen")
}

// TestBuySalakFlow_NonDrawDay_Succeeds proves the guard doesn't
// false-positive: a product with draw dates seeded on other days still
// allows a purchase today.
func TestBuySalakFlow_NonDrawDay_Succeeds(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()

	user := mustCreateUser(t, tx, "")
	funding := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	salakAccount := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))
	mustCreateDrawDate(t, tx, product.ID, today().AddDate(0, 0, -1))
	mustCreateDrawDate(t, tx, product.ID, today().AddDate(0, 0, 1))

	buySvc, _ := newBuySalakService(tx)

	_, err := buySvc.BuySalak(ctx, user.ID, funding.ID, salakAccount.ID, product.ID, nil, decimal.RequireFromString("2000"))
	require.NoError(t, err)
}
