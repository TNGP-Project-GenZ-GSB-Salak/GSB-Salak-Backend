package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// fixedNow is the instant every test's injected clock reports, chosen away
// from the real wall clock so a test that accidentally used time.Now()
// instead of the clock seam would fail rather than pass by coincidence.
var fixedNow = time.Date(2026, 1, 15, 3, 4, 5, 0, time.UTC)

func testClock() clock.Clock { return clock.Fixed(fixedNow) }

// --- fakes ---------------------------------------------------------------

// fakeAccountService is a hand-rolled implementation of account.Service.
type fakeAccountService struct {
	accounts     map[uuid.UUID]accountdomain.Account
	getByIDErrs  map[uuid.UUID]error
	debitErr     error
	creditErr    error
	debitResult  decimal.Decimal
	creditResult decimal.Decimal

	debitCalls  []decimal.Decimal
	creditCalls []decimal.Decimal
}

func newFakeAccountService() *fakeAccountService {
	return &fakeAccountService{
		accounts:    map[uuid.UUID]accountdomain.Account{},
		getByIDErrs: map[uuid.UUID]error{},
	}
}

func (f *fakeAccountService) ListByUser(ctx context.Context, userID uuid.UUID) ([]accountdomain.Account, error) {
	return nil, nil
}

func (f *fakeAccountService) GetByID(ctx context.Context, userID, accountID uuid.UUID) (accountdomain.Account, error) {
	if err, ok := f.getByIDErrs[accountID]; ok {
		return accountdomain.Account{}, err
	}
	a, ok := f.accounts[accountID]
	if !ok {
		return accountdomain.Account{}, apperror.NotFound("account not found")
	}
	return a, nil
}

func (f *fakeAccountService) Debit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	f.debitCalls = append(f.debitCalls, amount)
	if f.debitErr != nil {
		return decimal.Zero, f.debitErr
	}
	return f.debitResult, nil
}

func (f *fakeAccountService) Credit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	f.creditCalls = append(f.creditCalls, amount)
	if f.creditErr != nil {
		return decimal.Zero, f.creditErr
	}
	return f.creditResult, nil
}

// fakeSalakService is a hand-rolled implementation of salak.Service.
type fakeSalakService struct {
	getProductResult salakdomain.Product
	getProductErr    error

	validatePurchaseErr error

	mintHoldingResult salakdomain.Holding
	mintHoldingErr    error
}

func (f *fakeSalakService) ListProducts(ctx context.Context) ([]salakdomain.Product, error) {
	return nil, nil
}

func (f *fakeSalakService) GetProduct(ctx context.Context, productID uuid.UUID) (salakdomain.Product, error) {
	if f.getProductErr != nil {
		return salakdomain.Product{}, f.getProductErr
	}
	return f.getProductResult, nil
}

func (f *fakeSalakService) ValidatePurchase(product salakdomain.Product, amount decimal.Decimal) error {
	return f.validatePurchaseErr
}

func (f *fakeSalakService) MintHolding(ctx context.Context, tx *gorm.DB, accountID, productID uuid.UUID, amount decimal.Decimal) (salakdomain.Holding, error) {
	if f.mintHoldingErr != nil {
		return salakdomain.Holding{}, f.mintHoldingErr
	}
	return f.mintHoldingResult, nil
}

func (f *fakeSalakService) ListHoldingsByAccount(ctx context.Context, userID, accountID uuid.UUID) ([]salakdomain.Holding, error) {
	return nil, nil
}

// fakeLedgerRepo is a hand-rolled implementation of transaction.LedgerRepository.
type fakeLedgerRepo struct {
	createErr error
	created   []domain.LedgerEntry

	findByAccountResult   []domain.LedgerEntry
	findByAccountErr      error
	lastLimit, lastOffset int
}

func (f *fakeLedgerRepo) Create(ctx context.Context, tx *gorm.DB, e *domain.LedgerEntry) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, *e)
	return nil
}

func (f *fakeLedgerRepo) FindByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]domain.LedgerEntry, error) {
	f.lastLimit, f.lastOffset = limit, offset
	if f.findByAccountErr != nil {
		return nil, f.findByAccountErr
	}
	return f.findByAccountResult, nil
}

// fakeBadgeService is a hand-rolled implementation of badge.Service.
type fakeBadgeService struct {
	owns    bool
	ownsErr error
	called  bool
}

func (f *fakeBadgeService) UserOwnsBadge(ctx context.Context, userID, badgeID uuid.UUID) (bool, error) {
	f.called = true
	if f.ownsErr != nil {
		return false, f.ownsErr
	}
	return f.owns, nil
}

