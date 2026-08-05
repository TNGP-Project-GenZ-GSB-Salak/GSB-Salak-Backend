package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/service"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction"
	txdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
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
	activeByAccount          map[uuid.UUID]kapookdomain.Goal
	byID                     map[uuid.UUID]kapookdomain.Goal
	findByIDErr              error
	findErr                  error
	createErr                error
	findForUpdateErr         error
	updateSavingErr          error
	updateAfterPurchaseErr   error
	updateAfterWithdrawalErr error
	markGoalReachedErr       error
	claimDueGoalsResult      []kapookdomain.Goal
	claimDueGoalsErr         error
	setDeferralErr           error

	lastCreated                 *kapookdomain.Goal
	lastDeferralGoalID          uuid.UUID
	lastDeferralUntil           time.Time
	setDeferralCalls            int
	lastUpdatedGoalID           uuid.UUID
	lastNewSaving               decimal.Decimal
	lastNewSalakAmount          decimal.Decimal
	lastStillActive             bool
	updateAfterPurchaseCalled   bool
	updateAfterWithdrawalCalled bool
	lastMarkGoalReachedID       uuid.UUID
	lastMarkGoalReachedAt       time.Time

	updateAfterExpirationCalled  bool
	lastExpirationGoalID         uuid.UUID
	lastExpirationNewSalakAmount decimal.Decimal
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

func (f *fakeGoalRepo) FindByID(ctx context.Context, goalID uuid.UUID) (kapookdomain.Goal, error) {
	if f.findByIDErr != nil {
		return kapookdomain.Goal{}, f.findByIDErr
	}
	g, ok := f.byID[goalID]
	if !ok {
		return kapookdomain.Goal{}, gorm.ErrRecordNotFound
	}
	return g, nil
}

func (f *fakeGoalRepo) FindActiveByAccountIDForUpdate(ctx context.Context, tx *gorm.DB, accountID uuid.UUID) (kapookdomain.Goal, error) {
	if f.findForUpdateErr != nil {
		return kapookdomain.Goal{}, f.findForUpdateErr
	}
	g, ok := f.activeByAccount[accountID]
	if !ok {
		return kapookdomain.Goal{}, gorm.ErrRecordNotFound
	}
	return g, nil
}

func (f *fakeGoalRepo) UpdateSavingAmount(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSavingAmount decimal.Decimal) error {
	f.lastUpdatedGoalID = goalID
	f.lastNewSaving = newSavingAmount
	if f.updateSavingErr != nil {
		return f.updateSavingErr
	}
	for accID, g := range f.activeByAccount {
		if g.ID == goalID {
			g.SavingAmount = newSavingAmount
			f.activeByAccount[accID] = g
		}
	}
	return nil
}

func (f *fakeGoalRepo) UpdateAfterPurchase(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSalakAmount decimal.Decimal, stillActive bool) error {
	f.updateAfterPurchaseCalled = true
	f.lastUpdatedGoalID = goalID
	f.lastNewSalakAmount = newSalakAmount
	f.lastStillActive = stillActive
	if f.updateAfterPurchaseErr != nil {
		return f.updateAfterPurchaseErr
	}
	for accID, g := range f.activeByAccount {
		if g.ID == goalID {
			g.SalakAmount = newSalakAmount
			g.IsActive = stillActive
			f.activeByAccount[accID] = g
		}
	}
	return nil
}

func (f *fakeGoalRepo) UpdateAfterWithdrawal(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSavingAmount decimal.Decimal, stillActive bool) error {
	f.updateAfterWithdrawalCalled = true
	f.lastUpdatedGoalID = goalID
	f.lastNewSaving = newSavingAmount
	f.lastStillActive = stillActive
	if f.updateAfterWithdrawalErr != nil {
		return f.updateAfterWithdrawalErr
	}
	for accID, g := range f.activeByAccount {
		if g.ID == goalID {
			g.SavingAmount = newSavingAmount
			g.IsActive = stillActive
			f.activeByAccount[accID] = g
		}
	}
	return nil
}

func (f *fakeGoalRepo) UpdateAfterExpiration(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSalakAmount decimal.Decimal) error {
	f.updateAfterExpirationCalled = true
	f.lastExpirationGoalID = goalID
	f.lastExpirationNewSalakAmount = newSalakAmount
	return nil
}

func (f *fakeGoalRepo) MarkGoalReached(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, reachedAt time.Time) error {
	f.lastMarkGoalReachedID = goalID
	f.lastMarkGoalReachedAt = reachedAt
	if f.markGoalReachedErr != nil {
		return f.markGoalReachedErr
	}
	for accID, g := range f.activeByAccount {
		if g.ID == goalID {
			g.GoalReachedAt = &reachedAt
			f.activeByAccount[accID] = g
		}
	}
	return nil
}

func (f *fakeGoalRepo) ClaimDueGoals(ctx context.Context, tx *gorm.DB, cutoff, today time.Time, limit int) ([]kapookdomain.Goal, error) {
	if f.claimDueGoalsErr != nil {
		return nil, f.claimDueGoalsErr
	}
	return f.claimDueGoalsResult, nil
}

func (f *fakeGoalRepo) SetAutoPurchaseDeferral(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, until time.Time) error {
	f.lastDeferralGoalID = goalID
	f.lastDeferralUntil = until
	f.setDeferralCalls++
	return f.setDeferralErr
}

func (f *fakeGoalRepo) RecordAutoPurchaseFailure(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, errMsg string, attemptedAt time.Time) error {
	return nil
}

func (f *fakeGoalRepo) ListStuckGoals(ctx context.Context) ([]kapookdomain.Goal, error) {
	return nil, nil
}

// fakeAccountService is a hand-rolled implementation of account.Service.
// byID/errByID let a test wire up two distinct accounts (e.g. Deposit's
// kapook + savings pair); getByIDResult/getByIDErr are a simpler fallback
// for tests that only care about one account regardless of which id is
// looked up.
type fakeAccountService struct {
	getByIDResult accountdomain.Account
	getByIDErr    error
	byID          map[uuid.UUID]accountdomain.Account
	errByID       map[uuid.UUID]error

	lastUserID, lastAccountID uuid.UUID

	debitResult  decimal.Decimal
	debitErr     error
	creditResult decimal.Decimal
	creditErr    error
	debitCalls   []decimal.Decimal
	creditCalls  []decimal.Decimal

	lockForUpdateErr   error
	lockForUpdateCalls []uuid.UUID

	primaryAccountResult accountdomain.Account
	primaryAccountErr    error
}

func (f *fakeAccountService) ListByUser(ctx context.Context, userID uuid.UUID) ([]accountdomain.Account, error) {
	return nil, nil
}

func (f *fakeAccountService) GetByID(ctx context.Context, userID, accountID uuid.UUID) (accountdomain.Account, error) {
	f.lastUserID, f.lastAccountID = userID, accountID
	if f.errByID != nil {
		if err, ok := f.errByID[accountID]; ok {
			return accountdomain.Account{}, err
		}
	}
	if f.byID != nil {
		if acc, ok := f.byID[accountID]; ok {
			return acc, nil
		}
	}
	if f.getByIDErr != nil {
		return accountdomain.Account{}, f.getByIDErr
	}
	return f.getByIDResult, nil
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

func (f *fakeAccountService) LockForUpdate(ctx context.Context, tx *gorm.DB, accountID uuid.UUID) (accountdomain.Account, error) {
	f.lockForUpdateCalls = append(f.lockForUpdateCalls, accountID)
	if f.lockForUpdateErr != nil {
		return accountdomain.Account{}, f.lockForUpdateErr
	}
	return f.GetByID(ctx, uuid.Nil, accountID)
}

func (f *fakeAccountService) GetByIDUnscoped(ctx context.Context, accountID uuid.UUID) (accountdomain.Account, error) {
	return f.GetByID(ctx, uuid.Nil, accountID)
}

// Create is unused by KapookService - stubbed only to satisfy account.Service.
func (f *fakeAccountService) Create(ctx context.Context, tx *gorm.DB, userID uuid.UUID, accountType accountdomain.Type, startingBalance decimal.Decimal, isPrimary bool) (accountdomain.Account, error) {
	return accountdomain.Account{}, nil
}

// GetPrimaryAccount backs Withdraw's server-side destination resolution.
func (f *fakeAccountService) GetPrimaryAccount(ctx context.Context, userID uuid.UUID) (accountdomain.Account, error) {
	if f.primaryAccountErr != nil {
		return accountdomain.Account{}, f.primaryAccountErr
	}
	return f.primaryAccountResult, nil
}

// fakeSalakService is a hand-rolled implementation of salak.Service, only
// GetProduct/ValidatePurchase are exercised by KapookService.
type fakeSalakService struct {
	getProductResult salakdomain.Product
	getProductErr    error

	validatePurchaseErr error
	ensureNotDrawDayErr error

	nextAvailableDateResult time.Time
	nextAvailableDateErr    error
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

func (f *fakeSalakService) NextAvailableDate(ctx context.Context, product salakdomain.Product) (time.Time, error) {
	return f.nextAvailableDateResult, f.nextAvailableDateErr
}

func (f *fakeSalakService) EnsureNotDrawDay(ctx context.Context, product salakdomain.Product) error {
	return f.ensureNotDrawDayErr
}

func (f *fakeSalakService) MintHolding(ctx context.Context, tx *gorm.DB, accountID, productID uuid.UUID, amount decimal.Decimal) (salakdomain.Holding, error) {
	return salakdomain.Holding{}, nil
}

func (f *fakeSalakService) ListHoldingsByAccount(ctx context.Context, userID, accountID uuid.UUID) ([]salakdomain.Holding, error) {
	return nil, nil
}

func (f *fakeSalakService) FindHoldingForUpdate(ctx context.Context, tx *gorm.DB, id uuid.UUID) (salakdomain.Holding, error) {
	return salakdomain.Holding{}, nil
}

func (f *fakeSalakService) MarkHoldingSettled(ctx context.Context, tx *gorm.DB, id uuid.UUID, settledAt time.Time) error {
	return nil
}

// fakeLedgerRepo is a hand-rolled implementation of transaction.LedgerRepository.
type fakeLedgerRepo struct {
	createErr error
	created   []txdomain.LedgerEntry
}

func (f *fakeLedgerRepo) Create(ctx context.Context, tx *gorm.DB, e *txdomain.LedgerEntry) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, *e)
	return nil
}

