//go:build integration

package integration

import (
	"context"
	"testing"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	kapookrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoalRepo_Create_SecondActiveGoalRejectedByPartialIndex proves the
// partial unique index (account_id) WHERE is_active is real, not just
// service-layer discipline - a second active goal on the same account is
// rejected even via a raw repo call that bypasses any pre-check.
func TestGoalRepo_Create_SecondActiveGoalRejectedByPartialIndex(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	account := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))
	repo := kapookrepo.NewGormGoalRepository(tx)

	first := &kapookdomain.Goal{ID: uuid.New(), AccountID: account.ID, ProductID: product.ID, GoalAmount: decimal.RequireFromString("10000"), IsActive: true}
	require.NoError(t, repo.Create(ctx, first))

	second := &kapookdomain.Goal{ID: uuid.New(), AccountID: account.ID, ProductID: product.ID, GoalAmount: decimal.RequireFromString("5000"), IsActive: true}
	err := repo.Create(ctx, second)
	requirePgErrorCode(t, err, sqlStateUniqueViolation)
}

// TestGoalRepo_Create_ActiveGoalAlongsideInactiveIsAllowed proves the index
// is genuinely partial: an account can accumulate many inactive goals over
// its life, so long as at most one is active at a time.
func TestGoalRepo_Create_ActiveGoalAlongsideInactiveIsAllowed(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	account := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))
	repo := kapookrepo.NewGormGoalRepository(tx)

	inactive := &kapookdomain.Goal{ID: uuid.New(), AccountID: account.ID, ProductID: product.ID, GoalAmount: decimal.RequireFromString("10000"), IsActive: false}
	require.NoError(t, repo.Create(ctx, inactive))

	active := &kapookdomain.Goal{ID: uuid.New(), AccountID: account.ID, ProductID: product.ID, GoalAmount: decimal.RequireFromString("5000"), IsActive: true}
	require.NoError(t, repo.Create(ctx, active))

	got, err := repo.FindActiveByAccountID(ctx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, active.ID, got.ID)
}
