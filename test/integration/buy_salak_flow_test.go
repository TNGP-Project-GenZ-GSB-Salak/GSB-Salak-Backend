//go:build integration

package integration

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	accountrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/account/repository"
	accountservice "github.com/ciaabcdefg/gsb-salak-backend/internal/account/service"
	badgerepo "github.com/ciaabcdefg/gsb-salak-backend/internal/badge/repository"
	badgeservice "github.com/ciaabcdefg/gsb-salak-backend/internal/badge/service"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
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

// newBuySalakService wires the *real* services - not fakes - the same way
// cmd/api/main.go does, but with every repo constructed on the given tx so
// the whole cross-domain flow (including BuySalakService's own internal
// db.Transaction call, which GORM transparently turns into a SAVEPOINT since
// tx is already inside an open transaction) rolls back cleanly at
// t.Cleanup. This exists specifically to exercise db.Transaction's
// commit/rollback mechanics against real Postgres - something the
// sqlmock-based unit test for BuySalakService cannot do, since sqlmock only
// confirms Rollback() was called, not that Postgres actually undid real row
// changes.
func newBuySalakService(tx *gorm.DB) (*txservice.BuySalakService, *accountrepo.GormAccountRepository) {
	accountRepository := accountrepo.NewGormAccountRepository(tx)
	accountSvc := accountservice.NewAccountService(accountRepository)
	salakSvc := salakservice.NewSalakService(
		salakrepo.NewGormProductRepository(tx),
		salakrepo.NewGormHoldingRepository(tx),
		accountSvc,
		salakrepo.NewGormDrawDateRepository(tx),
		clock.Real{},
	)
	ledgerRepository := txrepo.NewGormLedgerRepository(tx)
	badgeSvc := badgeservice.NewBadgeService(badgerepo.NewGormBadgeRepository(tx))
	return txservice.NewBuySalakService(tx, accountSvc, salakSvc, ledgerRepository, badgeSvc, clock.Real{}), accountRepository
}

func TestBuySalakFlow_HappyPath_DebitsCreditsMintsAndLedgers(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()

	user := mustCreateUser(t, tx, "")
	funding := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	salakAccount := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	buySvc, accountRepository := newBuySalakService(tx)

	receipt, err := buySvc.BuySalak(ctx, user.ID, funding.ID, salakAccount.ID, product.ID, nil, decimal.RequireFromString("2000"))
	require.NoError(t, err)

	assert.EqualValues(t, 20, receipt.Units) // 2000 / 100 unit price
	assert.True(t, decimal.RequireFromString("8000").Equal(receipt.FundingAccountBalanceAfter))
	assert.True(t, decimal.RequireFromString("2000").Equal(receipt.SalakAccountBalanceAfter))

	gotFunding, err := accountRepository.FindByID(ctx, funding.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("8000").Equal(gotFunding.Balance))

	gotSalak, err := accountRepository.FindByID(ctx, salakAccount.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("2000").Equal(gotSalak.Balance))

	holdings, err := salakrepo.NewGormHoldingRepository(tx).FindByAccountID(ctx, salakAccount.ID)
	require.NoError(t, err)
	require.Len(t, holdings, 1)
	assert.EqualValues(t, 20, holdings[0].Units)
	assert.Equal(t, holdings[0].TicketEnd-holdings[0].TicketStart+1, holdings[0].Units)

	entries, err := txrepo.NewGormLedgerRepository(tx).FindByAccountID(ctx, funding.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	debitEntry := entries[0]

	creditEntries, err := txrepo.NewGormLedgerRepository(tx).FindByAccountID(ctx, salakAccount.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, creditEntries, 1)
	creditEntry := creditEntries[0]

	assert.Equal(t, debitEntry.ReferenceID, creditEntry.ReferenceID, "debit and credit must share one reference_id")
	assert.Equal(t, receipt.ReferenceID, debitEntry.ReferenceID)
	require.NotNil(t, debitEntry.HoldingID)
	assert.Equal(t, holdings[0].ID, *debitEntry.HoldingID)
	assert.True(t, decimal.RequireFromString("8000").Equal(debitEntry.BalanceAfter))
	assert.True(t, decimal.RequireFromString("2000").Equal(creditEntry.BalanceAfter))
}

// TestBuySalakFlow_MidTransactionFailure_RollsBackEverything forces a real,
// DB-level failure strictly after the debit has already run within
// BuySalak's transaction (by deleting the product's own salak.ticket_sequence
// cursor row that ReserveTicketRange depends on, so MintHolding fails on its
// very first query), then asserts the funding account's balance is completely
// unchanged and no holding/ledger rows exist - proving the whole
// debit->mint->credit->ledger chain rolled back atomically against real
// Postgres. This is the exact failure mode the BuySalakService unit test
// (sqlmock-based) cannot detect: sqlmock only verifies that Rollback() was
// called, not that a real database actually discarded real row changes.
func TestBuySalakFlow_MidTransactionFailure_RollsBackEverything(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()

	user := mustCreateUser(t, tx, "")
	funding := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	salakAccount := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	require.NoError(t, tx.Exec(`DELETE FROM salak.ticket_sequence WHERE product_id = ?`, product.ID).Error)

	buySvc, accountRepository := newBuySalakService(tx)

	_, err := buySvc.BuySalak(ctx, user.ID, funding.ID, salakAccount.ID, product.ID, nil, decimal.RequireFromString("2000"))
	require.Error(t, err)

	gotFunding, findErr := accountRepository.FindByID(ctx, funding.ID)
	require.NoError(t, findErr)
	assert.True(t, decimal.RequireFromString("10000").Equal(gotFunding.Balance), "debit must have been rolled back, balance must be untouched")

	gotSalak, findErr := accountRepository.FindByID(ctx, salakAccount.ID)
	require.NoError(t, findErr)
	assert.True(t, decimal.Zero.Equal(gotSalak.Balance))

	holdings, findErr := salakrepo.NewGormHoldingRepository(tx).FindByAccountID(ctx, salakAccount.ID)
	require.NoError(t, findErr)
	assert.Empty(t, holdings, "no holding should have been persisted")

	entries, findErr := txrepo.NewGormLedgerRepository(tx).FindByAccountID(ctx, funding.ID, 10, 0)
	require.NoError(t, findErr)
	assert.Empty(t, entries, "no ledger entry should have been persisted")
}

// TestBuySalakFlow_BadgeSuppliedAndOwned_Succeeds proves the badge-ownership
// gate's positive path against a real badge.user_badges row (not a fake),
// confirming GormBadgeRepository.UserOwnsBadge actually finds a real grant.
func TestBuySalakFlow_BadgeSuppliedAndOwned_Succeeds(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()

	user := mustCreateUser(t, tx, "")
	funding := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	salakAccount := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))
	badge := mustCreateBadge(t, tx, uniqueProductCode())
	mustGrantUserBadge(t, tx, user.ID, badge.ID)

	buySvc, _ := newBuySalakService(tx)

	_, err := buySvc.BuySalak(ctx, user.ID, funding.ID, salakAccount.ID, product.ID, &badge.ID, decimal.RequireFromString("2000"))
	require.NoError(t, err)
}

