package kapook

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// TermsRepository owns the one-row-per-user terms & conditions acceptance
// record.
type TermsRepository interface {
	// Accept records userID's acceptance, idempotently - accepting twice
	// never errors and never creates a second row.
	Accept(ctx context.Context, userID uuid.UUID) error
	HasAccepted(ctx context.Context, userID uuid.UUID) (bool, error)
}

// GoalRepository owns kapook goal persistence. AccountID is not unique - an
// account may have many goals over its life - so the "at most one active"
// rule lives in a partial unique index, not here.
type GoalRepository interface {
	Create(ctx context.Context, g *domain.Goal) error
	// FindActiveByAccountID returns gorm.ErrRecordNotFound if accountID has
	// no active goal.
	FindActiveByAccountID(ctx context.Context, accountID uuid.UUID) (domain.Goal, error)
	// FindActiveByAccountIDForUpdate is FindActiveByAccountID under
	// SELECT ... FOR UPDATE, so a caller can read-then-write SavingAmount
	// (e.g. a deposit) without losing a concurrent update. Requires a real
	// tx and must not fall back to an ambient handle.
	FindActiveByAccountIDForUpdate(ctx context.Context, tx *gorm.DB, accountID uuid.UUID) (domain.Goal, error)
	UpdateSavingAmount(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSavingAmount decimal.Decimal) error
}

// TransactionRepository owns the kapook_transactions ledger of movements
// into/out of a Kapook (and Kapook-side bookkeeping for Salak purchases and
// expirations).
type TransactionRepository interface {
	Create(ctx context.Context, tx *gorm.DB, t *domain.Transaction) error
}

// Service is the public surface the http layer depends on.
type Service interface {
	Accept(ctx context.Context, userID uuid.UUID) error
	HasAccepted(ctx context.Context, userID uuid.UUID) (bool, error)
	// CreateGoal validates accountID is a kapook account owned by userID,
	// that userID has accepted the terms, that no other goal on that
	// account is active, and that goalAmount is a valid eventual purchase
	// amount for productID (a multiple of its step, at or below its max).
	CreateGoal(ctx context.Context, userID, accountID, productID uuid.UUID, goalAmount decimal.Decimal) (domain.Goal, error)
	// GetActiveGoal returns apperror.NotFound if accountID has no active goal.
	GetActiveGoal(ctx context.Context, userID, accountID uuid.UUID) (domain.Goal, error)
	// Deposit debits savingsAccountID and credits kapookAccountID
	// atomically, then bumps the account's active goal's SavingAmount -
	// rejected if that would exceed the goal's target. Any positive
	// amount is accepted, no minimum.
	Deposit(ctx context.Context, userID, kapookAccountID, savingsAccountID uuid.UUID, amount decimal.Decimal) (domain.Goal, error)
}
