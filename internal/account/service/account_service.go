package service

import (
	"context"
	"errors"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type AccountService struct {
	repo account.Repository
}

func NewAccountService(repo account.Repository) *AccountService {
	return &AccountService{repo: repo}
}

var _ account.Service = (*AccountService)(nil)

func (s *AccountService) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	accounts, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, apperror.Internal("failed to list accounts", err)
	}
	return accounts, nil
}

func (s *AccountService) GetByID(ctx context.Context, userID, accountID uuid.UUID) (domain.Account, error) {
	a, err := s.repo.FindByID(ctx, accountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Account{}, apperror.NotFound("account not found")
	} else if err != nil {
		return domain.Account{}, apperror.Internal("failed to look up account", err)
	}
	if a.UserID != userID {
		return domain.Account{}, apperror.NotFound("account not found")
	}
	return a, nil
}

func (s *AccountService) GetByIDUnscoped(ctx context.Context, accountID uuid.UUID) (domain.Account, error) {
	a, err := s.repo.FindByID(ctx, accountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Account{}, apperror.NotFound("account not found")
	} else if err != nil {
		return domain.Account{}, apperror.Internal("failed to look up account", err)
	}
	return a, nil
}

func (s *AccountService) Create(ctx context.Context, tx *gorm.DB, userID uuid.UUID, accountType domain.Type, startingBalance decimal.Decimal, isPrimary bool) (domain.Account, error) {
	number, err := s.repo.NextAccountNumber(ctx, tx, accountType)
	if err != nil {
		return domain.Account{}, apperror.Internal("failed to generate account number", err)
	}
	a := &domain.Account{
		ID:               uuid.New(),
		UserID:           userID,
		AccountNumber:    number,
		Type:             accountType,
		Balance:          startingBalance,
		Currency:         "THB",
		IsPrimaryAccount: isPrimary,
	}
	if err := s.repo.Create(ctx, tx, a); err != nil {
		return domain.Account{}, apperror.Internal("failed to create account", err)
	}
	return *a, nil
}

func (s *AccountService) GetPrimaryAccount(ctx context.Context, userID uuid.UUID) (domain.Account, error) {
	a, err := s.repo.FindPrimaryByUserID(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Account{}, apperror.NotFound("no primary account for this user")
	} else if err != nil {
		return domain.Account{}, apperror.Internal("failed to look up primary account", err)
	}
	return a, nil
}

func (s *AccountService) Debit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	a, err := s.repo.FindForUpdate(ctx, tx, accountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return decimal.Zero, apperror.NotFound("account not found")
	} else if err != nil {
		return decimal.Zero, apperror.Internal("failed to lock account", err)
	}

	newBalance := a.Balance.Sub(amount)
	if newBalance.IsNegative() {
		return decimal.Zero, apperror.Validation("insufficient funds")
	}

	if err := s.repo.UpdateBalance(ctx, tx, accountID, newBalance); err != nil {
		return decimal.Zero, apperror.Internal("failed to debit account", err)
	}
	return newBalance, nil
}

func (s *AccountService) LockForUpdate(ctx context.Context, tx *gorm.DB, accountID uuid.UUID) (domain.Account, error) {
	a, err := s.repo.FindForUpdate(ctx, tx, accountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Account{}, apperror.NotFound("account not found")
	} else if err != nil {
		return domain.Account{}, apperror.Internal("failed to lock account", err)
	}
	return a, nil
}

func (s *AccountService) Credit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	a, err := s.repo.FindForUpdate(ctx, tx, accountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return decimal.Zero, apperror.NotFound("account not found")
	} else if err != nil {
		return decimal.Zero, apperror.Internal("failed to lock account", err)
	}

	newBalance := a.Balance.Add(amount)
	if err := s.repo.UpdateBalance(ctx, tx, accountID, newBalance); err != nil {
		return decimal.Zero, apperror.Internal("failed to credit account", err)
	}
	return newBalance, nil
}
