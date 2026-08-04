package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fixedNow is the instant every test's injected clock reports, chosen away
// from the real wall clock so a test that accidentally used time.Now()
// instead of the clock seam would fail rather than pass by coincidence.
var fixedNow = time.Date(2026, 1, 15, 3, 4, 5, 0, time.UTC)

func testClock() clock.Clock { return clock.Fixed(fixedNow) }

// --- fakes ---------------------------------------------------------------

type fakeProductRepo struct {
	active      []domain.Product
	byID        map[uuid.UUID]domain.Product
	listErr     error
	findByIDErr error
}

func newFakeProductRepo(products ...domain.Product) *fakeProductRepo {
	r := &fakeProductRepo{byID: map[uuid.UUID]domain.Product{}}
	for _, p := range products {
		r.byID[p.ID] = p
		if p.IsActive {
			r.active = append(r.active, p)
		}
	}
	return r
}

func (f *fakeProductRepo) ListActive(ctx context.Context) ([]domain.Product, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.active, nil
}

func (f *fakeProductRepo) FindByID(ctx context.Context, id uuid.UUID) (domain.Product, error) {
	if f.findByIDErr != nil {
		return domain.Product{}, f.findByIDErr
	}
	p, ok := f.byID[id]
	if !ok {
		return domain.Product{}, gorm.ErrRecordNotFound
	}
	return p, nil
}

func (f *fakeProductRepo) FindByCode(ctx context.Context, code string) (domain.Product, error) {
	for _, p := range f.byID {
		if p.Code == code {
			return p, nil
		}
	}
	return domain.Product{}, gorm.ErrRecordNotFound
}

func (f *fakeProductRepo) Upsert(ctx context.Context, p *domain.Product) error {
	f.byID[p.ID] = *p
	return nil
}

type fakeHoldingRepo struct {
	byAccountID map[uuid.UUID][]domain.Holding

	reserveStart, reserveEnd int64
	reserveErr               error
	createErr                error
	findByAccountErr         error

	lastReservedUnits int64
	lastCreated       *domain.Holding
}

func newFakeHoldingRepo() *fakeHoldingRepo {
	return &fakeHoldingRepo{byAccountID: map[uuid.UUID][]domain.Holding{}}
}

func (f *fakeHoldingRepo) Create(ctx context.Context, tx *gorm.DB, h *domain.Holding) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.lastCreated = h
	f.byAccountID[h.AccountID] = append(f.byAccountID[h.AccountID], *h)
	return nil
}

func (f *fakeHoldingRepo) FindByAccountID(ctx context.Context, accountID uuid.UUID) ([]domain.Holding, error) {
	if f.findByAccountErr != nil {
		return nil, f.findByAccountErr
	}
	return f.byAccountID[accountID], nil
}

func (f *fakeHoldingRepo) ReserveTicketRange(ctx context.Context, tx *gorm.DB, units int64) (int64, int64, error) {
	if f.reserveErr != nil {
		return 0, 0, f.reserveErr
	}
	f.lastReservedUnits = units
	return f.reserveStart, f.reserveEnd, nil
}

// fakeDrawDateRepo is a hand-rolled implementation of salak.DrawDateRepository.
type fakeDrawDateRepo struct {
	isDrawDay    bool
	isDrawDayErr error

	lastProductID uuid.UUID
	lastDate      time.Time
}

func newFakeDrawDateRepo() *fakeDrawDateRepo { return &fakeDrawDateRepo{} }

func (f *fakeDrawDateRepo) IsDrawDay(ctx context.Context, productID uuid.UUID, date time.Time) (bool, error) {
	f.lastProductID, f.lastDate = productID, date
	if f.isDrawDayErr != nil {
		return false, f.isDrawDayErr
	}
	return f.isDrawDay, nil
}

func (f *fakeDrawDateRepo) Create(ctx context.Context, d *domain.DrawDate) error {
	return nil
}

func (f *fakeDrawDateRepo) FurthestDrawDate(ctx context.Context, productID uuid.UUID) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

// fakeAccountService is a hand-rolled implementation of account.Service,
// only GetByID is exercised by SalakService.
type fakeAccountService struct {
	getByIDResult accountdomain.Account
	getByIDErr    error

	lastUserID, lastAccountID uuid.UUID
}

