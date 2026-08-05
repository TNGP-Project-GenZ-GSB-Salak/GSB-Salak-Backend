//go:build integration

package integration

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	salakrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// holdingFixture creates a user + salak account + active product inside tx,
// returning the account/product so callers only need to fill in
// holding-specific fields.
func holdingFixture(t *testing.T, tx *gorm.DB) (accountdomain.Account, salakdomain.Product) {
	t.Helper()
	user := mustCreateUser(t, tx, "")
	account := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))
	return account, product
}

func newHolding(accountID, productID uuid.UUID, units, ticketStart, ticketEnd int64) *salakdomain.Holding {
	now := time.Now().UTC()
	return &salakdomain.Holding{
		ID:             uuid.New(),
		AccountID:      accountID,
		ProductID:      productID,
		Units:          units,
		TicketLetter:   "ก",
		TicketStart:    ticketStart,
		TicketEnd:      ticketEnd,
		PurchaseAmount: decimal.RequireFromString("1000"),
		PurchaseDate:   now,
		MaturityDate:   now.AddDate(1, 0, 0),
	}
}

func TestHoldingRepo_Create_TicketEndNotGreaterThanStartRejected(t *testing.T) {
	tx := newTestTx(t)
	account, product := holdingFixture(t, tx)
	repo := salakrepo.NewGormHoldingRepository(tx)

	h := newHolding(account.ID, product.ID, 1, 10, 5) // end <= start
	err := repo.Create(context.Background(), tx, h)
	requirePgErrorCode(t, err, sqlStateCheckViolation)
}

func TestHoldingRepo_Create_UnitsNotPositiveRejected(t *testing.T) {
	tx := newTestTx(t)
	account, product := holdingFixture(t, tx)
	repo := salakrepo.NewGormHoldingRepository(tx)

	h := newHolding(account.ID, product.ID, 0, 1, 5)
	err := repo.Create(context.Background(), tx, h)
	requirePgErrorCode(t, err, sqlStateCheckViolation)
}

func TestHoldingRepo_Create_TicketRangeWidthMustMatchUnits(t *testing.T) {
	tx := newTestTx(t)
	account, product := holdingFixture(t, tx)
	repo := salakrepo.NewGormHoldingRepository(tx)

	// end > start (satisfies that check on its own) but the range spans 10
	// numbers while units claims 3 - violates the table-level
	// "ticket_end - ticket_start + 1 = units" check.
	h := newHolding(account.ID, product.ID, 3, 1, 10)
	err := repo.Create(context.Background(), tx, h)
	requirePgErrorCode(t, err, sqlStateCheckViolation)
}

// TestHoldingRepo_Create_OverlappingRangeRejectedByExclusionConstraint
// exercises holdings_no_overlapping_ranges directly - belt-and-braces on
// top of ReserveTicketRange's row lock, since ticket ranges are prize-
// determining (see migration 000025's comment).
func TestHoldingRepo_Create_OverlappingRangeRejectedByExclusionConstraint(t *testing.T) {
	tx := newTestTx(t)
	account, product := holdingFixture(t, tx)
	repo := salakrepo.NewGormHoldingRepository(tx)
	ctx := context.Background()

	first := newHolding(account.ID, product.ID, 10, 1, 10)
	require.NoError(t, repo.Create(ctx, tx, first))

	overlapping := newHolding(account.ID, product.ID, 10, 5, 14) // 5-14 overlaps 1-10
	err := repo.Create(ctx, tx, overlapping)
	requirePgErrorCode(t, err, sqlStateExclusionViolation)
}

func TestHoldingRepo_Create_UnknownAccountIDRejectedByForeignKey(t *testing.T) {
	tx := newTestTx(t)
	_, product := holdingFixture(t, tx)
	repo := salakrepo.NewGormHoldingRepository(tx)

	h := newHolding(uuid.New(), product.ID, 2, 1, 2)
	err := repo.Create(context.Background(), tx, h)
	requirePgErrorCode(t, err, sqlStateForeignKeyViolation)
}

func TestHoldingRepo_Create_UnknownProductIDRejectedByForeignKey(t *testing.T) {
	tx := newTestTx(t)
	account, _ := holdingFixture(t, tx)
	repo := salakrepo.NewGormHoldingRepository(tx)

	h := newHolding(account.ID, uuid.New(), 2, 1, 2)
	err := repo.Create(context.Background(), tx, h)
	requirePgErrorCode(t, err, sqlStateForeignKeyViolation)
}

