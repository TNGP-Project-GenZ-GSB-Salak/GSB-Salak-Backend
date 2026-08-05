package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/worker"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// --- fakes -------------------------------------------------------------

// fakeGoalRepo implements kapook.GoalRepository; RunOnce only ever calls
// ClaimDueGoals, but the interface must be satisfied in full.
type fakeGoalRepo struct {
	claimResult []kapookdomain.Goal
	claimErr    error

	setDeferralErr error
	deferralCalls  []deferralCall
}

type deferralCall struct {
	GoalID uuid.UUID
	Until  time.Time
}

func (f *fakeGoalRepo) Create(ctx context.Context, g *kapookdomain.Goal) error { return nil }
func (f *fakeGoalRepo) FindActiveByAccountID(ctx context.Context, accountID uuid.UUID) (kapookdomain.Goal, error) {
	return kapookdomain.Goal{}, gorm.ErrRecordNotFound
}
func (f *fakeGoalRepo) FindByID(ctx context.Context, goalID uuid.UUID) (kapookdomain.Goal, error) {
	return kapookdomain.Goal{}, gorm.ErrRecordNotFound
}
func (f *fakeGoalRepo) FindActiveByAccountIDForUpdate(ctx context.Context, tx *gorm.DB, accountID uuid.UUID) (kapookdomain.Goal, error) {
	return kapookdomain.Goal{}, gorm.ErrRecordNotFound
}
func (f *fakeGoalRepo) UpdateSavingAmount(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSavingAmount decimal.Decimal) error {
	return nil
}
func (f *fakeGoalRepo) UpdateAfterPurchase(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSalakAmount decimal.Decimal, stillActive bool) error {
	return nil
}
func (f *fakeGoalRepo) UpdateAfterWithdrawal(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSavingAmount decimal.Decimal, stillActive bool) error {
	return nil
}
func (f *fakeGoalRepo) MarkGoalReached(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, reachedAt time.Time) error {
	return nil
}
func (f *fakeGoalRepo) ClaimDueGoals(ctx context.Context, tx *gorm.DB, cutoff, today time.Time, limit int) ([]kapookdomain.Goal, error) {
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	return f.claimResult, nil
}
func (f *fakeGoalRepo) SetAutoPurchaseDeferral(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, until time.Time) error {
	f.deferralCalls = append(f.deferralCalls, deferralCall{GoalID: goalID, Until: until})
	return f.setDeferralErr
}

// fakeAccountService implements account.Service; the worker only calls
// GetByIDUnscoped and ListByUser.
type fakeAccountService struct {
	byID           map[uuid.UUID]accountdomain.Account
	byIDErr        map[uuid.UUID]error
	accountsByUser map[uuid.UUID][]accountdomain.Account
}

func newFakeAccountService() *fakeAccountService {
	return &fakeAccountService{byID: map[uuid.UUID]accountdomain.Account{}, byIDErr: map[uuid.UUID]error{}, accountsByUser: map[uuid.UUID][]accountdomain.Account{}}
}

func (f *fakeAccountService) ListByUser(ctx context.Context, userID uuid.UUID) ([]accountdomain.Account, error) {
	return f.accountsByUser[userID], nil
}
func (f *fakeAccountService) GetByID(ctx context.Context, userID, accountID uuid.UUID) (accountdomain.Account, error) {
	return f.GetByIDUnscoped(ctx, accountID)
}
func (f *fakeAccountService) GetByIDUnscoped(ctx context.Context, accountID uuid.UUID) (accountdomain.Account, error) {
	if err, ok := f.byIDErr[accountID]; ok {
		return accountdomain.Account{}, err
	}
	a, ok := f.byID[accountID]
	if !ok {
		return accountdomain.Account{}, apperror.NotFound("account not found")
	}
	return a, nil
}
func (f *fakeAccountService) Debit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	return decimal.Zero, nil
}
func (f *fakeAccountService) Credit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	return decimal.Zero, nil
}
func (f *fakeAccountService) LockForUpdate(ctx context.Context, tx *gorm.DB, accountID uuid.UUID) (accountdomain.Account, error) {
	return f.GetByIDUnscoped(ctx, accountID)
}

