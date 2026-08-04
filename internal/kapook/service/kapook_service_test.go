package service_test

import (
	"context"
	"errors"
	"testing"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/service"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- fakes ---------------------------------------------------------------

// fakeTermsRepo is a hand-rolled implementation of kapook.TermsRepository.
type fakeTermsRepo struct {
	accepted     map[uuid.UUID]bool
	acceptErr    error
	hasAcceptErr error

	lastAcceptedUserID    uuid.UUID
	lastHasAcceptedUserID uuid.UUID
	acceptCallCount       int
}

func newFakeTermsRepo() *fakeTermsRepo {
	return &fakeTermsRepo{accepted: map[uuid.UUID]bool{}}
}

func (f *fakeTermsRepo) Accept(ctx context.Context, userID uuid.UUID) error {
	f.acceptCallCount++
	f.lastAcceptedUserID = userID
	if f.acceptErr != nil {
		return f.acceptErr
	}
	f.accepted[userID] = true
	return nil
}

func (f *fakeTermsRepo) HasAccepted(ctx context.Context, userID uuid.UUID) (bool, error) {
	f.lastHasAcceptedUserID = userID
	if f.hasAcceptErr != nil {
		return false, f.hasAcceptErr
	}
	return f.accepted[userID], nil
}

// fakeGoalRepo is a hand-rolled implementation of kapook.GoalRepository.
type fakeGoalRepo struct {
	activeByAccount map[uuid.UUID]kapookdomain.Goal
	findErr         error
	createErr       error

	lastCreated *kapookdomain.Goal
}

func newFakeGoalRepo() *fakeGoalRepo {
	return &fakeGoalRepo{activeByAccount: map[uuid.UUID]kapookdomain.Goal{}}
}

func (f *fakeGoalRepo) Create(ctx context.Context, g *kapookdomain.Goal) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.lastCreated = g
	f.activeByAccount[g.AccountID] = *g
	return nil
}

func (f *fakeGoalRepo) FindActiveByAccountID(ctx context.Context, accountID uuid.UUID) (kapookdomain.Goal, error) {
	if f.findErr != nil {
		return kapookdomain.Goal{}, f.findErr
	}
	g, ok := f.activeByAccount[accountID]
	if !ok {
		return kapookdomain.Goal{}, gorm.ErrRecordNotFound
	}
	return g, nil
}

// fakeAccountService is a hand-rolled implementation of account.Service,
// only GetByID is exercised by KapookService.
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

// fakeSalakService is a hand-rolled implementation of salak.Service, only
// GetProduct/ValidatePurchase are exercised by KapookService.
type fakeSalakService struct {
	getProductResult salakdomain.Product
	getProductErr    error

	validatePurchaseErr error
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
	return salakdomain.Holding{}, nil
}

func (f *fakeSalakService) ListHoldingsByAccount(ctx context.Context, userID, accountID uuid.UUID) ([]salakdomain.Holding, error) {
	return nil, nil
}

// --- helpers ---------------------------------------------------------------

func assertAppErrKind(t *testing.T, err error, kind apperror.Kind) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, kind, appErr.Kind)
}

func kapookAccount(id uuid.UUID, userID uuid.UUID) accountdomain.Account {
	return accountdomain.Account{ID: id, UserID: userID, Type: accountdomain.TypeKapook}
}

func activeProduct() salakdomain.Product {
	return salakdomain.Product{
		ID:          uuid.New(),
		Code:        "SALAK_1Y",
		Name:        "Digital Salak 1-Year",
		TermMonths:  12,
		UnitPrice:   decimal.RequireFromString("100"),
		MinPurchase: decimal.RequireFromString("1000"),
		MaxPurchase: decimal.RequireFromString("10000000"),
		StepAmount:  decimal.RequireFromString("1000"),
		IsActive:    true,
	}
}

// newKapookService wires a KapookService with all-happy-path fakes, letting
// each test override just the piece it cares about.
func newKapookService(terms *fakeTermsRepo, goals *fakeGoalRepo, salakSvc *fakeSalakService, accounts *fakeAccountService) *service.KapookService {
	return service.NewKapookService(terms, goals, salakSvc, accounts)
}

// --- Accept / HasAccepted ---------------------------------------------------