func (f *fakeLedgerRepo) FindByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]txdomain.LedgerEntry, error) {
	return nil, nil
}

// fakeTransactionRepo is a hand-rolled implementation of kapook.TransactionRepository.
type fakeTransactionRepo struct {
	createErr   error
	lastCreated *kapookdomain.Transaction
	created     []kapookdomain.Transaction

	countResult     int
	countErr        error
	lastCountGoalID uuid.UUID

	sumUnitsResult int64
	sumCountResult int
	sumErr         error
	lastSumGoalID  uuid.UUID

	listByGoalResult []kapookdomain.Transaction
	listByGoalErr    error
	lastListGoalID   uuid.UUID
	lastListLimit    int
	lastListOffset   int
}

func (f *fakeTransactionRepo) Create(ctx context.Context, tx *gorm.DB, t *kapookdomain.Transaction) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.lastCreated = t
	f.created = append(f.created, *t)
	return nil
}

func (f *fakeTransactionRepo) FindByHoldingID(ctx context.Context, holdingID uuid.UUID) (*kapookdomain.Transaction, error) {
	for i := range f.created {
		if f.created[i].HoldingID != nil && *f.created[i].HoldingID == holdingID {
			return &f.created[i], nil
		}
	}
	return nil, nil
}

func (f *fakeTransactionRepo) CountByGoalAndTypesInWindow(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, types []kapookdomain.TransactionType, from, to time.Time) (int, error) {
	f.lastCountGoalID = goalID
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.countResult, nil
}

func (f *fakeTransactionRepo) ListByGoal(ctx context.Context, goalID uuid.UUID, limit, offset int) ([]kapookdomain.Transaction, error) {
	f.lastListGoalID = goalID
	f.lastListLimit = limit
	f.lastListOffset = offset
	if f.listByGoalErr != nil {
		return nil, f.listByGoalErr
	}
	return f.listByGoalResult, nil
}

func (f *fakeTransactionRepo) SumPurchasedUnitsAndCount(ctx context.Context, tx *gorm.DB, goalID uuid.UUID) (int64, int, error) {
	f.lastSumGoalID = goalID
	if f.sumErr != nil {
		return 0, 0, f.sumErr
	}
	return f.sumUnitsResult, f.sumCountResult, nil
}

// fakeBuySalakService is a hand-rolled implementation of transaction.Service,
// only BuySalakForKapook is exercised by KapookService.BuyFromGoal.
type fakeBuySalakService struct {
	buySalakForKapookResult transaction.BuySalakReceipt
	buySalakForKapookErr    error

	lastKapookAccountID uuid.UUID
	lastSalakAccountID  uuid.UUID
	lastAmount          decimal.Decimal
	callCount           int

	settleResult transaction.SettlementReceipt
	settleErr    error
}

func (f *fakeBuySalakService) BuySalak(ctx context.Context, userID, fundingAccountID, salakAccountID, productID uuid.UUID, badgeID *uuid.UUID, amount decimal.Decimal) (transaction.BuySalakReceipt, error) {
	return transaction.BuySalakReceipt{}, nil
}

func (f *fakeBuySalakService) BuySalakForKapook(ctx context.Context, tx *gorm.DB, userID, kapookAccountID, salakAccountID, productID uuid.UUID, amount decimal.Decimal) (transaction.BuySalakReceipt, error) {
	f.callCount++
	f.lastKapookAccountID = kapookAccountID
	f.lastSalakAccountID = salakAccountID
	f.lastAmount = amount
	if f.buySalakForKapookErr != nil {
		return transaction.BuySalakReceipt{}, f.buySalakForKapookErr
	}
	return f.buySalakForKapookResult, nil
}

func (f *fakeBuySalakService) ListHistory(ctx context.Context, userID, accountID uuid.UUID, limit, offset int) ([]txdomain.LedgerEntry, error) {
	return nil, nil
}

func (f *fakeBuySalakService) SettleMaturedHolding(ctx context.Context, holdingID uuid.UUID) (transaction.SettlementReceipt, error) {
	return f.SettleMaturedHoldingInTx(ctx, nil, holdingID)
}

func (f *fakeBuySalakService) SettleMaturedHoldingInTx(ctx context.Context, tx *gorm.DB, holdingID uuid.UUID) (transaction.SettlementReceipt, error) {
	if f.settleErr != nil {
		return transaction.SettlementReceipt{}, f.settleErr
	}
	return f.settleResult, nil
}

// --- helpers ---------------------------------------------------------------

func assertAppErrKind(t *testing.T, err error, kind apperror.Kind) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, kind, appErr.Kind)
}

func assertAppErrCode(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, code, appErr.Code)
}

func kapookAccount(id uuid.UUID, userID uuid.UUID) accountdomain.Account {
	return accountdomain.Account{ID: id, UserID: userID, Type: accountdomain.TypeKapook}
}

func savingsAccount(id uuid.UUID, userID uuid.UUID) accountdomain.Account {
	return accountdomain.Account{ID: id, UserID: userID, Type: accountdomain.TypeSavings}
}

// newSQLMockDB backs a real *gorm.DB with sqlmock so Deposit's own
// s.db.Transaction(...) call can actually Begin/Commit/Rollback without a
// live Postgres connection - every cross-domain call inside it goes to a
// fake, not a real repo, so no query expectations beyond Begin/Commit(/
// Rollback) are ever needed.
func newSQLMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return gdb, mock
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
// each test override just the piece it cares about. The clock defaults to
// clock.Real{} since most tests here don't exercise time-dependent
// behaviour; withdrawal tests that need a fixed "now" use
// newKapookServiceWithClock instead.
func newKapookService(terms *fakeTermsRepo, goals *fakeGoalRepo, salakSvc *fakeSalakService, accounts *fakeAccountService) *service.KapookService {
	return newKapookServiceWithClock(terms, goals, salakSvc, accounts, clock.Real{})
}

// defaultTestCountdownDuration mirrors the real 24h default - tests that
// care about a specific countdown length use
// newKapookServiceWithClockAndCountdown instead.
const defaultTestCountdownDuration = 24 * time.Hour

func newKapookServiceWithClock(terms *fakeTermsRepo, goals *fakeGoalRepo, salakSvc *fakeSalakService, accounts *fakeAccountService, clk clock.Clock) *service.KapookService {
	return newKapookServiceWithClockAndCountdown(terms, goals, salakSvc, accounts, clk, defaultTestCountdownDuration)
}