// Create and GetPrimaryAccount are unused by the worker - stubbed only to
// satisfy account.Service.
func (f *fakeAccountService) Create(ctx context.Context, tx *gorm.DB, userID uuid.UUID, accountType accountdomain.Type, startingBalance decimal.Decimal, isPrimary bool) (accountdomain.Account, error) {
	return accountdomain.Account{}, nil
}

func (f *fakeAccountService) GetPrimaryAccount(ctx context.Context, userID uuid.UUID) (accountdomain.Account, error) {
	return accountdomain.Account{}, nil
}

// fakeKapookService implements kapook.Service; the worker only calls
// BuyFromGoalInTx.
type fakeKapookService struct {
	buyErr    error
	buyResult kapook.BuyFromGoalResult

	calls []buyFromGoalInTxCall
}

type buyFromGoalInTxCall struct {
	UserID, KapookAccountID, SalakAccountID uuid.UUID
	Amount                                  decimal.Decimal
}

func (f *fakeKapookService) Accept(ctx context.Context, userID uuid.UUID) error { return nil }
func (f *fakeKapookService) HasAccepted(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}
func (f *fakeKapookService) CreateGoal(ctx context.Context, userID, accountID, productID uuid.UUID, goalAmount decimal.Decimal) (kapookdomain.Goal, error) {
	return kapookdomain.Goal{}, nil
}
func (f *fakeKapookService) GetActiveGoal(ctx context.Context, userID, accountID uuid.UUID) (*kapookdomain.Goal, error) {
	return nil, nil
}
func (f *fakeKapookService) Snapshot(ctx context.Context, goal kapookdomain.Goal) (kapook.GoalSnapshot, error) {
	return kapook.GoalSnapshot{Goal: goal}, nil
}
func (f *fakeKapookService) Deposit(ctx context.Context, userID, kapookAccountID, savingsAccountID uuid.UUID, amount decimal.Decimal) (kapookdomain.Goal, error) {
	return kapookdomain.Goal{}, nil
}
func (f *fakeKapookService) Withdraw(ctx context.Context, userID, kapookAccountID uuid.UUID, amount decimal.Decimal) (kapook.WithdrawResult, error) {
	return kapook.WithdrawResult{}, nil
}
func (f *fakeKapookService) GetWithdrawalStatus(ctx context.Context, userID, kapookAccountID uuid.UUID, amount *decimal.Decimal) (kapook.WithdrawalStatus, error) {
	return kapook.WithdrawalStatus{}, nil
}
func (f *fakeKapookService) GetGoalHistory(ctx context.Context, userID, goalID uuid.UUID, limit, offset int) ([]kapook.HistoryEntry, error) {
	return nil, nil
}
func (f *fakeKapookService) BuyFromGoal(ctx context.Context, userID, kapookAccountID, salakAccountID uuid.UUID, amount decimal.Decimal) (kapook.BuyFromGoalResult, error) {
	return kapook.BuyFromGoalResult{}, nil
}
func (f *fakeKapookService) BuyFromGoalInTx(ctx context.Context, tx *gorm.DB, userID, kapookAccountID, salakAccountID uuid.UUID, amount decimal.Decimal) (kapook.BuyFromGoalResult, error) {
	f.calls = append(f.calls, buyFromGoalInTxCall{userID, kapookAccountID, salakAccountID, amount})
	if f.buyErr != nil {
		return kapook.BuyFromGoalResult{}, f.buyErr
	}
	return f.buyResult, nil
}

// fakeSalakService implements salak.Service; the worker only calls
// GetProduct and NextAvailableDate, both only when deferring a draw-day
// rejection.
type fakeSalakService struct {
	getProductResult salakdomain.Product
	getProductErr    error

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
	return nil
}
func (f *fakeSalakService) EnsureNotDrawDay(ctx context.Context, product salakdomain.Product) error {
	return nil
}
func (f *fakeSalakService) NextAvailableDate(ctx context.Context, product salakdomain.Product) (time.Time, error) {
	return f.nextAvailableDateResult, f.nextAvailableDateErr
}
func (f *fakeSalakService) MintHolding(ctx context.Context, tx *gorm.DB, accountID, productID uuid.UUID, amount decimal.Decimal) (salakdomain.Holding, error) {
	return salakdomain.Holding{}, nil
}
func (f *fakeSalakService) ListHoldingsByAccount(ctx context.Context, userID, accountID uuid.UUID) ([]salakdomain.Holding, error) {
	return nil, nil
}