// TestBuySalakFlow_BadgeSuppliedButNotOwned_RejectsBeforeAnyStateChanges
// proves the gate rejects real, existing badges the user was never granted -
// asserting (same style as the mid-transaction-failure test above) that
// nothing changed, since this rejection happens even earlier, before the
// transaction is ever opened.
func TestBuySalakFlow_BadgeSuppliedButNotOwned_RejectsBeforeAnyStateChanges(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()

	user := mustCreateUser(t, tx, "")
	funding := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
	salakAccount := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))
	badge := mustCreateBadge(t, tx, uniqueProductCode()) // real badge, never granted to this user

	buySvc, accountRepository := newBuySalakService(tx)

	_, err := buySvc.BuySalak(ctx, user.ID, funding.ID, salakAccount.ID, product.ID, &badge.ID, decimal.RequireFromString("2000"))
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindForbidden, appErr.Kind)

	gotFunding, findErr := accountRepository.FindByID(ctx, funding.ID)
	require.NoError(t, findErr)
	assert.True(t, decimal.RequireFromString("10000").Equal(gotFunding.Balance), "rejected before any debit could happen")

	holdings, findErr := salakrepo.NewGormHoldingRepository(tx).FindByAccountID(ctx, salakAccount.ID)
	require.NoError(t, findErr)
	assert.Empty(t, holdings, "no holding should have been persisted")
}

// TestBuySalakFlow_KapookFundingAccount_Rejected proves the public
// BuySalak path stays closed to a kapook-type account even after ticket
// 08 added BuySalakForKapook as a second door in on the same service - the
// public endpoint's own funding-type check is untouched by that addition.
func TestBuySalakFlow_KapookFundingAccount_Rejected(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()

	user := mustCreateUser(t, tx, "")
	kapookAcc := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.RequireFromString("10000"))
	salakAccount := mustCreateAccount(t, tx, user.ID, accountdomain.TypeSalak, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	buySvc, _ := newBuySalakService(tx)

	_, err := buySvc.BuySalak(ctx, user.ID, kapookAcc.ID, salakAccount.ID, product.ID, nil, decimal.RequireFromString("2000"))
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindValidation, appErr.Kind)
}

