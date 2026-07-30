package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/account/service"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeAccountRepo is a hand-rolled in-memory implementation of account.Repository.
type fakeAccountRepo struct {
	byID map[uuid.UUID]domain.Account

	findByUserIDErr   error
	findByIDErr       error
	findForUpdateErr  error
	updateBalanceErr  error
	createErr         error
	lastUpdatedID     uuid.UUID
	lastUpdatedAmount decimal.Decimal
}

func newFakeAccountRepo(accounts ...domain.Account) *fakeAccountRepo {
	r := &fakeAccountRepo{byID: map[uuid.UUID]domain.Account{}}
	for _, a := range accounts {
		r.byID[a.ID] = a
	}
	return r
}

func (f *fakeAccountRepo) Create(ctx context.Context, tx *gorm.DB, a *domain.Account) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[a.ID] = *a
	return nil
}

func (f *fakeAccountRepo) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	if f.findByUserIDErr != nil {
		return nil, f.findByUserIDErr
	}
	var out []domain.Account
	for _, a := range f.byID {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAccountRepo) FindByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	if f.findByIDErr != nil {
		return domain.Account{}, f.findByIDErr
	}
	a, ok := f.byID[id]
	if !ok {
		return domain.Account{}, gorm.ErrRecordNotFound
	}
	return a, nil
}

func (f *fakeAccountRepo) FindForUpdate(ctx context.Context, tx *gorm.DB, id uuid.UUID) (domain.Account, error) {
	if f.findForUpdateErr != nil {
		return domain.Account{}, f.findForUpdateErr
	}
	a, ok := f.byID[id]
	if !ok {
		return domain.Account{}, gorm.ErrRecordNotFound
	}
	return a, nil
}

func (f *fakeAccountRepo) UpdateBalance(ctx context.Context, tx *gorm.DB, id uuid.UUID, newBalance decimal.Decimal) error {
	if f.updateBalanceErr != nil {
		return f.updateBalanceErr
	}
	f.lastUpdatedID = id
	f.lastUpdatedAmount = newBalance
	a := f.byID[id]
	a.Balance = newBalance
	f.byID[id] = a
	return nil
}

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