// --- helpers -------------------------------------------------------------

func kapookAccount(id, userID uuid.UUID) accountdomain.Account {
	return accountdomain.Account{ID: id, UserID: userID, Type: accountdomain.TypeKapook}
}

func salakAccount(id, userID uuid.UUID) accountdomain.Account {
	return accountdomain.Account{ID: id, UserID: userID, Type: accountdomain.TypeSalak}
}

// newSQLMockDB backs a real *gorm.DB with sqlmock so RunOnce's own
// s.db.Transaction(...) call can actually Begin/Commit/Rollback without a
// live Postgres connection - every dependency it calls goes to a fake, not
// a real repo, so no query expectations beyond Begin/Commit(/Rollback) are
// ever needed.
func newSQLMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return gdb, mock
}

// newCommittingDB is newSQLMockDB for the common case: a pass that's
// expected to commit successfully regardless of individual goal outcomes
// (RunOnce's transaction always commits unless the claim itself errors).
func newCommittingDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, mock := newSQLMockDB(t)
	mock.ExpectBegin()
	mock.ExpectCommit()
	return db
}

func reachedGoal(id, accountID uuid.UUID, reachedAt time.Time, saving, salak decimal.Decimal) kapookdomain.Goal {
	return kapookdomain.Goal{
		ID:            id,
		AccountID:     accountID,
		IsActive:      true,
		GoalAmount:    decimal.RequireFromString("5000"),
		SavingAmount:  saving,
		SalakAmount:   salak,
		GoalReachedAt: &reachedAt,
	}
}

// --- RunOnce --------------------------------------------------------------