// --- helpers ---------------------------------------------------------------

func assertAppErrKind(t *testing.T, err error, kind apperror.Kind) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, kind, appErr.Kind)
}

func mustDecimal(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	require.NoError(t, err)
	return d
}

// newSQLMockDB backs a real *gorm.DB with sqlmock so BuySalakService's
// s.db.Transaction(...) call can actually Begin/Commit/Rollback without a
// live Postgres connection: gorm's postgres.Dialector.Initialize sets
// db.ConnPool directly to the supplied *sql.DB when Conn is set, issuing no
// setup queries, and Begin/Commit/Rollback go through database/sql - which
// sqlmock intercepts regardless of the postgres dialect string translation.
func newSQLMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return gdb, mock
}

func savingsAccount(id uuid.UUID) accountdomain.Account {
	return accountdomain.Account{ID: id, Type: accountdomain.TypeSavings, Balance: decimal.RequireFromString("1000")}
}

func salakAccount(id uuid.UUID) accountdomain.Account {
	return accountdomain.Account{ID: id, Type: accountdomain.TypeSalak, Balance: decimal.Zero}
}

func activeProduct() salakdomain.Product {
	return salakdomain.Product{
		ID:         uuid.New(),
		Name:       "Salak 3-Month",
		TermMonths: 3,
		IsActive:   true,
	}
}

// --- BuySalak ---------------------------------------------------------------