func newKapookServiceWithClockAndCountdown(terms *fakeTermsRepo, goals *fakeGoalRepo, salakSvc *fakeSalakService, accounts *fakeAccountService, clk clock.Clock, countdownDuration time.Duration) *service.KapookService {
	return service.NewKapookService(terms, goals, salakSvc, accounts, nil, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clk, &fakeBuySalakService{}, countdownDuration)
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
		assertAppErrCode(t, err, kapook.CodeTermsNotAccepted)
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
		assertAppErrCode(t, err, kapook.CodeGoalAlreadyExists)
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
		assertAppErrCode(t, err, kapook.CodeGoalAlreadyExists)
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

	t.Run("no active goal returns (nil, nil) - a normal empty state, not an error", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDResult: kapookAccount(accountID, userID)}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		got, err := svc.GetActiveGoal(context.Background(), userID, accountID)
		require.NoError(t, err)
		assert.Nil(t, got)
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

// --- Snapshot --------------------------------------------------------

func TestKapookService_Snapshot(t *testing.T) {
	product := activeProduct() // min=1000, per fakeSalakService's default fixture

	t.Run("available balance is saving minus salak, not saving alone", func(t *testing.T) {
		goal := kapookdomain.Goal{
			ID: uuid.New(), ProductID: product.ID,
			SavingAmount: decimal.RequireFromString("3000"), SalakAmount: decimal.RequireFromString("2000"),
		}
		salakSvc := &fakeSalakService{getProductResult: product}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), salakSvc, &fakeAccountService{})

		snap, err := svc.Snapshot(context.Background(), goal)
		require.NoError(t, err)
		assert.True(t, decimal.RequireFromString("1000").Equal(snap.AvailableBalance))
	})

	t.Run("target not reached: no countdown, target_reached false", func(t *testing.T) {
		goal := kapookdomain.Goal{ID: uuid.New(), ProductID: product.ID, GoalReachedAt: nil}
		salakSvc := &fakeSalakService{getProductResult: product}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), salakSvc, &fakeAccountService{})

		snap, err := svc.Snapshot(context.Background(), goal)
		require.NoError(t, err)
		assert.False(t, snap.TargetReached)
		assert.Nil(t, snap.CountdownRemainingSeconds)
	})

	t.Run("target reached: countdown remaining seconds computed from the goal's clock and duration", func(t *testing.T) {
		reachedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		now := reachedAt.Add(10 * time.Hour)
		goal := kapookdomain.Goal{ID: uuid.New(), ProductID: product.ID, GoalReachedAt: &reachedAt}
		salakSvc := &fakeSalakService{getProductResult: product}
		svc := newKapookServiceWithClockAndCountdown(newFakeTermsRepo(), newFakeGoalRepo(), salakSvc, &fakeAccountService{}, clock.Fixed(now), 24*time.Hour)

		snap, err := svc.Snapshot(context.Background(), goal)
		require.NoError(t, err)
		assert.True(t, snap.TargetReached)
		require.NotNil(t, snap.CountdownRemainingSeconds)
		assert.Equal(t, int((14 * time.Hour).Seconds()), *snap.CountdownRemainingSeconds)
	})

	t.Run("countdown already expired clamps to zero, never negative", func(t *testing.T) {
		reachedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		now := reachedAt.Add(30 * time.Hour) // 6h past a 24h countdown
		goal := kapookdomain.Goal{ID: uuid.New(), ProductID: product.ID, GoalReachedAt: &reachedAt}
		salakSvc := &fakeSalakService{getProductResult: product}
		svc := newKapookServiceWithClockAndCountdown(newFakeTermsRepo(), newFakeGoalRepo(), salakSvc, &fakeAccountService{}, clock.Fixed(now), 24*time.Hour)

		snap, err := svc.Snapshot(context.Background(), goal)
		require.NoError(t, err)
		require.NotNil(t, snap.CountdownRemainingSeconds)
		assert.Equal(t, 0, *snap.CountdownRemainingSeconds)
	})

	t.Run("purchased units and count come from the transaction repository, not the goal", func(t *testing.T) {
		goal := kapookdomain.Goal{ID: uuid.New(), ProductID: product.ID}
		salakSvc := &fakeSalakService{getProductResult: product}
		transactions := &fakeTransactionRepo{sumUnitsResult: 42, sumCountResult: 3}
		svc := service.NewKapookService(newFakeTermsRepo(), newFakeGoalRepo(), salakSvc, &fakeAccountService{}, nil, &fakeLedgerRepo{}, transactions, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		snap, err := svc.Snapshot(context.Background(), goal)
		require.NoError(t, err)
		assert.Equal(t, int64(42), snap.PurchasedUnits)
		assert.Equal(t, 3, snap.PurchasedCount)
		assert.Equal(t, goal.ID, transactions.lastSumGoalID)
	})

	t.Run("buy eligible when available balance meets the product minimum", func(t *testing.T) {
		goal := kapookdomain.Goal{ID: uuid.New(), ProductID: product.ID, SavingAmount: decimal.RequireFromString("1000")}
		salakSvc := &fakeSalakService{getProductResult: product}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), salakSvc, &fakeAccountService{})

		snap, err := svc.Snapshot(context.Background(), goal)
		require.NoError(t, err)
		assert.True(t, snap.BuyEligible)
	})

	t.Run("not buy eligible when available balance is below the product minimum", func(t *testing.T) {
		goal := kapookdomain.Goal{ID: uuid.New(), ProductID: product.ID, SavingAmount: decimal.RequireFromString("999")}
		salakSvc := &fakeSalakService{getProductResult: product}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), salakSvc, &fakeAccountService{})

		snap, err := svc.Snapshot(context.Background(), goal)
		require.NoError(t, err)
		assert.False(t, snap.BuyEligible)
	})

	t.Run("product lookup failure is propagated verbatim", func(t *testing.T) {
		goal := kapookdomain.Goal{ID: uuid.New(), ProductID: product.ID}
		salakSvc := &fakeSalakService{getProductErr: apperror.NotFound("salak product not found")}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), salakSvc, &fakeAccountService{})

		_, err := svc.Snapshot(context.Background(), goal)
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("purchase-history aggregation failure returns internal error", func(t *testing.T) {
		goal := kapookdomain.Goal{ID: uuid.New(), ProductID: product.ID}
		salakSvc := &fakeSalakService{getProductResult: product}
		transactions := &fakeTransactionRepo{sumErr: errors.New("db down")}
		svc := service.NewKapookService(newFakeTermsRepo(), newFakeGoalRepo(), salakSvc, &fakeAccountService{}, nil, &fakeLedgerRepo{}, transactions, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.Snapshot(context.Background(), goal)
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

// --- Deposit --------------------------------------------------------

func TestKapookService_Deposit(t *testing.T) {
	userID := uuid.New()

	t.Run("success debits savings, credits kapook, and bumps the goal's saving amount", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := kapookdomain.Goal{
			ID:           uuid.New(),
			AccountID:    kapookAccID,
			IsActive:     true,
			GoalAmount:   decimal.RequireFromString("10000"),
			SavingAmount: decimal.RequireFromString("2000"),
		}
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID:  kapookAccount(kapookAccID, userID),
				savingsAccID: savingsAccount(savingsAccID, userID),
			},
			debitResult:  decimal.RequireFromString("8000"),
			creditResult: decimal.RequireFromString("2500"),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		ledger := &fakeLedgerRepo{}
		transactions := &fakeTransactionRepo{}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, ledger, transactions, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		got, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.RequireFromString("500"))
		require.NoError(t, err)

		assert.True(t, decimal.RequireFromString("2500").Equal(got.SavingAmount))
		require.Len(t, accounts.debitCalls, 1)
		assert.True(t, decimal.RequireFromString("500").Equal(accounts.debitCalls[0]))
		require.Len(t, accounts.creditCalls, 1)
		assert.True(t, decimal.RequireFromString("500").Equal(accounts.creditCalls[0]))

		require.Len(t, ledger.created, 2)
		debitEntry, creditEntry := ledger.created[0], ledger.created[1]
		assert.Equal(t, txdomain.EntryDebit, debitEntry.Type)
		assert.Equal(t, savingsAccID, debitEntry.AccountID)
		assert.Equal(t, txdomain.EntryCredit, creditEntry.Type)
		assert.Equal(t, kapookAccID, creditEntry.AccountID)
		assert.Equal(t, debitEntry.ReferenceID, creditEntry.ReferenceID, "debit/credit must share one reference_id")
		assert.Equal(t, "kapook_transaction", debitEntry.ReferenceType)
		assert.Equal(t, "kapook_transaction", creditEntry.ReferenceType)

		require.NotNil(t, transactions.lastCreated)
		assert.Equal(t, kapookdomain.TransactionDeposit, transactions.lastCreated.Type)
		assert.Equal(t, debitEntry.ReferenceID, transactions.lastCreated.ID, "the kapook_transaction's own id is the ledger pair's shared reference_id")
		require.NotNil(t, transactions.lastCreated.SavingsAccountID)
		assert.Equal(t, savingsAccID, *transactions.lastCreated.SavingsAccountID)
		assert.Equal(t, kapookAccID, transactions.lastCreated.KapookAccountID)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("kapook and savings account must differ", func(t *testing.T) {
		sameID := uuid.New()
		accounts := &fakeAccountService{getByIDResult: kapookAccount(sameID, userID)}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.Deposit(context.Background(), userID, sameID, sameID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("kapook account ownership failure is propagated verbatim", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDErr: apperror.NotFound("account not found")}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.Deposit(context.Background(), userID, uuid.New(), uuid.New(), decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("kapook account not of kapook type is rejected", func(t *testing.T) {
		kapookAccID := uuid.New()
		accounts := &fakeAccountService{getByIDResult: accountdomain.Account{ID: kapookAccID, UserID: userID, Type: accountdomain.TypeSavings}}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.Deposit(context.Background(), userID, kapookAccID, uuid.New(), decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("savings account ownership failure is propagated verbatim", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		accounts := &fakeAccountService{
			byID:    map[uuid.UUID]accountdomain.Account{kapookAccID: kapookAccount(kapookAccID, userID)},
			errByID: map[uuid.UUID]error{savingsAccID: apperror.NotFound("account not found")},
		}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("savings account not of savings type is rejected", func(t *testing.T) {
		kapookAccID, otherAccID := uuid.New(), uuid.New()
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			otherAccID:  {ID: otherAccID, UserID: userID, Type: accountdomain.TypeSalak},
		}}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.Deposit(context.Background(), userID, kapookAccID, otherAccID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("zero amount is rejected before any transaction opens", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID:  kapookAccount(kapookAccID, userID),
			savingsAccID: savingsAccount(savingsAccID, userID),
		}}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.Zero)
		assertAppErrKind(t, err, apperror.KindValidation)
		assertAppErrCode(t, err, kapook.CodeAmountMustBePositive)
	})

	t.Run("negative amount is rejected", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID:  kapookAccount(kapookAccID, userID),
			savingsAccID: savingsAccount(savingsAccID, userID),
		}}
		svc := newKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts)

		_, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.RequireFromString("-500"))
		assertAppErrKind(t, err, apperror.KindValidation)
		assertAppErrCode(t, err, kapook.CodeAmountMustBePositive)
	})

	t.Run("no active goal returns not found", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID:  kapookAccount(kapookAccID, userID),
			savingsAccID: savingsAccount(savingsAccID, userID),
		}}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("deposit that would exceed the target is rejected, before any debit", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := kapookdomain.Goal{
			ID: uuid.New(), AccountID: kapookAccID, IsActive: true,
			GoalAmount: decimal.RequireFromString("1000"), SavingAmount: decimal.RequireFromString("800"),
		}
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID:  kapookAccount(kapookAccID, userID),
			savingsAccID: savingsAccount(savingsAccID, userID),
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.RequireFromString("300"))
		assertAppErrKind(t, err, apperror.KindValidation)
		assertAppErrCode(t, err, kapook.CodeDepositExceedsTarget)
		assert.Empty(t, accounts.debitCalls, "must reject before debiting anything")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("deposit reaching exactly the target is allowed", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := kapookdomain.Goal{
			ID: uuid.New(), AccountID: kapookAccID, IsActive: true,
			GoalAmount: decimal.RequireFromString("1000"), SavingAmount: decimal.RequireFromString("800"),
		}
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID:  kapookAccount(kapookAccID, userID),
			savingsAccID: savingsAccount(savingsAccID, userID),
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		got, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.RequireFromString("200"))
		require.NoError(t, err)
		assert.True(t, decimal.RequireFromString("1000").Equal(got.SavingAmount))
		require.NotNil(t, got.GoalReachedAt, "crossing the target must start the countdown")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("a deposit that doesn't reach the target leaves GoalReachedAt nil", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := kapookdomain.Goal{
			ID: uuid.New(), AccountID: kapookAccID, IsActive: true,
			GoalAmount: decimal.RequireFromString("1000"), SavingAmount: decimal.RequireFromString("500"),
		}
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID:  kapookAccount(kapookAccID, userID),
			savingsAccID: savingsAccount(savingsAccID, userID),
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		got, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.RequireFromString("200"))
		require.NoError(t, err)
		assert.Nil(t, got.GoalReachedAt)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("depositing up to the target again when it's already marked reached does not call MarkGoalReached a second time", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		originalReachedAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		goal := kapookdomain.Goal{
			ID: uuid.New(), AccountID: kapookAccID, IsActive: true,
			GoalAmount: decimal.RequireFromString("1000"), SavingAmount: decimal.RequireFromString("800"),
			GoalReachedAt: &originalReachedAt,
		}
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID:  kapookAccount(kapookAccID, userID),
			savingsAccID: savingsAccount(savingsAccID, userID),
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		got, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.RequireFromString("200"))
		require.NoError(t, err)
		require.NotNil(t, got.GoalReachedAt)
		assert.True(t, originalReachedAt.Equal(*got.GoalReachedAt), "must stay the original instant, not be pushed forward")
		assert.Equal(t, uuid.Nil, goals.lastMarkGoalReachedID, "MarkGoalReached must not run again once GoalReachedAt is already set")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("debit failure rolls back and is propagated verbatim", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := kapookdomain.Goal{ID: uuid.New(), AccountID: kapookAccID, IsActive: true, GoalAmount: decimal.RequireFromString("10000"), SavingAmount: decimal.Zero}
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID:  kapookAccount(kapookAccID, userID),
				savingsAccID: savingsAccount(savingsAccID, userID),
			},
			debitErr: apperror.Validation("insufficient funds"),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindValidation)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ledger write failure rolls back the whole transaction", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := kapookdomain.Goal{ID: uuid.New(), AccountID: kapookAccID, IsActive: true, GoalAmount: decimal.RequireFromString("10000"), SavingAmount: decimal.Zero}
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID:  kapookAccount(kapookAccID, userID),
			savingsAccID: savingsAccount(savingsAccID, userID),
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		ledger := &fakeLedgerRepo{createErr: errors.New("write failed")}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, ledger, &fakeTransactionRepo{}, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindInternal)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("kapook transaction write failure rolls back the whole transaction", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := kapookdomain.Goal{ID: uuid.New(), AccountID: kapookAccID, IsActive: true, GoalAmount: decimal.RequireFromString("10000"), SavingAmount: decimal.Zero}
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID:  kapookAccount(kapookAccID, userID),
			savingsAccID: savingsAccount(savingsAccID, userID),
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		transactions := &fakeTransactionRepo{createErr: errors.New("write failed")}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, transactions, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.Deposit(context.Background(), userID, kapookAccID, savingsAccID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindInternal)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- Withdraw --------------------------------------------------------

func TestKapookService_Withdraw(t *testing.T) {
	userID := uuid.New()
	anchor := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fixedClk := clock.Fixed(anchor.AddDate(0, 1, 0)) // one month into the goal's first window

	newGoal := func(kapookAccID uuid.UUID, saving decimal.Decimal) kapookdomain.Goal {
		return kapookdomain.Goal{
			ID:           uuid.New(),
			AccountID:    kapookAccID,
			IsActive:     true,
			GoalAmount:   decimal.RequireFromString("10000"),
			SavingAmount: saving,
			CreatedAt:    anchor,
		}
	}

	t.Run("first withdrawal in the window is free", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := newGoal(kapookAccID, decimal.RequireFromString("2000"))
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
			debitResult:          decimal.RequireFromString("1500"),
			creditResult:         decimal.RequireFromString("9500"),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		ledger := &fakeLedgerRepo{}
		transactions := &fakeTransactionRepo{countResult: 0}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, ledger, transactions, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		result, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("500"))
		require.NoError(t, err)

		assert.False(t, result.FeeCharged)
		assert.True(t, decimal.Zero.Equal(result.FeeAmount))
		assert.True(t, decimal.RequireFromString("500").Equal(result.NetCredited))
		assert.True(t, decimal.RequireFromString("1500").Equal(result.Goal.SavingAmount))
		assert.True(t, result.Goal.IsActive, "the goal survives a withdrawal")

		require.Len(t, accounts.debitCalls, 1)
		assert.True(t, decimal.RequireFromString("500").Equal(accounts.debitCalls[0]), "kapook is debited the full pre-fee amount")
		require.Len(t, accounts.creditCalls, 1)
		assert.True(t, decimal.RequireFromString("500").Equal(accounts.creditCalls[0]), "savings is credited the same amount when free")

		require.NotEmpty(t, accounts.lockForUpdateCalls)
		assert.Equal(t, kapookAccID, accounts.lockForUpdateCalls[0], "kapook must be locked first, same order as Deposit")

		require.Len(t, ledger.created, 2)
		debitEntry, creditEntry := ledger.created[0], ledger.created[1]
		assert.Equal(t, txdomain.EntryDebit, debitEntry.Type)
		assert.Equal(t, kapookAccID, debitEntry.AccountID)
		assert.Equal(t, txdomain.EntryCredit, creditEntry.Type)
		assert.Equal(t, savingsAccID, creditEntry.AccountID, "the goal's savings leg resolves to the primary account, never a caller-supplied id")
		assert.Equal(t, debitEntry.ReferenceID, creditEntry.ReferenceID)

		require.NotNil(t, transactions.lastCreated)
		assert.Equal(t, kapookdomain.TransactionWithdraw, transactions.lastCreated.Type)
		assert.Equal(t, goal.ID, transactions.lastCreated.GoalID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("third withdrawal in the window incurs the 2% fee, taken out of what reaches savings", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := newGoal(kapookAccID, decimal.RequireFromString("2000"))
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		ledger := &fakeLedgerRepo{}
		transactions := &fakeTransactionRepo{countResult: 2} // two free withdrawals already used

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, ledger, transactions, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		result, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("1000"))
		require.NoError(t, err)

		assert.True(t, result.FeeCharged)
		assert.True(t, decimal.RequireFromString("20").Equal(result.FeeAmount), "2%% of 1000")
		assert.True(t, decimal.RequireFromString("980").Equal(result.NetCredited))
		assert.True(t, decimal.RequireFromString("1000").Equal(result.Amount), "the recorded amount is always the pre-fee figure")

		require.Len(t, accounts.debitCalls, 1)
		assert.True(t, decimal.RequireFromString("1000").Equal(accounts.debitCalls[0]), "kapook loses the full amount regardless of the fee")
		require.Len(t, accounts.creditCalls, 1)
		assert.True(t, decimal.RequireFromString("980").Equal(accounts.creditCalls[0]), "savings only receives the net-of-fee amount")

		require.NotNil(t, transactions.lastCreated)
		assert.Equal(t, kapookdomain.TransactionWithdrawWithFee, transactions.lastCreated.Type)
		assert.True(t, decimal.RequireFromString("1000").Equal(transactions.lastCreated.Amount), "stored amount is pre-fee, never the fee itself")

		require.Len(t, ledger.created, 2)
		debitEntry, creditEntry := ledger.created[0], ledger.created[1]
		assert.True(t, decimal.RequireFromString("1000").Equal(debitEntry.Amount))
		assert.True(t, decimal.RequireFromString("980").Equal(creditEntry.Amount))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("a satang-precision withdrawal fee is rounded to two decimal places", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := newGoal(kapookAccID, decimal.RequireFromString("2000"))
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		ledger := &fakeLedgerRepo{}
		transactions := &fakeTransactionRepo{countResult: 2} // two free withdrawals already used, this one is fee-charged

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, ledger, transactions, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		// 1000.01 * 0.02 = 20.0002 unrounded - the exact bug this ticket fixes.
		result, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("1000.01"))
		require.NoError(t, err)

		assert.True(t, decimal.RequireFromString("20.00").Equal(result.FeeAmount), "20.0002 rounds to 20.00, matching the database's numeric(18,2) column")
		assert.True(t, decimal.RequireFromString("980.01").Equal(result.NetCredited), "1000.01 - 20.00, not 1000.01 - 20.0002")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("kapook account ownership failure is propagated verbatim", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDErr: apperror.NotFound("account not found")}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, fixedClk)

		_, err := svc.Withdraw(context.Background(), userID, uuid.New(), decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("kapook account not of kapook type is rejected", func(t *testing.T) {
		kapookAccID := uuid.New()
		accounts := &fakeAccountService{getByIDResult: accountdomain.Account{ID: kapookAccID, UserID: userID, Type: accountdomain.TypeSavings}}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, fixedClk)

		_, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("no primary account fails loudly with a support-style message and a stable code", func(t *testing.T) {
		kapookAccID := uuid.New()
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountErr: apperror.NotFound("no primary account"),
		}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, fixedClk)

		_, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindNotFound)
		assertAppErrCode(t, err, kapook.CodeNoPrimaryAccount)
	})

	t.Run("zero amount is rejected before any transaction opens", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
		}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, fixedClk)

		_, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.Zero)
		assertAppErrKind(t, err, apperror.KindValidation)
		assertAppErrCode(t, err, kapook.CodeAmountMustBePositive)
	})

	t.Run("no active goal returns not found", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
		}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("amount exceeding the balance is rejected before any debit", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := newGoal(kapookAccID, decimal.RequireFromString("300"))
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("301"))
		assertAppErrKind(t, err, apperror.KindValidation)
		assertAppErrCode(t, err, kapook.CodeWithdrawalExceedsBalance)
		assert.Empty(t, accounts.debitCalls, "must reject before debiting anything")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("balance available to withdraw excludes amount already converted to Salak", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := newGoal(kapookAccID, decimal.RequireFromString("3000"))
		goal.SalakAmount = decimal.RequireFromString("2000") // only 1000 is still cash
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		// 1500 is well within the raw SavingAmount (3000) but exceeds the
		// 1000 actually still sitting as cash once SalakAmount is netted out.
		_, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("1500"))
		assertAppErrKind(t, err, apperror.KindValidation)
		assert.Empty(t, accounts.debitCalls, "must reject before debiting anything")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("withdrawing the entire balance leaves the goal active", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := newGoal(kapookAccID, decimal.RequireFromString("500"))
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		result, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("500"))
		require.NoError(t, err)
		assert.True(t, decimal.Zero.Equal(result.Goal.SavingAmount))
		assert.True(t, result.Goal.IsActive, "emptying the kapook does not close the goal")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("during a live countdown, a partial withdrawal is rejected", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := newGoal(kapookAccID, decimal.RequireFromString("500"))
		reachedAt := anchor.AddDate(0, 0, 15)
		goal.GoalReachedAt = &reachedAt
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("200"))
		assertAppErrKind(t, err, apperror.KindValidation)
		assertAppErrCode(t, err, kapook.CodeWithdrawalMustBeFullDuringCountdown)
		assert.Empty(t, accounts.debitCalls, "must reject before debiting anything")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("during a live countdown, a full withdrawal is allowed and closes the goal", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := newGoal(kapookAccID, decimal.RequireFromString("500"))
		reachedAt := anchor.AddDate(0, 0, 15)
		goal.GoalReachedAt = &reachedAt
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		result, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("500"))
		require.NoError(t, err)
		assert.True(t, result.GoalClosed)
		assert.False(t, result.Goal.IsActive)
		assert.True(t, goals.updateAfterWithdrawalCalled)
		assert.False(t, goals.lastStillActive)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("before the goal is reached, a partial withdrawal never closes it", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := newGoal(kapookAccID, decimal.RequireFromString("500")) // GoalReachedAt is nil
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		result, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("100"))
		require.NoError(t, err)
		assert.False(t, result.GoalClosed)
		assert.True(t, result.Goal.IsActive)
	})

	t.Run("debit failure rolls back and is propagated verbatim", func(t *testing.T) {
		kapookAccID, savingsAccID := uuid.New(), uuid.New()
		goal := newGoal(kapookAccID, decimal.RequireFromString("2000"))
		accounts := &fakeAccountService{
			byID: map[uuid.UUID]accountdomain.Account{
				kapookAccID: kapookAccount(kapookAccID, userID),
			},
			primaryAccountResult: savingsAccount(savingsAccID, userID),
			debitErr:             apperror.Validation("insufficient funds"),
		}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.Withdraw(context.Background(), userID, kapookAccID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindValidation)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- GetWithdrawalStatus ----------------------------------------------

func TestKapookService_GetWithdrawalStatus(t *testing.T) {
	userID := uuid.New()
	anchor := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fixedClk := clock.Fixed(anchor.AddDate(0, 1, 0))

	t.Run("no withdrawals used yet: two remain and the next is free", func(t *testing.T) {
		kapookAccID := uuid.New()
		goal := kapookdomain.Goal{ID: uuid.New(), AccountID: kapookAccID, IsActive: true, CreatedAt: anchor}
		accounts := &fakeAccountService{getByIDResult: kapookAccount(kapookAccID, userID)}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		svc := newKapookServiceWithClock(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, fixedClk)

		status, err := svc.GetWithdrawalStatus(context.Background(), userID, kapookAccID, nil)
		require.NoError(t, err)
		assert.Equal(t, 0, status.FreeWithdrawalsUsed)
		assert.Equal(t, 2, status.FreeWithdrawalsRemaining)
		assert.True(t, status.NextWithdrawalIsFree)
		assert.True(t, anchor.Equal(status.WindowStart))
		assert.True(t, anchor.AddDate(1, 0, 0).Equal(status.WindowEnd))
		assert.Nil(t, status.Quote, "no amount was requested, so no quote is computed")
	})

	t.Run("allowance exhausted: none remain and the next would be charged", func(t *testing.T) {
		kapookAccID := uuid.New()
		goal := kapookdomain.Goal{ID: uuid.New(), AccountID: kapookAccID, IsActive: true, CreatedAt: anchor}
		accounts := &fakeAccountService{getByIDResult: kapookAccount(kapookAccID, userID)}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, nil, &fakeLedgerRepo{}, &fakeTransactionRepo{countResult: 2}, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		status, err := svc.GetWithdrawalStatus(context.Background(), userID, kapookAccID, nil)
		require.NoError(t, err)
		assert.Equal(t, 2, status.FreeWithdrawalsUsed)
		assert.Equal(t, 0, status.FreeWithdrawalsRemaining)
		assert.False(t, status.NextWithdrawalIsFree)
	})

	t.Run("amount quote when the next withdrawal is free reports zero fee and the full net", func(t *testing.T) {
		kapookAccID := uuid.New()
		goal := kapookdomain.Goal{ID: uuid.New(), AccountID: kapookAccID, IsActive: true, CreatedAt: anchor}
		accounts := &fakeAccountService{getByIDResult: kapookAccount(kapookAccID, userID)}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		svc := newKapookServiceWithClock(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, fixedClk)

		amount := decimal.RequireFromString("500")
		status, err := svc.GetWithdrawalStatus(context.Background(), userID, kapookAccID, &amount)
		require.NoError(t, err)
		require.NotNil(t, status.Quote)
		assert.True(t, decimal.Zero.Equal(status.Quote.FeeAmount))
		assert.True(t, decimal.RequireFromString("500").Equal(status.Quote.NetAmount))
	})

	t.Run("amount quote when fee-charged rounds to two decimal places, matching what Withdraw would actually charge", func(t *testing.T) {
		kapookAccID := uuid.New()
		goal := kapookdomain.Goal{ID: uuid.New(), AccountID: kapookAccID, IsActive: true, CreatedAt: anchor}
		accounts := &fakeAccountService{getByIDResult: kapookAccount(kapookAccID, userID)}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, nil, &fakeLedgerRepo{}, &fakeTransactionRepo{countResult: 2}, fixedClk, &fakeBuySalakService{}, defaultTestCountdownDuration)

		amount := decimal.RequireFromString("1000.01")
		status, err := svc.GetWithdrawalStatus(context.Background(), userID, kapookAccID, &amount)
		require.NoError(t, err)
		require.NotNil(t, status.Quote)
		assert.True(t, decimal.RequireFromString("20.00").Equal(status.Quote.FeeAmount))
		assert.True(t, decimal.RequireFromString("980.01").Equal(status.Quote.NetAmount))
	})

	t.Run("no active goal returns not found", func(t *testing.T) {
		kapookAccID := uuid.New()
		accounts := &fakeAccountService{getByIDResult: kapookAccount(kapookAccID, userID)}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, fixedClk)

		_, err := svc.GetWithdrawalStatus(context.Background(), userID, kapookAccID, nil)
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("kapook account ownership failure is propagated verbatim", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDErr: apperror.NotFound("account not found")}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, fixedClk)

		_, err := svc.GetWithdrawalStatus(context.Background(), userID, uuid.New(), nil)
		assertAppErrKind(t, err, apperror.KindNotFound)
	})
}