func (f *fakeAccountService) ListByUser(ctx context.Context, userID uuid.UUID) ([]accountdomain.Account, error) {
	return nil, nil
}

func (f *fakeAccountService) GetByID(ctx context.Context, userID, accountID uuid.UUID) (accountdomain.Account, error) {
	f.lastUserID, f.lastAccountID = userID, accountID
	if f.getByIDErr != nil {
		return accountdomain.Account{}, f.getByIDErr
	}
	return f.getByIDResult, nil
}

func (f *fakeAccountService) Debit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	return decimal.Zero, nil
}

func (f *fakeAccountService) Credit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	return decimal.Zero, nil
}

func (f *fakeAccountService) LockForUpdate(ctx context.Context, tx *gorm.DB, accountID uuid.UUID) (accountdomain.Account, error) {
	return accountdomain.Account{}, nil
}

func (f *fakeAccountService) GetByIDUnscoped(ctx context.Context, accountID uuid.UUID) (accountdomain.Account, error) {
	return f.GetByID(ctx, uuid.Nil, accountID)
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

func activeProduct() domain.Product {
	return domain.Product{
		ID:          uuid.New(),
		Code:        "SALAK-3M",
		Name:        "Salak 3-Month",
		TermMonths:  3,
		UnitPrice:   decimal.RequireFromString("100"),
		MinPurchase: decimal.RequireFromString("100"),
		MaxPurchase: decimal.RequireFromString("1000000"),
		StepAmount:  decimal.RequireFromString("100"),
		IsActive:    true,
	}
}

// --- ListProducts ---------------------------------------------------------

func TestSalakService_ListProducts(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		p := activeProduct()
		products := newFakeProductRepo(p)
		svc := service.NewSalakService(products, newFakeHoldingRepo(), &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		got, err := svc.ListProducts(context.Background())
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, p.ID, got[0].ID)
	})

	t.Run("repo error returns internal error", func(t *testing.T) {
		products := newFakeProductRepo()
		products.listErr = errors.New("db down")
		svc := service.NewSalakService(products, newFakeHoldingRepo(), &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		_, err := svc.ListProducts(context.Background())
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

// --- GetProduct -------------------------------------------------------------

func TestSalakService_GetProduct(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		p := activeProduct()
		products := newFakeProductRepo(p)
		svc := service.NewSalakService(products, newFakeHoldingRepo(), &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		got, err := svc.GetProduct(context.Background(), p.ID)
		require.NoError(t, err)
		assert.Equal(t, p.ID, got.ID)
	})

	t.Run("not found", func(t *testing.T) {
		products := newFakeProductRepo()
		svc := service.NewSalakService(products, newFakeHoldingRepo(), &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		_, err := svc.GetProduct(context.Background(), uuid.New())
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("repo error returns internal error", func(t *testing.T) {
		products := newFakeProductRepo()
		products.findByIDErr = errors.New("db down")
		svc := service.NewSalakService(products, newFakeHoldingRepo(), &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		_, err := svc.GetProduct(context.Background(), uuid.New())
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("inactive product is not purchasable", func(t *testing.T) {
		p := activeProduct()
		p.IsActive = false
		products := newFakeProductRepo(p)
		svc := service.NewSalakService(products, newFakeHoldingRepo(), &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		_, err := svc.GetProduct(context.Background(), p.ID)
		assertAppErrKind(t, err, apperror.KindValidation)
	})
}

// --- ValidatePurchase --------------------------------------------------------

func TestSalakService_ValidatePurchase(t *testing.T) {
	product := activeProduct() // min=100, max=1_000_000, step=100

	svc := service.NewSalakService(newFakeProductRepo(), newFakeHoldingRepo(), &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

	cases := []struct {
		name    string
		amount  decimal.Decimal
		wantErr bool
	}{
		{"zero amount rejected", decimal.Zero, true},
		{"negative amount rejected", mustDecimal(t, "-100"), true},
		{"below minimum rejected", mustDecimal(t, "50"), true},
		{"above maximum rejected", mustDecimal(t, "1000001"), true},
		{"not a multiple of step rejected", mustDecimal(t, "150"), true},
		{"exactly at minimum boundary is valid", mustDecimal(t, "100"), false},
		{"exactly at maximum boundary is valid", mustDecimal(t, "1000000"), false},
		{"valid multiple of step in range", mustDecimal(t, "500"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.ValidatePurchase(product, tc.amount)
			if tc.wantErr {
				assertAppErrKind(t, err, apperror.KindValidation)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- EnsureNotDrawDay --------------------------------------------------------

func TestSalakService_EnsureNotDrawDay(t *testing.T) {
	product := activeProduct()

	t.Run("not a draw day succeeds and checks against today per the injected clock", func(t *testing.T) {
		drawDates := newFakeDrawDateRepo()
		svc := service.NewSalakService(newFakeProductRepo(), newFakeHoldingRepo(), &fakeAccountService{}, drawDates, testClock())

		err := svc.EnsureNotDrawDay(context.Background(), product)
		require.NoError(t, err)
		assert.Equal(t, product.ID, drawDates.lastProductID)
		assert.Equal(t, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), drawDates.lastDate)
	})

	t.Run("draw day is rejected with an error recognisable as retryable", func(t *testing.T) {
		drawDates := newFakeDrawDateRepo()
		drawDates.isDrawDay = true
		svc := service.NewSalakService(newFakeProductRepo(), newFakeHoldingRepo(), &fakeAccountService{}, drawDates, testClock())

		err := svc.EnsureNotDrawDay(context.Background(), product)
		assertAppErrKind(t, err, apperror.KindValidation)
		assert.True(t, errors.Is(err, salak.ErrDrawDay), "expected errors.Is(err, salak.ErrDrawDay), got: %v", err)
	})

	t.Run("repo error returns internal error, not treated as a draw day", func(t *testing.T) {
		drawDates := newFakeDrawDateRepo()
		drawDates.isDrawDayErr = errors.New("db down")
		svc := service.NewSalakService(newFakeProductRepo(), newFakeHoldingRepo(), &fakeAccountService{}, drawDates, testClock())

		err := svc.EnsureNotDrawDay(context.Background(), product)
		assertAppErrKind(t, err, apperror.KindInternal)
		assert.False(t, errors.Is(err, salak.ErrDrawDay))
	})
}

// --- MintHolding --------------------------------------------------------

func TestSalakService_MintHolding(t *testing.T) {
	accountID := uuid.New()

	t.Run("success computes units, ticket range, and maturity date", func(t *testing.T) {
		product := activeProduct() // unit price 100, term 3 months
		products := newFakeProductRepo(product)
		holdings := newFakeHoldingRepo()
		holdings.reserveStart, holdings.reserveEnd = 1000, 1004
		svc := service.NewSalakService(products, holdings, &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		h, err := svc.MintHolding(context.Background(), nil, accountID, product.ID, mustDecimal(t, "500"))
		require.NoError(t, err)

		assert.Equal(t, accountID, h.AccountID)
		assert.Equal(t, product.ID, h.ProductID)
		assert.EqualValues(t, 5, h.Units)
		assert.EqualValues(t, 5, holdings.lastReservedUnits)
		assert.EqualValues(t, 1000, h.TicketStart)
		assert.EqualValues(t, 1004, h.TicketEnd)
		assert.True(t, mustDecimal(t, "500").Equal(h.PurchaseAmount))
		// Asserted against the injected clock's fixed instant, not
		// time.Now() - this is what proves MintHolding actually reads the
		// clock seam rather than the wall clock.
		wantPurchaseDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, wantPurchaseDate, h.PurchaseDate)
		assert.Equal(t, wantPurchaseDate.AddDate(0, product.TermMonths, 0), h.MaturityDate)
		letterRunes := []rune(h.TicketLetter)
		require.Len(t, letterRunes, 1)
		assert.True(t, letterRunes[0] >= 0x0E01 && letterRunes[0] <= 0x0E2E, "ticket letter %q out of the Thai consonant range", h.TicketLetter)
		require.NotNil(t, holdings.lastCreated)
		assert.Equal(t, h.ID, holdings.lastCreated.ID)
	})

	t.Run("amount truncates to whole units when not an exact multiple", func(t *testing.T) {
		product := activeProduct() // unit price 100
		products := newFakeProductRepo(product)
		holdings := newFakeHoldingRepo()
		svc := service.NewSalakService(products, holdings, &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		h, err := svc.MintHolding(context.Background(), nil, accountID, product.ID, mustDecimal(t, "250"))
		require.NoError(t, err)
		assert.EqualValues(t, 2, h.Units)
	})

	t.Run("amount below one unit price is rejected", func(t *testing.T) {
		product := activeProduct() // unit price 100
		products := newFakeProductRepo(product)
		svc := service.NewSalakService(products, newFakeHoldingRepo(), &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		_, err := svc.MintHolding(context.Background(), nil, accountID, product.ID, mustDecimal(t, "50"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("zero amount is rejected", func(t *testing.T) {
		product := activeProduct()
		products := newFakeProductRepo(product)
		svc := service.NewSalakService(products, newFakeHoldingRepo(), &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		_, err := svc.MintHolding(context.Background(), nil, accountID, product.ID, decimal.Zero)
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("product not found", func(t *testing.T) {
		svc := service.NewSalakService(newFakeProductRepo(), newFakeHoldingRepo(), &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		_, err := svc.MintHolding(context.Background(), nil, accountID, uuid.New(), mustDecimal(t, "100"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("product lookup error returns internal error", func(t *testing.T) {
		products := newFakeProductRepo()
		products.findByIDErr = errors.New("db down")
		svc := service.NewSalakService(products, newFakeHoldingRepo(), &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		_, err := svc.MintHolding(context.Background(), nil, accountID, uuid.New(), mustDecimal(t, "100"))
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("ticket range reservation failure returns internal error", func(t *testing.T) {
		product := activeProduct()
		products := newFakeProductRepo(product)
		holdings := newFakeHoldingRepo()
		holdings.reserveErr = errors.New("lock timeout")
		svc := service.NewSalakService(products, holdings, &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		_, err := svc.MintHolding(context.Background(), nil, accountID, product.ID, mustDecimal(t, "100"))
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("holding create failure returns internal error", func(t *testing.T) {
		product := activeProduct()
		products := newFakeProductRepo(product)
		holdings := newFakeHoldingRepo()
		holdings.createErr = errors.New("write failed")
		svc := service.NewSalakService(products, holdings, &fakeAccountService{}, newFakeDrawDateRepo(), testClock())

		_, err := svc.MintHolding(context.Background(), nil, accountID, product.ID, mustDecimal(t, "100"))
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

// --- ListHoldingsByAccount --------------------------------------------------

func TestSalakService_ListHoldingsByAccount(t *testing.T) {
	userID := uuid.New()
	accountID := uuid.New()

	t.Run("success after ownership check passes", func(t *testing.T) {
		holdings := newFakeHoldingRepo()
		want := domain.Holding{ID: uuid.New(), AccountID: accountID}
		holdings.byAccountID[accountID] = []domain.Holding{want}
		accounts := &fakeAccountService{getByIDResult: accountdomain.Account{ID: accountID, UserID: userID}}
		svc := service.NewSalakService(newFakeProductRepo(), holdings, accounts, newFakeDrawDateRepo(), testClock())

		got, err := svc.ListHoldingsByAccount(context.Background(), userID, accountID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, want.ID, got[0].ID)
		assert.Equal(t, userID, accounts.lastUserID)
		assert.Equal(t, accountID, accounts.lastAccountID)
	})

	t.Run("ownership check failure is propagated verbatim", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDErr: apperror.NotFound("account not found")}
		svc := service.NewSalakService(newFakeProductRepo(), newFakeHoldingRepo(), accounts, newFakeDrawDateRepo(), testClock())

		_, err := svc.ListHoldingsByAccount(context.Background(), userID, accountID)
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("holding repo error returns internal error", func(t *testing.T) {
		holdings := newFakeHoldingRepo()
		holdings.findByAccountErr = errors.New("db down")
		accounts := &fakeAccountService{getByIDResult: accountdomain.Account{ID: accountID, UserID: userID}}
		svc := service.NewSalakService(newFakeProductRepo(), holdings, accounts, newFakeDrawDateRepo(), testClock())

		_, err := svc.ListHoldingsByAccount(context.Background(), userID, accountID)
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("no holdings returns an empty slice, not an error", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: accountdomain.Account{ID: accountID, UserID: userID}}
		svc := service.NewSalakService(newFakeProductRepo(), newFakeHoldingRepo(), accounts, newFakeDrawDateRepo(), testClock())

		got, err := svc.ListHoldingsByAccount(context.Background(), userID, accountID)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