func TestAccountService_ListByUser(t *testing.T) {
	userID := uuid.New()
	other := uuid.New()

	t.Run("success returns only that user's accounts", func(t *testing.T) {
		a1 := domain.Account{ID: uuid.New(), UserID: userID}
		a2 := domain.Account{ID: uuid.New(), UserID: other}
		repo := newFakeAccountRepo(a1, a2)
		svc := service.NewAccountService(repo)

		got, err := svc.ListByUser(context.Background(), userID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, a1.ID, got[0].ID)
	})

	t.Run("empty result when user has no accounts", func(t *testing.T) {
		repo := newFakeAccountRepo()
		svc := service.NewAccountService(repo)

		got, err := svc.ListByUser(context.Background(), userID)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("repo error returns internal error", func(t *testing.T) {
		repo := newFakeAccountRepo()
		repo.findByUserIDErr = errors.New("db down")
		svc := service.NewAccountService(repo)

		_, err := svc.ListByUser(context.Background(), userID)
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

func TestAccountService_GetByID(t *testing.T) {
	userID := uuid.New()
	accountID := uuid.New()

	t.Run("success", func(t *testing.T) {
		repo := newFakeAccountRepo(domain.Account{ID: accountID, UserID: userID})
		svc := service.NewAccountService(repo)

		a, err := svc.GetByID(context.Background(), userID, accountID)
		require.NoError(t, err)
		assert.Equal(t, accountID, a.ID)
	})

	t.Run("not found", func(t *testing.T) {
		repo := newFakeAccountRepo()
		svc := service.NewAccountService(repo)

		_, err := svc.GetByID(context.Background(), userID, accountID)
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("account belongs to a different user returns not found, not forbidden", func(t *testing.T) {
		repo := newFakeAccountRepo(domain.Account{ID: accountID, UserID: uuid.New()})
		svc := service.NewAccountService(repo)

		_, err := svc.GetByID(context.Background(), userID, accountID)
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("repo error returns internal error", func(t *testing.T) {
		repo := newFakeAccountRepo()
		repo.findByIDErr = errors.New("db down")
		svc := service.NewAccountService(repo)

		_, err := svc.GetByID(context.Background(), userID, accountID)
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

func TestAccountService_Debit(t *testing.T) {
	accountID := uuid.New()

	t.Run("success reduces balance", func(t *testing.T) {
		repo := newFakeAccountRepo(domain.Account{ID: accountID, Balance: mustDecimal(t, "100.00")})
		svc := service.NewAccountService(repo)

		newBal, err := svc.Debit(context.Background(), nil, accountID, mustDecimal(t, "40.00"))
		require.NoError(t, err)
		assert.True(t, mustDecimal(t, "60.00").Equal(newBal))
		assert.True(t, mustDecimal(t, "60.00").Equal(repo.lastUpdatedAmount))
	})

	t.Run("debiting exact balance leaves zero, not an error", func(t *testing.T) {
		repo := newFakeAccountRepo(domain.Account{ID: accountID, Balance: mustDecimal(t, "50.00")})
		svc := service.NewAccountService(repo)

		newBal, err := svc.Debit(context.Background(), nil, accountID, mustDecimal(t, "50.00"))
		require.NoError(t, err)
		assert.True(t, decimal.Zero.Equal(newBal))
	})

	t.Run("insufficient funds is rejected", func(t *testing.T) {
		repo := newFakeAccountRepo(domain.Account{ID: accountID, Balance: mustDecimal(t, "10.00")})
		svc := service.NewAccountService(repo)

		_, err := svc.Debit(context.Background(), nil, accountID, mustDecimal(t, "10.01"))
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("account not found", func(t *testing.T) {
		repo := newFakeAccountRepo()
		svc := service.NewAccountService(repo)

		_, err := svc.Debit(context.Background(), nil, accountID, mustDecimal(t, "1.00"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("lock/lookup error returns internal error", func(t *testing.T) {
		repo := newFakeAccountRepo()
		repo.findForUpdateErr = errors.New("lock timeout")
		svc := service.NewAccountService(repo)

		_, err := svc.Debit(context.Background(), nil, accountID, mustDecimal(t, "1.00"))
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("update failure returns internal error", func(t *testing.T) {
		repo := newFakeAccountRepo(domain.Account{ID: accountID, Balance: mustDecimal(t, "100.00")})
		repo.updateBalanceErr = errors.New("write failed")
		svc := service.NewAccountService(repo)

		_, err := svc.Debit(context.Background(), nil, accountID, mustDecimal(t, "10.00"))
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

func TestAccountService_Credit(t *testing.T) {
	accountID := uuid.New()

	t.Run("success increases balance", func(t *testing.T) {
		repo := newFakeAccountRepo(domain.Account{ID: accountID, Balance: mustDecimal(t, "100.00")})
		svc := service.NewAccountService(repo)

		newBal, err := svc.Credit(context.Background(), nil, accountID, mustDecimal(t, "25.50"))
		require.NoError(t, err)
		assert.True(t, mustDecimal(t, "125.50").Equal(newBal))
	})

	t.Run("account not found", func(t *testing.T) {
		repo := newFakeAccountRepo()
		svc := service.NewAccountService(repo)

		_, err := svc.Credit(context.Background(), nil, accountID, mustDecimal(t, "1.00"))
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("lock/lookup error returns internal error", func(t *testing.T) {
		repo := newFakeAccountRepo()
		repo.findForUpdateErr = errors.New("lock timeout")
		svc := service.NewAccountService(repo)

		_, err := svc.Credit(context.Background(), nil, accountID, mustDecimal(t, "1.00"))
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("update failure returns internal error", func(t *testing.T) {
		repo := newFakeAccountRepo(domain.Account{ID: accountID, Balance: mustDecimal(t, "100.00")})
		repo.updateBalanceErr = errors.New("write failed")
		svc := service.NewAccountService(repo)

		_, err := svc.Credit(context.Background(), nil, accountID, mustDecimal(t, "10.00"))
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("crediting zero amount is a no-op success", func(t *testing.T) {
		repo := newFakeAccountRepo(domain.Account{ID: accountID, Balance: mustDecimal(t, "100.00")})
		svc := service.NewAccountService(repo)

		newBal, err := svc.Credit(context.Background(), nil, accountID, decimal.Zero)
		require.NoError(t, err)
		assert.True(t, mustDecimal(t, "100.00").Equal(newBal))
	})
}