// --- BuyFromGoal --------------------------------------------------------

func TestKapookService_BuyFromGoal(t *testing.T) {
	userID := uuid.New()

	productFor := func(goalAmount decimal.Decimal) salakdomain.Product {
		return salakdomain.Product{
			ID:          uuid.New(),
			MinPurchase: decimal.RequireFromString("1000"),
			MaxPurchase: goalAmount,
			StepAmount:  decimal.RequireFromString("1000"),
		}
	}

	newGoalWithProduct := func(kapookAccID, productID uuid.UUID, saving, salak decimal.Decimal) kapookdomain.Goal {
		return kapookdomain.Goal{
			ID:           uuid.New(),
			AccountID:    kapookAccID,
			ProductID:    productID,
			IsActive:     true,
			GoalAmount:   decimal.RequireFromString("5000"),
			SavingAmount: saving,
			SalakAmount:  salak,
		}
	}

	t.Run("partial purchase converts amount, leaves the goal active", func(t *testing.T) {
		kapookAccID, salakAccID := uuid.New(), uuid.New()
		product := productFor(decimal.RequireFromString("5000"))
		goal := newGoalWithProduct(kapookAccID, product.ID, decimal.RequireFromString("3000"), decimal.Zero)
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			salakAccID:  {ID: salakAccID, UserID: userID, Type: accountdomain.TypeSalak},
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		salakSvc := &fakeSalakService{getProductResult: product}
		holdingID := uuid.New()
		buySalakSvc := &fakeBuySalakService{buySalakForKapookResult: transaction.BuySalakReceipt{HoldingID: holdingID}}
		transactions := &fakeTransactionRepo{}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, salakSvc, accounts, db, &fakeLedgerRepo{}, transactions, clock.Real{}, buySalakSvc, defaultTestCountdownDuration)

		result, err := svc.BuyFromGoal(context.Background(), userID, kapookAccID, salakAccID, decimal.RequireFromString("2000"))
		require.NoError(t, err)

		assert.False(t, result.GoalCompleted)
		assert.True(t, result.Goal.IsActive)
		assert.True(t, decimal.RequireFromString("2000").Equal(result.Goal.SalakAmount))
		assert.True(t, decimal.RequireFromString("3000").Equal(result.Goal.SavingAmount), "SavingAmount is untouched by a purchase")
		assert.Equal(t, holdingID, result.Receipt.HoldingID)

		assert.Equal(t, 1, buySalakSvc.callCount)
		assert.Equal(t, kapookAccID, buySalakSvc.lastKapookAccountID)
		assert.Equal(t, salakAccID, buySalakSvc.lastSalakAccountID)
		assert.True(t, decimal.RequireFromString("2000").Equal(buySalakSvc.lastAmount))

		require.True(t, goals.updateAfterPurchaseCalled)
		assert.True(t, goals.lastStillActive)
		assert.True(t, decimal.RequireFromString("2000").Equal(goals.lastNewSalakAmount))

		require.NotNil(t, transactions.lastCreated)
		assert.Equal(t, kapookdomain.TransactionBuySalak, transactions.lastCreated.Type)
		assert.Equal(t, goal.ID, transactions.lastCreated.GoalID)
		require.NotNil(t, transactions.lastCreated.HoldingID)
		assert.Equal(t, holdingID, *transactions.lastCreated.HoldingID)
		require.NotNil(t, transactions.lastCreated.IsAutomaticPurchase)
		assert.False(t, *transactions.lastCreated.IsAutomaticPurchase, "a customer-initiated purchase is never marked automatic")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("purchase that fully satisfies the target deactivates the goal", func(t *testing.T) {
		kapookAccID, salakAccID := uuid.New(), uuid.New()
		product := productFor(decimal.RequireFromString("5000"))
		goal := newGoalWithProduct(kapookAccID, product.ID, decimal.RequireFromString("5000"), decimal.RequireFromString("3000"))
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			salakAccID:  {ID: salakAccID, UserID: userID, Type: accountdomain.TypeSalak},
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		salakSvc := &fakeSalakService{getProductResult: product}
		buySalakSvc := &fakeBuySalakService{buySalakForKapookResult: transaction.BuySalakReceipt{HoldingID: uuid.New()}}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, salakSvc, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, buySalakSvc, defaultTestCountdownDuration)

		result, err := svc.BuyFromGoal(context.Background(), userID, kapookAccID, salakAccID, decimal.RequireFromString("2000"))
		require.NoError(t, err)

		assert.True(t, result.GoalCompleted)
		assert.False(t, result.Goal.IsActive)
		assert.True(t, decimal.RequireFromString("5000").Equal(result.Goal.SalakAmount))
		assert.False(t, goals.lastStillActive)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("kapook and salak account must differ", func(t *testing.T) {
		sameID := uuid.New()
		accounts := &fakeAccountService{getByIDResult: kapookAccount(sameID, userID)}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, clock.Real{})

		_, err := svc.BuyFromGoal(context.Background(), userID, sameID, sameID, decimal.RequireFromString("1000"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("kapook account ownership failure is propagated verbatim", func(t *testing.T) {
		accounts := &fakeAccountService{getByIDErr: apperror.NotFound("account not found")}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, clock.Real{})

		_, err := svc.BuyFromGoal(context.Background(), userID, uuid.New(), uuid.New(), decimal.RequireFromString("1000"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("kapook account not of kapook type is rejected", func(t *testing.T) {
		kapookAccID := uuid.New()
		accounts := &fakeAccountService{getByIDResult: accountdomain.Account{ID: kapookAccID, UserID: userID, Type: accountdomain.TypeSavings}}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, clock.Real{})

		_, err := svc.BuyFromGoal(context.Background(), userID, kapookAccID, uuid.New(), decimal.RequireFromString("1000"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("salak account not of salak type is rejected", func(t *testing.T) {
		kapookAccID, otherAccID := uuid.New(), uuid.New()
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			otherAccID:  {ID: otherAccID, UserID: userID, Type: accountdomain.TypeSavings},
		}}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, clock.Real{})

		_, err := svc.BuyFromGoal(context.Background(), userID, kapookAccID, otherAccID, decimal.RequireFromString("1000"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("zero amount is rejected before any transaction opens", func(t *testing.T) {
		kapookAccID, salakAccID := uuid.New(), uuid.New()
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			salakAccID:  {ID: salakAccID, UserID: userID, Type: accountdomain.TypeSalak},
		}}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, clock.Real{})

		_, err := svc.BuyFromGoal(context.Background(), userID, kapookAccID, salakAccID, decimal.Zero)
		assertAppErrKind(t, err, apperror.KindValidation)
		assertAppErrCode(t, err, kapook.CodeAmountMustBePositive)
	})

	t.Run("no active goal returns not found", func(t *testing.T) {
		kapookAccID, salakAccID := uuid.New(), uuid.New()
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			salakAccID:  {ID: salakAccID, UserID: userID, Type: accountdomain.TypeSalak},
		}}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.BuyFromGoal(context.Background(), userID, kapookAccID, salakAccID, decimal.RequireFromString("1000"))
		assertAppErrKind(t, err, apperror.KindNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("balance below the product minimum is rejected before calling BuySalakForKapook", func(t *testing.T) {
		kapookAccID, salakAccID := uuid.New(), uuid.New()
		product := productFor(decimal.RequireFromString("5000"))
		goal := newGoalWithProduct(kapookAccID, product.ID, decimal.RequireFromString("500"), decimal.Zero)
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			salakAccID:  {ID: salakAccID, UserID: userID, Type: accountdomain.TypeSalak},
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		salakSvc := &fakeSalakService{getProductResult: product}
		buySalakSvc := &fakeBuySalakService{}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, salakSvc, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, buySalakSvc, defaultTestCountdownDuration)

		_, err := svc.BuyFromGoal(context.Background(), userID, kapookAccID, salakAccID, decimal.RequireFromString("500"))
		assertAppErrKind(t, err, apperror.KindValidation)
		assertAppErrCode(t, err, kapook.CodeBalanceBelowMinimumPurchase)
		assert.Equal(t, 0, buySalakSvc.callCount)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("amount invalid for the product (ValidatePurchase) is rejected", func(t *testing.T) {
		kapookAccID, salakAccID := uuid.New(), uuid.New()
		product := productFor(decimal.RequireFromString("5000"))
		goal := newGoalWithProduct(kapookAccID, product.ID, decimal.RequireFromString("3000"), decimal.Zero)
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			salakAccID:  {ID: salakAccID, UserID: userID, Type: accountdomain.TypeSalak},
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		salakSvc := &fakeSalakService{getProductResult: product, validatePurchaseErr: apperror.Validation("amount must be a multiple of the step amount")}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, salakSvc, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.BuyFromGoal(context.Background(), userID, kapookAccID, salakAccID, decimal.RequireFromString("1500"))
		assertAppErrKind(t, err, apperror.KindValidation)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("amount exceeding the available balance is rejected", func(t *testing.T) {
		kapookAccID, salakAccID := uuid.New(), uuid.New()
		product := productFor(decimal.RequireFromString("5000"))
		// 3000 saved, 2000 already converted -> only 1000 still available.
		goal := newGoalWithProduct(kapookAccID, product.ID, decimal.RequireFromString("3000"), decimal.RequireFromString("2000"))
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			salakAccID:  {ID: salakAccID, UserID: userID, Type: accountdomain.TypeSalak},
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		salakSvc := &fakeSalakService{getProductResult: product}
		buySalakSvc := &fakeBuySalakService{}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, salakSvc, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, buySalakSvc, defaultTestCountdownDuration)

		_, err := svc.BuyFromGoal(context.Background(), userID, kapookAccID, salakAccID, decimal.RequireFromString("2000"))
		assertAppErrKind(t, err, apperror.KindValidation)
		assertAppErrCode(t, err, kapook.CodeBuyAmountExceedsBalance)
		assert.Equal(t, 0, buySalakSvc.callCount)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("draw-day rejection from BuySalakForKapook rolls back and is propagated verbatim", func(t *testing.T) {
		kapookAccID, salakAccID := uuid.New(), uuid.New()
		product := productFor(decimal.RequireFromString("5000"))
		goal := newGoalWithProduct(kapookAccID, product.ID, decimal.RequireFromString("3000"), decimal.Zero)
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			salakAccID:  {ID: salakAccID, UserID: userID, Type: accountdomain.TypeSalak},
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		salakSvc := &fakeSalakService{getProductResult: product}
		buySalakSvc := &fakeBuySalakService{buySalakForKapookErr: apperror.Validation("salak: product is closed for purchases on this draw day")}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, salakSvc, accounts, db, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, buySalakSvc, defaultTestCountdownDuration)

		_, err := svc.BuyFromGoal(context.Background(), userID, kapookAccID, salakAccID, decimal.RequireFromString("1000"))
		assertAppErrKind(t, err, apperror.KindValidation)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- BuyFromGoalInTx ----------------------------------------------------

// TestKapookService_BuyFromGoalInTx exercises the worker-facing variant
// against the same happy/failure shapes as BuyFromGoal - it shares
// buyFromGoalCore, so this mainly proves the tx-supplied wiring itself
// (validation still runs, the caller's tx is used, errors still surface)
// rather than re-testing every validation branch again.
func TestKapookService_BuyFromGoalInTx(t *testing.T) {
	userID := uuid.New()
	product := salakdomain.Product{
		ID:          uuid.New(),
		MinPurchase: decimal.RequireFromString("1000"),
		MaxPurchase: decimal.RequireFromString("5000"),
		StepAmount:  decimal.RequireFromString("1000"),
	}

	t.Run("success converts amount via the caller's own tx", func(t *testing.T) {
		kapookAccID, salakAccID := uuid.New(), uuid.New()
		goal := kapookdomain.Goal{
			ID: uuid.New(), AccountID: kapookAccID, ProductID: product.ID, IsActive: true,
			GoalAmount: decimal.RequireFromString("5000"), SavingAmount: decimal.RequireFromString("3000"),
		}
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			salakAccID:  {ID: salakAccID, UserID: userID, Type: accountdomain.TypeSalak},
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		salakSvc := &fakeSalakService{getProductResult: product}
		holdingID := uuid.New()
		buySalakSvc := &fakeBuySalakService{buySalakForKapookResult: transaction.BuySalakReceipt{HoldingID: holdingID}}

		transactions := &fakeTransactionRepo{}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin() // the savepoint's own tx.Transaction call - a plain BEGIN here since db isn't itself already nested
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, salakSvc, accounts, nil, &fakeLedgerRepo{}, transactions, clock.Real{}, buySalakSvc, defaultTestCountdownDuration)

		result, err := svc.BuyFromGoalInTx(context.Background(), db, userID, kapookAccID, salakAccID, decimal.RequireFromString("2000"))
		require.NoError(t, err)
		assert.False(t, result.GoalCompleted)
		assert.True(t, decimal.RequireFromString("2000").Equal(result.Goal.SalakAmount))
		assert.Equal(t, holdingID, result.Receipt.HoldingID)
		assert.Equal(t, 1, buySalakSvc.callCount)
		require.NotNil(t, transactions.lastCreated.IsAutomaticPurchase)
		assert.True(t, *transactions.lastCreated.IsAutomaticPurchase, "the worker-driven path always marks its own purchase automatic")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("validation failure still runs before touching the tx", func(t *testing.T) {
		sameID := uuid.New()
		accounts := &fakeAccountService{getByIDResult: kapookAccount(sameID, userID)}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), newFakeGoalRepo(), &fakeSalakService{}, accounts, clock.Real{})

		_, err := svc.BuyFromGoalInTx(context.Background(), nil, userID, sameID, sameID, decimal.RequireFromString("1000"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("a purchase failure is propagated verbatim", func(t *testing.T) {
		kapookAccID, salakAccID := uuid.New(), uuid.New()
		goal := kapookdomain.Goal{
			ID: uuid.New(), AccountID: kapookAccID, ProductID: product.ID, IsActive: true,
			GoalAmount: decimal.RequireFromString("5000"), SavingAmount: decimal.RequireFromString("3000"),
		}
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
			salakAccID:  {ID: salakAccID, UserID: userID, Type: accountdomain.TypeSalak},
		}}
		goals := newFakeGoalRepo()
		goals.activeByAccount[kapookAccID] = goal
		salakSvc := &fakeSalakService{getProductResult: product}
		buySalakSvc := &fakeBuySalakService{buySalakForKapookErr: apperror.Validation("salak: product is closed for purchases on this draw day")}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, salakSvc, accounts, nil, &fakeLedgerRepo{}, &fakeTransactionRepo{}, clock.Real{}, buySalakSvc, defaultTestCountdownDuration)

		_, err := svc.BuyFromGoalInTx(context.Background(), db, userID, kapookAccID, salakAccID, decimal.RequireFromString("1000"))
		assertAppErrKind(t, err, apperror.KindValidation)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- GetGoalHistory ------------------------------------------------------

func TestKapookService_GetGoalHistory(t *testing.T) {
	userID := uuid.New()

	t.Run("returns each row with server-computed fee/net, scoped to the given goal", func(t *testing.T) {
		kapookAccID, goalID := uuid.New(), uuid.New()
		goal := kapookdomain.Goal{ID: goalID, AccountID: kapookAccID}
		goals := newFakeGoalRepo()
		goals.byID = map[uuid.UUID]kapookdomain.Goal{goalID: goal}
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
		}}
		deposit := kapookdomain.Transaction{ID: uuid.New(), Type: kapookdomain.TransactionDeposit, Amount: decimal.RequireFromString("500")}
		withdrawFree := kapookdomain.Transaction{ID: uuid.New(), Type: kapookdomain.TransactionWithdraw, Amount: decimal.RequireFromString("200")}
		withdrawFee := kapookdomain.Transaction{ID: uuid.New(), Type: kapookdomain.TransactionWithdrawWithFee, Amount: decimal.RequireFromString("1000")}
		buy := kapookdomain.Transaction{ID: uuid.New(), Type: kapookdomain.TransactionBuySalak, Amount: decimal.RequireFromString("1000")}
		transactions := &fakeTransactionRepo{listByGoalResult: []kapookdomain.Transaction{withdrawFee, withdrawFree, buy, deposit}}

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, nil, &fakeLedgerRepo{}, transactions, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		entries, err := svc.GetGoalHistory(context.Background(), userID, goalID, 10, 0)
		require.NoError(t, err)
		require.Len(t, entries, 4)

		assert.Equal(t, transactions.lastListGoalID, goalID)

		byType := map[kapookdomain.TransactionType]kapook.HistoryEntry{}
		for _, e := range entries {
			byType[e.Transaction.Type] = e
		}

		depositEntry := byType[kapookdomain.TransactionDeposit]
		assert.True(t, decimal.Zero.Equal(depositEntry.Fee), "deposit carries no fee")
		assert.True(t, decimal.RequireFromString("500").Equal(depositEntry.Net))

		freeEntry := byType[kapookdomain.TransactionWithdraw]
		assert.True(t, decimal.Zero.Equal(freeEntry.Fee), "a free withdrawal carries no fee")
		assert.True(t, decimal.RequireFromString("200").Equal(freeEntry.Net))

		feeEntry := byType[kapookdomain.TransactionWithdrawWithFee]
		assert.True(t, decimal.RequireFromString("20").Equal(feeEntry.Fee), "2%% of 1000")
		assert.True(t, decimal.RequireFromString("980").Equal(feeEntry.Net))

		buyEntry := byType[kapookdomain.TransactionBuySalak]
		assert.True(t, decimal.Zero.Equal(buyEntry.Fee), "a purchase carries no withdrawal fee")
		assert.True(t, decimal.RequireFromString("1000").Equal(buyEntry.Net))
	})

	t.Run("a satang withdraw_with_fee row's fee rounds to two decimal places", func(t *testing.T) {
		kapookAccID, goalID := uuid.New(), uuid.New()
		goals := newFakeGoalRepo()
		goals.byID = map[uuid.UUID]kapookdomain.Goal{goalID: {ID: goalID, AccountID: kapookAccID}}
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
		}}
		row := kapookdomain.Transaction{ID: uuid.New(), Type: kapookdomain.TransactionWithdrawWithFee, Amount: decimal.RequireFromString("1000.01")}
		transactions := &fakeTransactionRepo{listByGoalResult: []kapookdomain.Transaction{row}}

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, nil, &fakeLedgerRepo{}, transactions, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		entries, err := svc.GetGoalHistory(context.Background(), userID, goalID, 10, 0)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.True(t, decimal.RequireFromString("20.00").Equal(entries[0].Fee), "20.0002 must round to 20.00")
		assert.True(t, decimal.RequireFromString("980.01").Equal(entries[0].Net))
	})

	t.Run("a nonexistent goal returns not found", func(t *testing.T) {
		goals := newFakeGoalRepo()
		svc := newKapookServiceWithClock(newFakeTermsRepo(), goals, &fakeSalakService{}, &fakeAccountService{}, clock.Real{})

		_, err := svc.GetGoalHistory(context.Background(), userID, uuid.New(), 10, 0)
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("a goal owned by a different customer is masked as not found, identically to a missing one", func(t *testing.T) {
		kapookAccID, goalID := uuid.New(), uuid.New()
		goals := newFakeGoalRepo()
		goals.byID = map[uuid.UUID]kapookdomain.Goal{goalID: {ID: goalID, AccountID: kapookAccID}}
		accounts := &fakeAccountService{errByID: map[uuid.UUID]error{kapookAccID: apperror.NotFound("account not found")}}
		svc := newKapookServiceWithClock(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, clock.Real{})

		_, err := svc.GetGoalHistory(context.Background(), userID, goalID, 10, 0)
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("limit/offset default and clamp the same way the account ledger endpoint does", func(t *testing.T) {
		kapookAccID, goalID := uuid.New(), uuid.New()
		goals := newFakeGoalRepo()
		goals.byID = map[uuid.UUID]kapookdomain.Goal{goalID: {ID: goalID, AccountID: kapookAccID}}
		accounts := &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{
			kapookAccID: kapookAccount(kapookAccID, userID),
		}}
		transactions := &fakeTransactionRepo{}
		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, accounts, nil, &fakeLedgerRepo{}, transactions, clock.Real{}, &fakeBuySalakService{}, defaultTestCountdownDuration)

		_, err := svc.GetGoalHistory(context.Background(), userID, goalID, 0, -5)
		require.NoError(t, err)
		assert.Equal(t, 20, transactions.lastListLimit, "non-positive limit defaults to 20")
		assert.Equal(t, 0, transactions.lastListOffset, "negative offset clamps to 0")

		_, err = svc.GetGoalHistory(context.Background(), userID, goalID, 500, 0)
		require.NoError(t, err)
		assert.Equal(t, 100, transactions.lastListLimit, "limit caps at 100")
	})
}

// --- SettleMaturedHolding ----------------------------------------------------

func TestKapookService_SettleMaturedHolding(t *testing.T) {
	t.Run("directly-purchased holding: money moves, no kapook bookkeeping touched", func(t *testing.T) {
		holdingID := uuid.New()
		buySalakSvc := &fakeBuySalakService{}
		buySalakSvc.settleResult = transaction.SettlementReceipt{HoldingID: holdingID, Principal: decimal.RequireFromString("1000")}
		transactions := &fakeTransactionRepo{} // FindByHoldingID finds nothing - never linked to a goal
		goals := newFakeGoalRepo()

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, &fakeAccountService{}, db, &fakeLedgerRepo{}, transactions, clock.Real{}, buySalakSvc, defaultTestCountdownDuration)

		receipt, err := svc.SettleMaturedHolding(context.Background(), holdingID)
		require.NoError(t, err)
		assert.Equal(t, holdingID, receipt.HoldingID)
		assert.False(t, goals.updateAfterExpirationCalled, "no goal to update for a non-Kapook holding")
		assert.Empty(t, transactions.created, "no salak_expiration row for a non-Kapook holding")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Kapook-originated holding: decrements SalakAmount and records a salak_expiration row", func(t *testing.T) {
		holdingID, goalID, kapookAccID := uuid.New(), uuid.New(), uuid.New()
		buySalakSvc := &fakeBuySalakService{}
		buySalakSvc.settleResult = transaction.SettlementReceipt{HoldingID: holdingID, Principal: decimal.RequireFromString("1000")}

		transactions := &fakeTransactionRepo{
			created: []kapookdomain.Transaction{
				{ID: uuid.New(), Type: kapookdomain.TransactionBuySalak, GoalID: goalID, KapookAccountID: kapookAccID, HoldingID: &holdingID},
			},
		}
		goals := newFakeGoalRepo()
		goals.byID = map[uuid.UUID]kapookdomain.Goal{
			goalID: {ID: goalID, AccountID: kapookAccID, SalakAmount: decimal.RequireFromString("1500"), IsActive: false},
		}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, &fakeAccountService{}, db, &fakeLedgerRepo{}, transactions, clock.Real{}, buySalakSvc, defaultTestCountdownDuration)

		receipt, err := svc.SettleMaturedHolding(context.Background(), holdingID)
		require.NoError(t, err)
		assert.Equal(t, holdingID, receipt.HoldingID)

		require.True(t, goals.updateAfterExpirationCalled)
		assert.Equal(t, goalID, goals.lastExpirationGoalID)
		assert.True(t, decimal.RequireFromString("500").Equal(goals.lastExpirationNewSalakAmount), "1500 - 1000 principal")

		require.Len(t, transactions.created, 2, "the pre-existing buy_salak row, plus one new salak_expiration row")
		expirationRow := transactions.created[1]
		assert.Equal(t, kapookdomain.TransactionSalakExpiration, expirationRow.Type)
		assert.Equal(t, goalID, expirationRow.GoalID)
		assert.Equal(t, kapookAccID, expirationRow.KapookAccountID)
		require.NotNil(t, expirationRow.HoldingID)
		assert.Equal(t, holdingID, *expirationRow.HoldingID)
		assert.True(t, decimal.RequireFromString("1000").Equal(expirationRow.Amount))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("a goal's SalakAmount never goes negative even if principal exceeds it", func(t *testing.T) {
		holdingID, goalID, kapookAccID := uuid.New(), uuid.New(), uuid.New()
		buySalakSvc := &fakeBuySalakService{}
		buySalakSvc.settleResult = transaction.SettlementReceipt{HoldingID: holdingID, Principal: decimal.RequireFromString("1000")}

		transactions := &fakeTransactionRepo{
			created: []kapookdomain.Transaction{
				{ID: uuid.New(), Type: kapookdomain.TransactionBuySalak, GoalID: goalID, KapookAccountID: kapookAccID, HoldingID: &holdingID},
			},
		}
		goals := newFakeGoalRepo()
		goals.byID = map[uuid.UUID]kapookdomain.Goal{
			goalID: {ID: goalID, AccountID: kapookAccID, SalakAmount: decimal.RequireFromString("400")},
		}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, &fakeAccountService{}, db, &fakeLedgerRepo{}, transactions, clock.Real{}, buySalakSvc, defaultTestCountdownDuration)

		_, err := svc.SettleMaturedHolding(context.Background(), holdingID)
		require.NoError(t, err)
		assert.True(t, decimal.Zero.Equal(goals.lastExpirationNewSalakAmount), "clamped at zero, never negative")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("settlement failure rolls back and is propagated verbatim, no kapook bookkeeping attempted", func(t *testing.T) {
		holdingID := uuid.New()
		buySalakSvc := &fakeBuySalakService{}
		buySalakSvc.settleErr = apperror.Conflict("holding has already been settled")
		transactions := &fakeTransactionRepo{}
		goals := newFakeGoalRepo()

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewKapookService(newFakeTermsRepo(), goals, &fakeSalakService{}, &fakeAccountService{}, db, &fakeLedgerRepo{}, transactions, clock.Real{}, buySalakSvc, defaultTestCountdownDuration)

		_, err := svc.SettleMaturedHolding(context.Background(), holdingID)
		assertAppErrKind(t, err, apperror.KindConflict)
		assert.False(t, goals.updateAfterExpirationCalled)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