func TestKapookService_Accept(t *testing.T) {
	userID := uuid.New()

	t.Run("success records acceptance", func(t *testing.T) {
		repo := newFakeTermsRepo()
		svc := newKapookService(repo, newFakeGoalRepo(), &fakeSalakService{}, &fakeAccountService{})

		err := svc.Accept(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, userID, repo.lastAcceptedUserID)
		assert.True(t, repo.accepted[userID])
	})

	t.Run("accepting twice is idempotent - no error, repo called both times", func(t *testing.T) {
		repo := newFakeTermsRepo()
		svc := newKapookService(repo, newFakeGoalRepo(), &fakeSalakService{}, &fakeAccountService{})

		require.NoError(t, svc.Accept(context.Background(), userID))
		require.NoError(t, svc.Accept(context.Background(), userID))
		assert.Equal(t, 2, repo.acceptCallCount, "the service itself doesn't short-circuit a repeat accept - idempotency is the repo's job")
	})

	t.Run("repo error returns internal error", func(t *testing.T) {
		repo := newFakeTermsRepo()
		repo.acceptErr = errors.New("db down")
		svc := newKapookService(repo, newFakeGoalRepo(), &fakeSalakService{}, &fakeAccountService{})

		err := svc.Accept(context.Background(), userID)
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

func TestKapookService_HasAccepted(t *testing.T) {
	userID := uuid.New()

	t.Run("false before acceptance", func(t *testing.T) {
		repo := newFakeTermsRepo()
		svc := newKapookService(repo, newFakeGoalRepo(), &fakeSalakService{}, &fakeAccountService{})

		got, err := svc.HasAccepted(context.Background(), userID)
		require.NoError(t, err)
		assert.False(t, got)
		assert.Equal(t, userID, repo.lastHasAcceptedUserID)
	})

	t.Run("true after acceptance", func(t *testing.T) {
		repo := newFakeTermsRepo()
		require.NoError(t, repo.Accept(context.Background(), userID))
		svc := newKapookService(repo, newFakeGoalRepo(), &fakeSalakService{}, &fakeAccountService{})

		got, err := svc.HasAccepted(context.Background(), userID)
		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("repo error returns internal error", func(t *testing.T) {
		repo := newFakeTermsRepo()
		repo.hasAcceptErr = errors.New("db down")
		svc := newKapookService(repo, newFakeGoalRepo(), &fakeSalakService{}, &fakeAccountService{})

		_, err := svc.HasAccepted(context.Background(), userID)
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

// --- CreateGoal --------------------------------------------------------

func TestKapookService_CreateGoal(t *testing.T) {
	userID := uuid.New()
	accountID := uuid.New()
	productID := uuid.New()

	// acceptedTerms wires a fakeTermsRepo that already reports userID as
	// having accepted, since most CreateGoal tests aren't about that gate.
	acceptedTerms := func() *fakeTermsRepo {
		r := newFakeTermsRepo()
		r.accepted[userID] = true
		return r
	}

	t.Run("success creates an active goal with zero saved/converted amounts", func(t *testing.T) {
		product := activeProduct()
		product.ID = productID
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		salakSvc := &fakeSalakService{getProductResult: product}
		goals := newFakeGoalRepo()
		svc := newKapookService(acceptedTerms(), goals, salakSvc, accounts)

		got, err := svc.CreateGoal(context.Background(), userID, accountID, productID, decimal.RequireFromString("10000"))
		require.NoError(t, err)

		assert.Equal(t, accountID, got.AccountID)
		assert.Equal(t, productID, got.ProductID)
		assert.True(t, decimal.RequireFromString("10000").Equal(got.GoalAmount))
		assert.True(t, decimal.Zero.Equal(got.SavingAmount))
		assert.True(t, decimal.Zero.Equal(got.SalakAmount))
		assert.True(t, got.IsActive)
		assert.Nil(t, got.GoalReachedAt)
		assert.Equal(t, userID, accounts.lastUserID)
		assert.Equal(t, accountID, accounts.lastAccountID)
		require.NotNil(t, goals.lastCreated)
		assert.Equal(t, got.ID, goals.lastCreated.ID)
	})

	t.Run("account ownership failure is propagated verbatim", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDErr: apperror.NotFound("account not found")}
		svc := newKapookService(acceptedTerms(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.CreateGoal(context.Background(), userID, accountID, productID, decimal.RequireFromString("10000"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("account not of kapook type is rejected", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: accountdomain.Account{ID: accountID, UserID: userID, Type: accountdomain.TypeSavings}}
		svc := newKapookService(acceptedTerms(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.CreateGoal(context.Background(), userID, accountID, productID, decimal.RequireFromString("10000"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("terms not accepted is rejected", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.CreateGoal(context.Background(), userID, accountID, productID, decimal.RequireFromString("10000"))
		assertAppErrKind(t, err, apperror.KindForbidden)
	})

	t.Run("terms check repo error returns internal error", func(t *testing.T) {
		terms := newFakeTermsRepo()
		terms.hasAcceptErr = errors.New("db down")
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		svc := newKapookService(terms, newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.CreateGoal(context.Background(), userID, accountID, productID, decimal.RequireFromString("10000"))
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("an existing active goal is rejected", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		goals := newFakeGoalRepo()
		goals.activeByAccount[accountID] = kapookdomain.Goal{ID: uuid.New(), AccountID: accountID, IsActive: true}
		svc := newKapookService(acceptedTerms(), goals, &fakeSalakService{}, accounts)

		_, err := svc.CreateGoal(context.Background(), userID, accountID, productID, decimal.RequireFromString("10000"))
		assertAppErrKind(t, err, apperror.KindConflict)
	})

	t.Run("active-goal lookup error (other than not-found) returns internal error", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		goals := newFakeGoalRepo()
		goals.findErr = errors.New("db down")
		svc := newKapookService(acceptedTerms(), goals, &fakeSalakService{}, accounts)

		_, err := svc.CreateGoal(context.Background(), userID, accountID, productID, decimal.RequireFromString("10000"))
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("product lookup failure is propagated verbatim", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		salakSvc := &fakeSalakService{getProductErr: apperror.NotFound("salak product not found")}
		svc := newKapookService(acceptedTerms(), newFakeGoalRepo(), salakSvc, accounts)

		_, err := svc.CreateGoal(context.Background(), userID, accountID, productID, decimal.RequireFromString("10000"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("goal amount invalid for the product is rejected", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		salakSvc := &fakeSalakService{
			getProductResult:    activeProduct(),
			validatePurchaseErr: apperror.Validation("amount must be a multiple of the step amount"),
		}
		svc := newKapookService(acceptedTerms(), newFakeGoalRepo(), salakSvc, accounts)

		_, err := svc.CreateGoal(context.Background(), userID, accountID, productID, decimal.RequireFromString("1500"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("repo create failure returns internal error", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		salakSvc := &fakeSalakService{getProductResult: activeProduct()}
		goals := newFakeGoalRepo()
		goals.createErr = errors.New("write failed")
		svc := newKapookService(acceptedTerms(), goals, salakSvc, accounts)

		_, err := svc.CreateGoal(context.Background(), userID, accountID, productID, decimal.RequireFromString("10000"))
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	// A concurrent double-create races past the FindActiveByAccountID
	// pre-check above (both callers see "no active goal"), so it's the
	// partial unique index - not the pre-check - that's the actual
	// race-safe authority. This proves the resulting unique-violation
	// still surfaces as the same Conflict a sequential second attempt
	// gets, not a 500.
	t.Run("repo create failure from a raced unique violation returns conflict, not internal", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		salakSvc := &fakeSalakService{getProductResult: activeProduct()}
		goals := newFakeGoalRepo()
		goals.createErr = &pgconn.PgError{Code: "23505", ConstraintName: "idx_kapook_goals_account_active"}
		svc := newKapookService(acceptedTerms(), goals, salakSvc, accounts)

		_, err := svc.CreateGoal(context.Background(), userID, accountID, productID, decimal.RequireFromString("10000"))
		assertAppErrKind(t, err, apperror.KindConflict)
	})
}

// --- GetActiveGoal --------------------------------------------------------

func TestKapookService_GetActiveGoal(t *testing.T) {
	userID := uuid.New()
	accountID := uuid.New()

	t.Run("success returns the active goal", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		goals := newFakeGoalRepo()
		want := kapookdomain.Goal{ID: uuid.New(), AccountID: accountID, IsActive: true}
		goals.activeByAccount[accountID] = want
		svc := newKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts)

		got, err := svc.GetActiveGoal(context.Background(), userID, accountID)
		require.NoError(t, err)
		assert.Equal(t, want.ID, got.ID)
	})

	t.Run("account ownership failure is propagated verbatim", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDErr: apperror.NotFound("account not found")}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.GetActiveGoal(context.Background(), userID, accountID)
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("account not of kapook type is rejected", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: accountdomain.Account{ID: accountID, UserID: userID, Type: accountdomain.TypeSavings}}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.GetActiveGoal(context.Background(), userID, accountID)
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("no active goal returns not found", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.GetActiveGoal(context.Background(), userID, accountID)
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("repo error (other than not-found) returns internal error", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		goals := newFakeGoalRepo()
		goals.findErr = errors.New("db down")
		svc := newKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts)

		_, err := svc.GetActiveGoal(context.Background(), userID, accountID)
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}
