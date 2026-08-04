package kapook

import (
	"context"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction"
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
	// UpdateAfterPurchase records a purchase's effect on the goal: SalakAmount
	// grows to newSalakAmount (SavingAmount is untouched - see
	// domain.Goal.AvailableBalance), and IsActive is set to stillActive
	// (false only once a purchase fully satisfies GoalAmount).
	UpdateAfterPurchase(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSalakAmount decimal.Decimal, stillActive bool) error
}

// TransactionRepository owns the kapook_transactions ledger of movements
// into/out of a Kapook (and Kapook-side bookkeeping for Salak purchases and
// expirations).
type TransactionRepository interface {
	Create(ctx context.Context, tx *gorm.DB, t *domain.Transaction) error
	// CountByGoalAndTypesInWindow counts goalID's existing transactions whose
	// Type is in types and CreatedAt falls in [from, to) - the free-
	// withdrawal allowance check. tx may be nil for an unlocked, read-only
	// count (e.g. a status preview); the authoritative check inside Withdraw
	// always passes the goal's already-locked tx so it can't race a
	// concurrent withdrawal on the same goal.
	CountByGoalAndTypesInWindow(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, types []domain.TransactionType, from, to time.Time) (int, error)
}

// WithdrawalStatus describes a goal's free-withdrawal allowance for the
// rolling 12-month window (see domain.WithdrawalWindow) containing "now".
// It's a read-time projection, not a persisted row - FreeWithdrawalsUsed is
// always recomputed from kapook_transactions, never stored.
type WithdrawalStatus struct {
	WindowStart              time.Time
	WindowEnd                time.Time
	FreeWithdrawalsUsed      int
	FreeWithdrawalsRemaining int
	NextWithdrawalIsFree     bool
}

// WithdrawResult is what Withdraw actually did, as opposed to
// WithdrawalStatus's forward-looking "what would the next one cost".
// FeeAmount is 0 when FeeCharged is false. NetCredited is what the savings
// account actually received (Amount minus FeeAmount) - the kapook balance
// and the goal's SavingAmount both drop by the full pre-fee Amount, since
// the fee is retained rather than left behind in the Kapook.
type WithdrawResult struct {
	Goal        domain.Goal
	Amount      decimal.Decimal
	FeeCharged  bool
	FeeAmount   decimal.Decimal
	NetCredited decimal.Decimal
}

// BuyFromGoalResult is what BuyFromGoal actually did. GoalCompleted mirrors
// !Goal.IsActive - kept as its own field so a caller doesn't have to infer
// "this purchase was the one that finished it" from state alone.
type BuyFromGoalResult struct {
	Goal          domain.Goal
	Receipt       transaction.BuySalakReceipt
	GoalCompleted bool
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
	// Withdraw debits kapookAccountID and credits savingsAccountID
	// atomically, for any amount up to the active goal's SavingAmount (no
	// minimum). The goal survives - IsActive is untouched even when the
	// withdrawal empties the balance. The first two withdrawals in the
	// goal's current rolling-12-month window are free; later ones in the
	// same window carry a 2% fee taken out of what reaches savings.
	Withdraw(ctx context.Context, userID, kapookAccountID, savingsAccountID uuid.UUID, amount decimal.Decimal) (WithdrawResult, error)
	// GetWithdrawalStatus previews the free/fee outcome a withdrawal would
	// have right now, without side effects - so a caller can warn the
	// customer before they commit. Returns apperror.NotFound if
	// kapookAccountID has no active goal.
	GetWithdrawalStatus(ctx context.Context, userID, kapookAccountID uuid.UUID) (WithdrawalStatus, error)
	// BuyFromGoal converts amount of the active goal's available balance
	// (SavingAmount minus SalakAmount) into the goal's own product, gated on
	// that balance being at least the product's minimum purchase and amount
	// being a valid step/max amount for it. A purchase that fully satisfies
	// GoalAmount deactivates the goal; a partial one leaves it active.
	BuyFromGoal(ctx context.Context, userID, kapookAccountID, salakAccountID uuid.UUID, amount decimal.Decimal) (BuyFromGoalResult, error)
}