// TestBuySalakFlow_ConcurrentUsersBuyingSameProduct_NoDuplicateOrOverlappingTickets
// answers a real question about the ticket-allocation redesign (W23): does
// ReserveTicketRange's per-product row lock actually hold up once many
// distinct customers - not just many calls from one test goroutine - buy
// the same product at the same time? TestReserveTicketRange_
// ConcurrentCallersGetDisjointContiguousRanges (holding_repo_test.go)
// already proves the repository method itself is safe under concurrency;
// this test exercises the *whole* BuySalakService stack instead (debit,
// mint, credit, ledger, one top-level db.Transaction per purchase), which
// is what a real multi-user production scenario actually calls, and would
// also catch a duplicate that only the EXCLUDE constraint (not the row
// lock) ends up rejecting - i.e. a bug, not just a slow query.
//
// This deliberately breaks the rollback-per-test pattern the same way
// TestReserveTicketRange_ConcurrentCallersGetDisjointContiguousRanges
// does: a *gorm.DB bound to one *sql.Tx isn't safe for concurrent
// goroutines, so every "customer" here is a genuinely separate user,
// account pair, and top-level transaction against sharedDB.
func TestBuySalakFlow_ConcurrentUsersBuyingSameProduct_NoDuplicateOrOverlappingTickets(t *testing.T) {
	if sharedDB == nil {
		t.Skip("integration DB unreachable; run `make docker-up migrate-up` first")
	}
	ctx := context.Background()
	const numCustomers = 15
	const unitsEach = 20 // 2000 THB at unit price 100

	// All fixtures are created up front, sequentially, on the main test
	// goroutine (require/t.Fatal must never run from a spawned goroutine -
	// testing.T explicitly documents FailNow as unsafe there). Only the
	// purchase itself - the thing actually under test - runs concurrently.
	product := mustCreateProduct(t, sharedDB, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("1000000"), decimal.RequireFromString("1000"))

	type customer struct {
		userID, fundingID, salakAccountID uuid.UUID
	}
	customers := make([]customer, numCustomers)
	for i := range customers {
		user := mustCreateUser(t, sharedDB, "")
		funding := mustCreateAccount(t, sharedDB, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("10000"))
		salakAccount := mustCreateAccount(t, sharedDB, user.ID, accountdomain.TypeSalak, decimal.Zero)
		customers[i] = customer{user.ID, funding.ID, salakAccount.ID}
	}

	t.Cleanup(func() {
		var userIDs []uuid.UUID
		for _, c := range customers {
			userIDs = append(userIDs, c.userID)
		}
		// ledger_entries has both a debit row (funding account) and a
		// credit row (salak account) per purchase, both carrying the same
		// holding_id - filtering by holding_id (via this product) catches
		// both sides regardless of which account they're on, unlike
		// filtering by account_id alone.
		sharedDB.Exec(`DELETE FROM transaction.ledger_entries WHERE holding_id IN (SELECT id FROM salak.holdings WHERE product_id = ?)`, product.ID)
		sharedDB.Exec(`DELETE FROM salak.holdings WHERE product_id = ?`, product.ID)
		sharedDB.Exec(`DELETE FROM account.accounts WHERE user_id IN (?)`, userIDs)
		sharedDB.Exec(`DELETE FROM "user".users WHERE id IN (?)`, userIDs)
		sharedDB.Exec(`DELETE FROM salak.ticket_sequence WHERE product_id = ?`, product.ID)
		sharedDB.Exec(`DELETE FROM salak.products WHERE id = ?`, product.ID)
	})

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, c := range customers {
		wg.Add(1)
		go func(c customer) {
			defer wg.Done()
			buySvc, _ := newBuySalakService(sharedDB) // separate connection per call - the point
			_, err := buySvc.BuySalak(ctx, c.userID, c.fundingID, c.salakAccountID, product.ID, nil, decimal.NewFromInt(int64(unitsEach)*100))
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(c)
	}
	wg.Wait()

	require.Empty(t, errs, "no concurrent purchase should fail")

	// Read back what actually landed in the table - this is the real
	// source of truth for "did any two customers get the same ticket",
	// not whatever each goroutine's own receipt claimed.
	var holdings []salakdomain.Holding
	require.NoError(t, sharedDB.Where("product_id = ?", product.ID).Find(&holdings).Error)
	require.Len(t, holdings, numCustomers)

	type purchase struct {
		accountID  uuid.UUID
		letter     string
		start, end int64
	}
	parsed := make([]purchase, len(holdings))
	for i, h := range holdings {
		parsed[i] = purchase{accountID: h.AccountID, letter: h.TicketLetter, start: h.TicketStart, end: h.TicketEnd}
		assert.Equal(t, int64(unitsEach-1), h.TicketEnd-h.TicketStart, "each purchase must reserve exactly %d contiguous tickets", unitsEach)
	}

	// The actual duplicate-number check: sort by (letter, start) and
	// confirm every range is strictly disjoint from its neighbor - no
	// shared ticket number and no overlap, letter and number together.
	sort.Slice(parsed, func(i, j int) bool {
		if parsed[i].letter != parsed[j].letter {
			return parsed[i].letter < parsed[j].letter
		}
		return parsed[i].start < parsed[j].start
	})
	seen := map[string]uuid.UUID{}
	for _, r := range parsed {
		for n := r.start; n <= r.end; n++ {
			key := fmt.Sprintf("%s%07d", r.letter, n)
			if owner, dup := seen[key]; dup {
				t.Fatalf("duplicate ticket %s: held by both account %s and account %s", key, owner, r.accountID)
			}
			seen[key] = r.accountID
		}
	}
	for i := 1; i < len(parsed); i++ {
		prev, cur := parsed[i-1], parsed[i]
		if prev.letter != cur.letter {
			continue
		}
		assert.Less(t, prev.end, cur.start, "overlapping ranges under letter %s: %d-%d and %d-%d", cur.letter, prev.start, prev.end, cur.start, cur.end)
	}
}