func TestHoldingRepo_FindByAccountID_OrdersByPurchaseDateDescending(t *testing.T) {
	tx := newTestTx(t)
	account, product := holdingFixture(t, tx)
	repo := salakrepo.NewGormHoldingRepository(tx)
	ctx := context.Background()

	older := newHolding(account.ID, product.ID, 2, 1, 2)
	older.PurchaseDate = time.Now().UTC().AddDate(0, 0, -5)
	require.NoError(t, repo.Create(ctx, tx, older))

	newer := newHolding(account.ID, product.ID, 2, 3, 4)
	newer.PurchaseDate = time.Now().UTC()
	require.NoError(t, repo.Create(ctx, tx, newer))

	got, err := repo.FindByAccountID(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, newer.ID, got[0].ID, "most recent purchase_date must come first")
	assert.Equal(t, older.ID, got[1].ID)
}

func TestHoldingRepo_ReserveTicketRange_SequentialCallsAreContiguous(t *testing.T) {
	tx := newTestTx(t)
	_, product := holdingFixture(t, tx)
	repo := salakrepo.NewGormHoldingRepository(tx)
	ctx := context.Background()

	letter1, start1, end1, err := repo.ReserveTicketRange(ctx, tx, product.ID, 5)
	require.NoError(t, err)
	assert.Equal(t, "ก", letter1, "a fresh product's cursor starts at ก")
	assert.Equal(t, start1+4, end1)

	letter2, start2, end2, err := repo.ReserveTicketRange(ctx, tx, product.ID, 3)
	require.NoError(t, err)
	assert.Equal(t, letter1, letter2, "second reservation must stay on the same letter - this is exactly the bug being fixed")
	assert.Equal(t, end1+1, start2, "second reservation must start immediately after the first ends")
	assert.Equal(t, start2+2, end2)
}

// TestHoldingRepo_ReserveTicketRange_AdvancesLetterWhenBlockDoesNotFit
// reproduces the user's own description of the desired behavior: if a
// letter's block doesn't have enough numbers left for the whole purchase,
// the entire purchase moves to the next letter's 0 rather than splitting
// across the boundary.
func TestHoldingRepo_ReserveTicketRange_AdvancesLetterWhenBlockDoesNotFit(t *testing.T) {
	tx := newTestTx(t)
	_, product := holdingFixture(t, tx)
	repo := salakrepo.NewGormHoldingRepository(tx)
	ctx := context.Background()

	// Only 2 numbers left under ก (9999998, 9999999) - a 5-unit purchase
	// can't fit.
	require.NoError(t, tx.Model(&salakdomain.TicketSequence{}).
		Where("product_id = ?", product.ID).
		Update("next_ticket_number", 9_999_998).Error)

	letter, start, end, err := repo.ReserveTicketRange(ctx, tx, product.ID, 5)
	require.NoError(t, err)
	assert.Equal(t, "ข", letter, "must skip to the next letter, not split across the boundary")
	assert.EqualValues(t, 0, start, "the whole purchase lands at the fresh letter's 0, abandoning ก's leftover 2-ticket tail")
	assert.EqualValues(t, 4, end)

	var seq salakdomain.TicketSequence
	require.NoError(t, tx.Where("product_id = ?", product.ID).First(&seq).Error)
	assert.Equal(t, "ข", seq.NextTicketLetter)
	assert.EqualValues(t, 5, seq.NextTicketNumber)
}

func TestHoldingRepo_ReserveTicketRange_UnitsExceedingSingleLetterCapacityRejected(t *testing.T) {
	tx := newTestTx(t)
	_, product := holdingFixture(t, tx)
	repo := salakrepo.NewGormHoldingRepository(tx)
	ctx := context.Background()

	_, _, _, err := repo.ReserveTicketRange(ctx, tx, product.ID, 10_000_001)
	require.Error(t, err)
	assert.True(t, errors.Is(err, salak.ErrUnitsExceedLetterCapacity))
}

