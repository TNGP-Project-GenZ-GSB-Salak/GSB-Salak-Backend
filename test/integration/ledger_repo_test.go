//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	salakrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/repository"
	txdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/domain"
	txrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLedgerEntry(accountID uuid.UUID, entryType txdomain.EntryType, amount decimal.Decimal) *txdomain.LedgerEntry {
	return &txdomain.LedgerEntry{
		ID:            uuid.New(),
		AccountID:     accountID,
		Type:          entryType,
		Amount:        amount,
		BalanceAfter:  decimal.RequireFromString("500"),
		ReferenceType: "buy_salak",
		ReferenceID:   uuid.New(),
		Description:   "Integration test entry",
	}
}

func TestLedgerRepo_Create_InvalidTypeRejectedByCheckConstraint(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")
	account := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.Zero)
	repo := txrepo.NewGormLedgerRepository(tx)

	e := newLedgerEntry(account.ID, txdomain.EntryType("transfer"), decimal.RequireFromString("100"))
	err := repo.Create(context.Background(), tx, e)
	requirePgErrorCode(t, err, sqlStateCheckViolation)
}

func TestLedgerRepo_Create_NonPositiveAmountRejectedByCheckConstraint(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")
	account := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.Zero)
	repo := txrepo.NewGormLedgerRepository(tx)

	e := newLedgerEntry(account.ID, txdomain.EntryDebit, decimal.Zero)
	err := repo.Create(context.Background(), tx, e)
	requirePgErrorCode(t, err, sqlStateCheckViolation)
}

func TestLedgerRepo_Create_UnknownAccountIDRejectedByForeignKey(t *testing.T) {
	tx := newTestTx(t)
	repo := txrepo.NewGormLedgerRepository(tx)

	e := newLedgerEntry(uuid.New(), txdomain.EntryDebit, decimal.RequireFromString("100"))
	err := repo.Create(context.Background(), tx, e)
	requirePgErrorCode(t, err, sqlStateForeignKeyViolation)
}

func TestLedgerRepo_FindByAccountID_RespectsLimitOffsetAndOrdering(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")
	account := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.Zero)
	repo := txrepo.NewGormLedgerRepository(tx)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	makeAt := func(when time.Time) *txdomain.LedgerEntry {
		e := newLedgerEntry(account.ID, txdomain.EntryDebit, decimal.RequireFromString("100"))
		e.CreatedAt = when
		return e
	}

	oldest := makeAt(base.Add(-2 * time.Hour))
	middle := makeAt(base.Add(-1 * time.Hour))
	newest := makeAt(base)
	for _, e := range []*txdomain.LedgerEntry{oldest, middle, newest} {
		require.NoError(t, repo.Create(ctx, tx, e))
	}

	// limit=2, offset=1 over 3 entries ordered created_at DESC (newest,
	// middle, oldest) must return exactly [middle, oldest].
	got, err := repo.FindByAccountID(ctx, account.ID, 2, 1)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, middle.ID, got[0].ID)
	assert.Equal(t, oldest.ID, got[1].ID)
}

func TestLedgerRepo_HoldingID_RoundTripsNilAndSet(t *testing.T) {
	tx := newTestTx(t)
	account, product := holdingFixture(t, tx)
	repo := txrepo.NewGormLedgerRepository(tx)
	ctx := context.Background()

	withoutHolding := newLedgerEntry(account.ID, txdomain.EntryCredit, decimal.RequireFromString("100"))
	require.NoError(t, repo.Create(ctx, tx, withoutHolding))

	holding := newHolding(account.ID, product.ID, 2, 1, 2)
	require.NoError(t, salakrepo.NewGormHoldingRepository(tx).Create(ctx, tx, holding))

	withHolding := newLedgerEntry(account.ID, txdomain.EntryCredit, decimal.RequireFromString("200"))
	withHolding.HoldingID = &holding.ID
	require.NoError(t, repo.Create(ctx, tx, withHolding))

	got, err := repo.FindByAccountID(ctx, account.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byID := map[uuid.UUID]txdomain.LedgerEntry{}
	for _, e := range got {
		byID[e.ID] = e
	}
	assert.Nil(t, byID[withoutHolding.ID].HoldingID)
	require.NotNil(t, byID[withHolding.ID].HoldingID)
	assert.Equal(t, holding.ID, *byID[withHolding.ID].HoldingID)
}