func TestWorker_RunOnce(t *testing.T) {
	t.Run("no due goals: an empty summary, no purchase attempted", func(t *testing.T) {
		goals := &fakeGoalRepo{}
		kapookSvc := &fakeKapookService{}
		w := worker.New(newCommittingDB(t), goals, newFakeAccountService(), kapookSvc, &fakeSalakService{}, clock.Real{}, 24*time.Hour)

		summary, err := w.RunOnce(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, summary.Claimed)
		assert.Empty(t, summary.Results)
		assert.Empty(t, kapookSvc.calls)
	})

	t.Run("a due goal is bought for its available balance, not the raw goal amount", func(t *testing.T) {
		userID, kapookAccID, salakAccID := uuid.New(), uuid.New(), uuid.New()
		goalID := uuid.New()
		reachedAt := time.Now().Add(-48 * time.Hour)
		// 5000 saved, 1000 already converted manually during the countdown -
		// only 4000 should be requested, not the full 5000 goal amount.
		goal := reachedGoal(goalID, kapookAccID, reachedAt, decimal.RequireFromString("5000"), decimal.RequireFromString("1000"))

		goals := &fakeGoalRepo{claimResult: []kapookdomain.Goal{goal}}
		accounts := newFakeAccountService()
		accounts.byID[kapookAccID] = kapookAccount(kapookAccID, userID)
		accounts.accountsByUser[userID] = []accountdomain.Account{
			kapookAccount(kapookAccID, userID),
			salakAccount(salakAccID, userID),
		}
		kapookSvc := &fakeKapookService{buyResult: kapook.BuyFromGoalResult{GoalCompleted: true}}

		w := worker.New(newCommittingDB(t), goals, accounts, kapookSvc, &fakeSalakService{}, clock.Real{}, 24*time.Hour)

		summary, err := w.RunOnce(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, summary.Claimed)
		require.Len(t, summary.Results, 1)
		assert.Equal(t, worker.OutcomePurchased, summary.Results[0].Outcome)
		assert.Equal(t, goalID, summary.Results[0].GoalID)

		require.Len(t, kapookSvc.calls, 1)
		call := kapookSvc.calls[0]
		assert.Equal(t, userID, call.UserID)
		assert.Equal(t, kapookAccID, call.KapookAccountID)
		assert.Equal(t, salakAccID, call.SalakAccountID)
		assert.True(t, decimal.RequireFromString("4000").Equal(call.Amount), "must buy the remaining available balance, not the full goal amount")
	})

	t.Run("a draw-day rejection is deferred and the retry date persisted, not counted as a failure", func(t *testing.T) {
		userID, kapookAccID, salakAccID := uuid.New(), uuid.New(), uuid.New()
		goalID := uuid.New()
		goal := reachedGoal(goalID, kapookAccID, time.Now().Add(-48*time.Hour), decimal.RequireFromString("5000"), decimal.Zero)
		goal.ProductID = uuid.New()

		goals := &fakeGoalRepo{claimResult: []kapookdomain.Goal{goal}}
		accounts := newFakeAccountService()
		accounts.byID[kapookAccID] = kapookAccount(kapookAccID, userID)
		accounts.accountsByUser[userID] = []accountdomain.Account{salakAccount(salakAccID, userID)}
		kapookSvc := &fakeKapookService{buyErr: apperror.Wrap(apperror.KindValidation, "salak cannot be purchased on its draw day", salak.ErrDrawDay)}
		nextAvailable := time.Date(2026, 1, 17, 0, 0, 0, 0, time.UTC)
		salakSvc := &fakeSalakService{
			getProductResult:        salakdomain.Product{ID: goal.ProductID},
			nextAvailableDateResult: nextAvailable,
		}

		w := worker.New(newCommittingDB(t), goals, accounts, kapookSvc, salakSvc, clock.Real{}, 24*time.Hour)

		summary, err := w.RunOnce(context.Background())
		require.NoError(t, err)
		require.Len(t, summary.Results, 1)
		assert.Equal(t, worker.OutcomeDeferred, summary.Results[0].Outcome)
		assert.Equal(t, 0, summary.Failed())

		require.Len(t, goals.deferralCalls, 1)
		assert.Equal(t, goalID, goals.deferralCalls[0].GoalID)
		assert.True(t, nextAvailable.Equal(goals.deferralCalls[0].Until))
	})

	t.Run("re-deferring an already-deferred goal on a later tick is harmless, not an error", func(t *testing.T) {
		userID, kapookAccID, salakAccID := uuid.New(), uuid.New(), uuid.New()
		goal := reachedGoal(uuid.New(), kapookAccID, time.Now().Add(-48*time.Hour), decimal.RequireFromString("5000"), decimal.Zero)

		goals := &fakeGoalRepo{claimResult: []kapookdomain.Goal{goal}}
		accounts := newFakeAccountService()
		accounts.byID[kapookAccID] = kapookAccount(kapookAccID, userID)
		accounts.accountsByUser[userID] = []accountdomain.Account{salakAccount(salakAccID, userID)}
		kapookSvc := &fakeKapookService{buyErr: apperror.Wrap(apperror.KindValidation, "salak cannot be purchased on its draw day", salak.ErrDrawDay)}
		salakSvc := &fakeSalakService{nextAvailableDateResult: time.Date(2026, 1, 17, 0, 0, 0, 0, time.UTC)}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectCommit()
		w := worker.New(db, goals, accounts, kapookSvc, salakSvc, clock.Real{}, 24*time.Hour)

		for i := 0; i < 2; i++ {
			summary, err := w.RunOnce(context.Background())
			require.NoError(t, err)
			require.Len(t, summary.Results, 1)
			assert.Equal(t, worker.OutcomeDeferred, summary.Results[0].Outcome)
		}
		assert.Len(t, goals.deferralCalls, 2, "re-setting the same deferral on a repeated tick must not error or be skipped")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("a failure to resolve the product while deferring is a real failure, not silently deferred", func(t *testing.T) {
		userID, kapookAccID, salakAccID := uuid.New(), uuid.New(), uuid.New()
		goal := reachedGoal(uuid.New(), kapookAccID, time.Now().Add(-48*time.Hour), decimal.RequireFromString("5000"), decimal.Zero)

		goals := &fakeGoalRepo{claimResult: []kapookdomain.Goal{goal}}
		accounts := newFakeAccountService()
		accounts.byID[kapookAccID] = kapookAccount(kapookAccID, userID)
		accounts.accountsByUser[userID] = []accountdomain.Account{salakAccount(salakAccID, userID)}
		kapookSvc := &fakeKapookService{buyErr: apperror.Wrap(apperror.KindValidation, "salak cannot be purchased on its draw day", salak.ErrDrawDay)}
		salakSvc := &fakeSalakService{getProductErr: apperror.NotFound("salak product not found")}

		w := worker.New(newCommittingDB(t), goals, accounts, kapookSvc, salakSvc, clock.Real{}, 24*time.Hour)

		summary, err := w.RunOnce(context.Background())
		require.NoError(t, err)
		require.Len(t, summary.Results, 1)
		assert.Equal(t, worker.OutcomeFailed, summary.Results[0].Outcome)
		assert.Empty(t, goals.deferralCalls)
	})

	t.Run("any other purchase error is recorded as a failure, with the error attached", func(t *testing.T) {
		userID, kapookAccID, salakAccID := uuid.New(), uuid.New(), uuid.New()
		goal := reachedGoal(uuid.New(), kapookAccID, time.Now().Add(-48*time.Hour), decimal.RequireFromString("5000"), decimal.Zero)

		goals := &fakeGoalRepo{claimResult: []kapookdomain.Goal{goal}}
		accounts := newFakeAccountService()
		accounts.byID[kapookAccID] = kapookAccount(kapookAccID, userID)
		accounts.accountsByUser[userID] = []accountdomain.Account{salakAccount(salakAccID, userID)}
		wantErr := apperror.Validation("salak product is not available for purchase")
		kapookSvc := &fakeKapookService{buyErr: wantErr}

		w := worker.New(newCommittingDB(t), goals, accounts, kapookSvc, &fakeSalakService{}, clock.Real{}, 24*time.Hour)

		summary, err := w.RunOnce(context.Background())
		require.NoError(t, err)
		require.Len(t, summary.Results, 1)
		assert.Equal(t, worker.OutcomeFailed, summary.Results[0].Outcome)
		assert.ErrorIs(t, summary.Results[0].Err, wantErr)
	})

	t.Run("no salak account for the user is recorded as an explicit failure, not silently skipped", func(t *testing.T) {
		userID, kapookAccID := uuid.New(), uuid.New()
		goal := reachedGoal(uuid.New(), kapookAccID, time.Now().Add(-48*time.Hour), decimal.RequireFromString("5000"), decimal.Zero)

		goals := &fakeGoalRepo{claimResult: []kapookdomain.Goal{goal}}
		accounts := newFakeAccountService()
		accounts.byID[kapookAccID] = kapookAccount(kapookAccID, userID)
		// accountsByUser[userID] deliberately has no salak account.
		kapookSvc := &fakeKapookService{}

		w := worker.New(newCommittingDB(t), goals, accounts, kapookSvc, &fakeSalakService{}, clock.Real{}, 24*time.Hour)

		summary, err := w.RunOnce(context.Background())
		require.NoError(t, err)
		require.Len(t, summary.Results, 1)
		assert.Equal(t, worker.OutcomeFailed, summary.Results[0].Outcome)
		require.Error(t, summary.Results[0].Err)
		assert.Empty(t, kapookSvc.calls, "must not attempt a purchase with no salak account to fund")
	})

	t.Run("a claimed goal that fails the defensive is_active/reached_at re-check is skipped, not processed", func(t *testing.T) {
		inactiveGoal := reachedGoal(uuid.New(), uuid.New(), time.Now().Add(-48*time.Hour), decimal.RequireFromString("5000"), decimal.Zero)
		inactiveGoal.IsActive = false
		notReachedGoal := reachedGoal(uuid.New(), uuid.New(), time.Now().Add(-48*time.Hour), decimal.RequireFromString("5000"), decimal.Zero)
		notReachedGoal.GoalReachedAt = nil

		goals := &fakeGoalRepo{claimResult: []kapookdomain.Goal{inactiveGoal, notReachedGoal}}
		kapookSvc := &fakeKapookService{}
		w := worker.New(newCommittingDB(t), goals, newFakeAccountService(), kapookSvc, &fakeSalakService{}, clock.Real{}, 24*time.Hour)

		summary, err := w.RunOnce(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 2, summary.Claimed)
		assert.Empty(t, summary.Results, "neither claimed row should have been processed")
		assert.Empty(t, kapookSvc.calls)
	})

	t.Run("a claim failure propagates as RunOnce's own error", func(t *testing.T) {
		goals := &fakeGoalRepo{claimErr: errors.New("db down")}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()
		w := worker.New(db, goals, newFakeAccountService(), &fakeKapookService{}, &fakeSalakService{}, clock.Real{}, 24*time.Hour)

		_, err := w.RunOnce(context.Background())
		require.Error(t, err)
	})
}