// TestReserveTicketRange_ConcurrentCallersGetDisjointContiguousRanges is the
// single most valuable test in this suite: it proves the row lock in
// ReserveTicketRange actually serializes access under real concurrency, a
// guarantee unit-test fakes cannot check (a fake's "lock" is just a Go mutex
// around in-memory state, unrelated to whether the real SQL clause works).
//
// This deliberately breaks the rollback-per-test pattern used everywhere
// else in this suite: a *gorm.DB bound to one *sql.Tx isn't safe for
// concurrent goroutines (same rule as database/sql's *sql.Tx), so each
// "concurrent caller" here is a genuinely separate Begin()+Commit() against
// sharedDB, against a product created directly on sharedDB (not a
// rollback-per-test tx, so it's visible to every goroutine's own
// transaction) - which means it permanently advances that product's real
// salak.ticket_sequence cursor. Assertions are therefore delta-based
// (relative to a snapshot taken at the start), not absolute.
func TestReserveTicketRange_ConcurrentCallersGetDisjointContiguousRanges(t *testing.T) {
	if sharedDB == nil {
		t.Skip("integration DB unreachable; run `make docker-up migrate-up` first")
	}
	product := mustCreateProduct(t, sharedDB, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))
	repo := salakrepo.NewGormHoldingRepository(sharedDB)

	var before salakdomain.TicketSequence
	require.NoError(t, sharedDB.Where("product_id = ?", product.ID).First(&before).Error)

	const n, unitsPerCall = 20, 5
	type reservation struct {
		letter     string
		start, end int64
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []reservation
	var errs []error

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx := sharedDB.Begin() // separate connection per goroutine - the point
			letter, start, end, err := repo.ReserveTicketRange(context.Background(), tx, product.ID, unitsPerCall)
			if err != nil {
				tx.Rollback()
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			commitErr := tx.Commit().Error
			mu.Lock()
			defer mu.Unlock()
			if commitErr != nil {
				errs = append(errs, commitErr)
				return
			}
			results = append(results, reservation{letter, start, end})
		}()
	}
	wg.Wait()

	require.Empty(t, errs)
	require.Len(t, results, n)

	sort.Slice(results, func(i, j int) bool { return results[i].start < results[j].start })

	assert.Equal(t, before.NextTicketNumber, results[0].start)
	for i, r := range results {
		assert.Equal(t, before.NextTicketLetter, r.letter, "all %d reservations stay under one product's current letter, well short of its 10M capacity", n)
		assert.Equal(t, int64(unitsPerCall-1), r.end-r.start, "reservation %d must span exactly %d tickets", i, unitsPerCall)
	}
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[i-1].end+1, results[i].start, "gap or overlap between concurrent reservations at index %d", i)
	}

	var after salakdomain.TicketSequence
	require.NoError(t, sharedDB.Where("product_id = ?", product.ID).First(&after).Error)
	assert.Equal(t, before.NextTicketNumber+int64(n*unitsPerCall), after.NextTicketNumber)
}

// TestReserveTicketRange_TwoProductsAdvanceIndependentlyWithoutContending
// proves the per-product re-key's headline benefit (docs/GAPS.md §2.3):
// two products' cursors are entirely independent rows, so concurrent
// purchases of different products never block or interleave with each
// other.
func TestReserveTicketRange_TwoProductsAdvanceIndependentlyWithoutContending(t *testing.T) {
	if sharedDB == nil {
		t.Skip("integration DB unreachable; run `make docker-up migrate-up` first")
	}
	productA := mustCreateProduct(t, sharedDB, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))
	productB := mustCreateProduct(t, sharedDB, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))
	repo := salakrepo.NewGormHoldingRepository(sharedDB)

	const n, unitsPerCall = 10, 5
	type reservation struct {
		productID  uuid.UUID
		letter     string
		start, end int64
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []reservation

	for i := 0; i < n; i++ {
		for _, productID := range []uuid.UUID{productA.ID, productB.ID} {
			wg.Add(1)
			go func(productID uuid.UUID) {
				defer wg.Done()
				tx := sharedDB.Begin()
				letter, start, end, err := repo.ReserveTicketRange(context.Background(), tx, productID, unitsPerCall)
				require.NoError(t, err)
				require.NoError(t, tx.Commit().Error)
				mu.Lock()
				defer mu.Unlock()
				results = append(results, reservation{productID, letter, start, end})
			}(productID)
		}
	}
	wg.Wait()

	require.Len(t, results, 2*n)
	byProduct := map[uuid.UUID][]reservation{}
	for _, r := range results {
		byProduct[r.productID] = append(byProduct[r.productID], r)
	}
	require.Len(t, byProduct[productA.ID], n)
	require.Len(t, byProduct[productB.ID], n)

	for _, productID := range []uuid.UUID{productA.ID, productB.ID} {
		rs := byProduct[productID]
		sort.Slice(rs, func(i, j int) bool { return rs[i].start < rs[j].start })
		for i := 1; i < len(rs); i++ {
			assert.Equal(t, rs[i-1].end+1, rs[i].start, "product %s: gap or overlap at index %d", productID, i)
		}
	}
}