func TestBuySalakService_BuySalak(t *testing.T) {
	userID := uuid.New()

	t.Run("funding and salak account must differ", func(t *testing.T) {
		accountID := uuid.New()
		accounts := newFakeAccountService()
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, &fakeLedgerRepo{}, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.BuySalak(context.Background(), userID, accountID, accountID, uuid.New(), nil, mustDecimal(t, "100"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("funding account lookup failure is propagated verbatim", func(t *testing.T) {
		fundingID, salakID := uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.getByIDErrs[fundingID] = apperror.NotFound("account not found")
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, &fakeLedgerRepo{}, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, uuid.New(), nil, mustDecimal(t, "100"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("funding account must be savings-type", func(t *testing.T) {
		fundingID, salakID := uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = salakAccount(fundingID) // wrong type on purpose
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, &fakeLedgerRepo{}, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, uuid.New(), nil, mustDecimal(t, "100"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("salak account lookup failure is propagated verbatim", func(t *testing.T) {
		fundingID, salakID := uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.getByIDErrs[salakID] = apperror.NotFound("account not found")
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, &fakeLedgerRepo{}, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, uuid.New(), nil, mustDecimal(t, "100"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("salak account must be salak-type", func(t *testing.T) {
		fundingID, salakID := uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.accounts[salakID] = savingsAccount(salakID) // wrong type on purpose
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, &fakeLedgerRepo{}, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, uuid.New(), nil, mustDecimal(t, "100"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("product lookup failure is propagated verbatim", func(t *testing.T) {
		fundingID, salakID := uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.accounts[salakID] = salakAccount(salakID)
		salakSvc := &fakeSalakService{getProductErr: apperror.NotFound("salak product not found")}
		svc := service.NewBuySalakService(nil, accounts, salakSvc, &fakeLedgerRepo{}, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, uuid.New(), nil, mustDecimal(t, "100"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("purchase validation failure is propagated verbatim", func(t *testing.T) {
		fundingID, salakID := uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.accounts[salakID] = salakAccount(salakID)
		salakSvc := &fakeSalakService{
			getProductResult:    activeProduct(),
			validatePurchaseErr: apperror.Validation("amount must be a multiple of the step amount"),
		}
		svc := service.NewBuySalakService(nil, accounts, salakSvc, &fakeLedgerRepo{}, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, uuid.New(), nil, mustDecimal(t, "150"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("success commits the transaction and returns a full receipt", func(t *testing.T) {
		fundingID, salakID, productID := uuid.New(), uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.accounts[salakID] = salakAccount(salakID)
		accounts.debitResult = mustDecimal(t, "900")
		accounts.creditResult = mustDecimal(t, "500")

		product := activeProduct()
		holding := salakdomain.Holding{
			ID:          uuid.New(),
			Units:       5,
			TicketStart: 1000,
			TicketEnd:   1004,
		}
		salakSvc := &fakeSalakService{getProductResult: product, mintHoldingResult: holding}
		ledger := &fakeLedgerRepo{}
		badgeSvc := &fakeBadgeService{owns: true}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewBuySalakService(db, accounts, salakSvc, ledger, badgeSvc, testClock())

		receipt, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, productID, nil, mustDecimal(t, "500"))
		require.NoError(t, err)

		assert.Equal(t, product.Name, receipt.ProductName)
		assert.EqualValues(t, 5, receipt.Units)
		assert.True(t, mustDecimal(t, "500").Equal(receipt.Amount))
		assert.True(t, mustDecimal(t, "900").Equal(receipt.FundingAccountBalanceAfter))
		assert.True(t, mustDecimal(t, "500").Equal(receipt.SalakAccountBalanceAfter))
		assert.NotEqual(t, uuid.Nil, receipt.ReferenceID)

		require.Len(t, ledger.created, 2)
		debitEntry, creditEntry := ledger.created[0], ledger.created[1]
		assert.Equal(t, domain.EntryDebit, debitEntry.Type)
		assert.Equal(t, fundingID, debitEntry.AccountID)
		assert.Equal(t, domain.EntryCredit, creditEntry.Type)
		assert.Equal(t, salakID, creditEntry.AccountID)
		assert.Equal(t, debitEntry.ReferenceID, creditEntry.ReferenceID, "debit/credit must share one reference_id")
		assert.Equal(t, receipt.ReferenceID, debitEntry.ReferenceID)
		require.NotNil(t, debitEntry.HoldingID)
		assert.Equal(t, holding.ID, *debitEntry.HoldingID)
		// Asserted against the injected clock's fixed instant, not
		// time.Now() - this is what proves BuySalak actually reads the
		// clock seam rather than the wall clock.
		assert.Equal(t, fixedNow, debitEntry.CreatedAt)
		assert.Equal(t, fixedNow, creditEntry.CreatedAt)

		assert.False(t, badgeSvc.called, "badge ownership must not be checked when no badge is supplied")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("badge supplied and owned succeeds", func(t *testing.T) {
		fundingID, salakID, productID, badgeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.accounts[salakID] = salakAccount(salakID)

		salakSvc := &fakeSalakService{getProductResult: activeProduct(), mintHoldingResult: salakdomain.Holding{ID: uuid.New(), Units: 5}}
		badgeSvc := &fakeBadgeService{owns: true}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewBuySalakService(db, accounts, salakSvc, &fakeLedgerRepo{}, badgeSvc, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, productID, &badgeID, mustDecimal(t, "500"))
		require.NoError(t, err)
		assert.True(t, badgeSvc.called)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("badge supplied but not owned is rejected before any transaction opens", func(t *testing.T) {
		fundingID, salakID, productID, badgeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.accounts[salakID] = salakAccount(salakID)

		salakSvc := &fakeSalakService{getProductResult: activeProduct()}
		badgeSvc := &fakeBadgeService{owns: false}
		svc := service.NewBuySalakService(nil, accounts, salakSvc, &fakeLedgerRepo{}, badgeSvc, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, productID, &badgeID, mustDecimal(t, "500"))
		assertAppErrKind(t, err, apperror.KindForbidden)
	})

	t.Run("badge ownership check error returns internal error", func(t *testing.T) {
		fundingID, salakID, productID, badgeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.accounts[salakID] = salakAccount(salakID)

		salakSvc := &fakeSalakService{getProductResult: activeProduct()}
		badgeSvc := &fakeBadgeService{ownsErr: errors.New("db down")}
		svc := service.NewBuySalakService(nil, accounts, salakSvc, &fakeLedgerRepo{}, badgeSvc, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, productID, &badgeID, mustDecimal(t, "500"))
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("debit failure rolls back and is propagated verbatim", func(t *testing.T) {
		fundingID, salakID, productID := uuid.New(), uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.accounts[salakID] = salakAccount(salakID)
		accounts.debitErr = apperror.Validation("insufficient funds")

		salakSvc := &fakeSalakService{getProductResult: activeProduct()}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewBuySalakService(db, accounts, salakSvc, &fakeLedgerRepo{}, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, productID, nil, mustDecimal(t, "500"))
		assertAppErrKind(t, err, apperror.KindValidation)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("mint holding failure rolls back and is propagated verbatim", func(t *testing.T) {
		fundingID, salakID, productID := uuid.New(), uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.accounts[salakID] = salakAccount(salakID)

		salakSvc := &fakeSalakService{
			getProductResult: activeProduct(),
			mintHoldingErr:   apperror.Internal("failed to reserve ticket range", errors.New("lock timeout")),
		}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewBuySalakService(db, accounts, salakSvc, &fakeLedgerRepo{}, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, productID, nil, mustDecimal(t, "500"))
		assertAppErrKind(t, err, apperror.KindInternal)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("credit failure rolls back and is propagated verbatim", func(t *testing.T) {
		fundingID, salakID, productID := uuid.New(), uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.accounts[salakID] = salakAccount(salakID)
		accounts.creditErr = errors.New("update failed")

		salakSvc := &fakeSalakService{
			getProductResult:  activeProduct(),
			mintHoldingResult: salakdomain.Holding{ID: uuid.New(), Units: 5},
		}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewBuySalakService(db, accounts, salakSvc, &fakeLedgerRepo{}, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, productID, nil, mustDecimal(t, "500"))
		assert.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ledger write failure rolls back the whole transaction", func(t *testing.T) {
		fundingID, salakID, productID := uuid.New(), uuid.New(), uuid.New()
		accounts := newFakeAccountService()
		accounts.accounts[fundingID] = savingsAccount(fundingID)
		accounts.accounts[salakID] = salakAccount(salakID)

		salakSvc := &fakeSalakService{
			getProductResult:  activeProduct(),
			mintHoldingResult: salakdomain.Holding{ID: uuid.New(), Units: 5},
		}
		ledger := &fakeLedgerRepo{createErr: errors.New("disk full")}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewBuySalakService(db, accounts, salakSvc, ledger, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.BuySalak(context.Background(), userID, fundingID, salakID, productID, nil, mustDecimal(t, "500"))
		assert.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- ListHistory --------------------------------------------------------

func TestBuySalakService_ListHistory(t *testing.T) {
	userID := uuid.New()
	accountID := uuid.New()

	t.Run("success", func(t *testing.T) {
		accounts := newFakeAccountService()
		accounts.accounts[accountID] = savingsAccount(accountID)
		want := []domain.LedgerEntry{{ID: uuid.New(), AccountID: accountID}}
		ledger := &fakeLedgerRepo{findByAccountResult: want}
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, ledger, &fakeBadgeService{owns: true}, testClock())

		got, err := svc.ListHistory(context.Background(), userID, accountID, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("ownership check failure is propagated verbatim", func(t *testing.T) {
		accounts := newFakeAccountService()
		accounts.getByIDErrs[accountID] = apperror.NotFound("account not found")
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, &fakeLedgerRepo{}, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.ListHistory(context.Background(), userID, accountID, 10, 0)
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("non-positive limit defaults to 20", func(t *testing.T) {
		accounts := newFakeAccountService()
		accounts.accounts[accountID] = savingsAccount(accountID)
		ledger := &fakeLedgerRepo{}
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, ledger, &fakeBadgeService{owns: true}, testClock())

		for _, limit := range []int{0, -1, -100} {
			_, err := svc.ListHistory(context.Background(), userID, accountID, limit, 0)
			require.NoError(t, err)
			assert.Equal(t, 20, ledger.lastLimit)
		}
	})

	t.Run("limit above 100 is clamped to 100", func(t *testing.T) {
		accounts := newFakeAccountService()
		accounts.accounts[accountID] = savingsAccount(accountID)
		ledger := &fakeLedgerRepo{}
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, ledger, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.ListHistory(context.Background(), userID, accountID, 500, 0)
		require.NoError(t, err)
		assert.Equal(t, 100, ledger.lastLimit)
	})

	t.Run("limit exactly at the 100 boundary is not clamped", func(t *testing.T) {
		accounts := newFakeAccountService()
		accounts.accounts[accountID] = savingsAccount(accountID)
		ledger := &fakeLedgerRepo{}
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, ledger, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.ListHistory(context.Background(), userID, accountID, 100, 0)
		require.NoError(t, err)
		assert.Equal(t, 100, ledger.lastLimit)
	})

	t.Run("negative offset is clamped to 0", func(t *testing.T) {
		accounts := newFakeAccountService()
		accounts.accounts[accountID] = savingsAccount(accountID)
		ledger := &fakeLedgerRepo{}
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, ledger, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.ListHistory(context.Background(), userID, accountID, 10, -5)
		require.NoError(t, err)
		assert.Equal(t, 0, ledger.lastOffset)
	})

	t.Run("repo error returns internal error", func(t *testing.T) {
		accounts := newFakeAccountService()
		accounts.accounts[accountID] = savingsAccount(accountID)
		ledger := &fakeLedgerRepo{findByAccountErr: errors.New("db down")}
		svc := service.NewBuySalakService(nil, accounts, &fakeSalakService{}, ledger, &fakeBadgeService{owns: true}, testClock())

		_, err := svc.ListHistory(context.Background(), userID, accountID, 10, 0)
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}
