package account

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Repository is implemented by the gorm repository and consumed by the service.
type Repository interface {
	Create(ctx context.Context, tx *gorm.DB, a *domain.Account) error
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Account, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.Account, error)
	FindForUpdate(ctx context.Context, tx *gorm.DB, id uuid.UUID) (domain.Account, error)
	UpdateBalance(ctx context.Context, tx *gorm.DB, id uuid.UUID, newBalance decimal.Decimal) error
}

// Service is the public surface the transaction domain (and http layer) depend on.
// Mutating methods take an explicit tx so callers can compose them into one
// Postgres transaction that spans multiple domains/schemas.
type Service interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Account, error)
	GetByID(ctx context.Context, userID, accountID uuid.UUID) (domain.Account, error)
	Debit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error)
	Credit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error)
	// LockForUpdate takes a row lock on accountID without changing its
	// balance, so an orchestrator that debits one account and credits
	// another can lock them in a fixed, money-flow-independent order (lock
	// this account first, regardless of whether its own leg turns out to be
	// the debit or the credit) - avoiding an AB-BA deadlock against another
	// orchestration that touches the same two accounts in the opposite
	// role. Re-locking a row this same tx already holds is a no-op in
	// Postgres, so calling this before Debit/Credit on the same account is
	// always safe.
	LockForUpdate(ctx context.Context, tx *gorm.DB, accountID uuid.UUID) (domain.Account, error)
}
