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
	// UpdateSavingAmount is Deposit's write path: SavingAmount grows,
	// nothing else changes.
	UpdateSavingAmount(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSavingAmount decimal.Decimal) error
	// UpdateAfterPurchase records a purchase's effect on the goal: SalakAmount
	// grows to newSalakAmount (SavingAmount is untouched - see
	// domain.Goal.AvailableBalance), and IsActive is set to stillActive
	// (false only once a purchase fully satisfies GoalAmount).
	UpdateAfterPurchase(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSalakAmount decimal.Decimal, stillActive bool) error
	// UpdateAfterWithdrawal is Withdraw's write path: SavingAmount shrinks to
	// newSavingAmount, and IsActive is set to stillActive - false only for
	// the all-or-nothing full withdrawal that closes a goal during its live
	// countdown; every other withdrawal passes stillActive true.
	UpdateAfterWithdrawal(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSavingAmount decimal.Decimal, stillActive bool) error
	// MarkGoalReached stamps GoalReachedAt the moment a deposit first brings
	// SavingAmount up to GoalAmount - starting the auto-purchase countdown.
	MarkGoalReached(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, reachedAt time.Time) error
	// ClaimDueGoals locks and returns up to limit active goals whose
	// GoalReachedAt is at or before cutoff (i.e. cutoff = now minus the
	// countdown duration), via SELECT ... FOR UPDATE SKIP LOCKED - so
	// concurrent worker passes (different ticks overlapping, or multiple
	// replicas) each claim a disjoint subset instead of blocking on each
	// other. Requires a real tx; every claimed row stays locked until the
	// caller's transaction ends.
	ClaimDueGoals(ctx context.Context, tx *gorm.DB, cutoff time.Time, limit int) ([]domain.Goal, error)
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
	// SumPurchasedUnitsAndCount aggregates goalID's buy_salak transactions,
	// joined through their holding_id to salak.holdings for the unit count -
	// derived at read time rather than stored, so it's correct whether the
	// customer or the worker bought, with nothing to keep in sync. tx may be
	// nil for the ambient handle; pass the caller's own tx/savepoint when the
	// purchase being counted was written earlier in the same transaction.
	SumPurchasedUnitsAndCount(ctx context.Context, tx *gorm.DB, goalID uuid.UUID) (units int64, count int, err error)
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
	// Quote is non-nil only when GetWithdrawalStatus was called with a
	// candidate amount - the fee/net a withdrawal of that amount would
	// incur right now, from the same computation Withdraw itself uses.
	Quote *WithdrawalQuote
}

// WithdrawResult is what Withdraw actually did, as opposed to
// WithdrawalStatus's forward-looking "what would the next one cost".
// FeeAmount is 0 when FeeCharged is false. NetCredited is what the savings
// account actually received (Amount minus FeeAmount) - the kapook balance
// and the goal's SavingAmount both drop by the full pre-fee Amount, since
// the fee is retained rather than left behind in the Kapook. GoalClosed is
// true only for the all-or-nothing full withdrawal that walks away from a
// live countdown; every other withdrawal leaves the goal open.
// WithdrawalQuote is what GetWithdrawalStatus's optional amount preview
// returns - fee/net for a candidate amount, computed by the exact same
// computeWithdrawalFee helper Withdraw itself uses, so a quote can never
// disagree with what Withdraw actually charges. Both fields together
// (rather than a single struct pointer) would fine too; kept as one pointer
// so "no amount was requested" round-trips as one nil check.
type WithdrawalQuote struct {
	FeeAmount decimal.Decimal
	NetAmount decimal.Decimal
}

type WithdrawResult struct {
	Goal        domain.Goal
	Amount      decimal.Decimal
	FeeCharged  bool
	FeeAmount   decimal.Decimal
	NetCredited decimal.Decimal
	GoalClosed  bool
}

// BuyFromGoalResult is what BuyFromGoal actually did. GoalCompleted mirrors
// !Goal.IsActive - kept as its own field so a caller doesn't have to infer
// "this purchase was the one that finished it" from state alone.
type BuyFromGoalResult struct {
	Goal          domain.Goal
	Receipt       transaction.BuySalakReceipt
	GoalCompleted bool
}

// GoalSnapshot enriches a Goal with the read model the client's tracker
// screen needs - available balance, target-reached, live countdown,
// purchased units/count, and buy eligibility - all computed fresh at read
// time rather than stored, so it's correct regardless of who or what last
// touched the goal (the customer, another device, or the unattended
// worker). See KapookService.Snapshot.
type GoalSnapshot struct {
	Goal domain.Goal
	// AvailableBalance is Goal.AvailableBalance() (SavingAmount minus
	// SalakAmount) - what a withdrawal or purchase can still draw on.
	AvailableBalance decimal.Decimal
	// TargetReached mirrors Goal.GoalReachedAt != nil.
	TargetReached bool
	// CountdownRemainingSeconds is nil unless TargetReached - the seconds
	// left in the auto-purchase countdown, clamped to zero rather than
	// going negative once it expires (the worker, not the client, decides
	// when a purchase actually fires).
	CountdownRemainingSeconds *int
	// PurchasedUnits/PurchasedCount are derived from the goal's buy_salak
	// transaction history (TransactionRepository.SumPurchasedUnitsAndCount),
	// never stored on the goal itself.
	PurchasedUnits int64
	PurchasedCount int
	// BuyEligible mirrors the same rule BuyFromGoal enforces on submit:
	// AvailableBalance at least the product's MinPurchase.
	BuyEligible bool
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
	// GetActiveGoal returns (nil, nil) if accountID has no active goal - a
	// normal state for the tracker's empty-state screen, not an error.
	// Ownership failures (wrong user, wrong account type) still return
	// apperror.NotFound, indistinguishable from a real not-found on purpose.
	GetActiveGoal(ctx context.Context, userID, accountID uuid.UUID) (*domain.Goal, error)
	// Deposit debits savingsAccountID and credits kapookAccountID
	// atomically, then bumps the account's active goal's SavingAmount -
	// rejected if that would exceed the goal's target. Any positive
	// amount is accepted, no minimum. The instant a deposit first brings
	// SavingAmount up to GoalAmount, GoalReachedAt is stamped, starting the
	// auto-purchase countdown (see the worker package).
	Deposit(ctx context.Context, userID, kapookAccountID, savingsAccountID uuid.UUID, amount decimal.Decimal) (domain.Goal, error)
	// Withdraw debits kapookAccountID and credits the customer's primary
	// account (account.Service.GetPrimaryAccount - the บัญชีคู่โอน),
	// atomically, for any amount up to the active goal's AvailableBalance
	// (no minimum). The destination is never customer-chosen: a customer
	// with no primary account flagged fails loudly with
	// kapook.CodeNoPrimaryAccount rather than guessing one. Once
	// GoalReachedAt is set (a countdown is live), withdrawal becomes
	// all-or-nothing: a partial amount is rejected, and withdrawing the
	// full balance closes the goal instead of leaving it active - the
	// customer's escape from the countdown. Before the goal is reached, the
	// goal always survives, even a withdrawal that empties it. The first
	// two withdrawals in the goal's current rolling-12-month window are
	// free regardless of which case this is; later ones in the same window
	// carry a 2% fee, rounded to two decimal places, taken out of what
	// reaches savings.
	Withdraw(ctx context.Context, userID, kapookAccountID uuid.UUID, amount decimal.Decimal) (WithdrawResult, error)
	// GetWithdrawalStatus previews the free/fee outcome a withdrawal would
	// have right now, without side effects - so a caller can warn the
	// customer before they commit. Returns apperror.NotFound if
	// kapookAccountID has no active goal. amount is optional (nil skips the
	// quote): when given, the returned WithdrawalStatus.Quote reports the
	// fee/net that amount would incur if withdrawn right now, computed by
	// the exact same logic Withdraw itself uses.
	GetWithdrawalStatus(ctx context.Context, userID, kapookAccountID uuid.UUID, amount *decimal.Decimal) (WithdrawalStatus, error)
	// BuyFromGoal converts amount of the active goal's available balance
	// (SavingAmount minus SalakAmount) into the goal's own product, gated on
	// that balance being at least the product's minimum purchase and amount
	// being a valid step/max amount for it. A purchase that fully satisfies
	// GoalAmount deactivates the goal; a partial one leaves it active.
	BuyFromGoal(ctx context.Context, userID, kapookAccountID, salakAccountID uuid.UUID, amount decimal.Decimal) (BuyFromGoalResult, error)
	// BuyFromGoalInTx is BuyFromGoal's tx-supplied variant, for the kapook
	// worker only: it runs the same validation and purchase inside a
	// savepoint on the caller's own already-open tx (via tx.Transaction),
	// rather than opening a new top-level one, so one goal's purchase
	// failure rolls back to the savepoint without losing the caller's own
	// transaction or its row locks on other claimed goals.
	BuyFromGoalInTx(ctx context.Context, tx *gorm.DB, userID, kapookAccountID, salakAccountID uuid.UUID, amount decimal.Decimal) (BuyFromGoalResult, error)
	// Snapshot computes GoalSnapshot's derived fields for goal - called
	// after CreateGoal/GetActiveGoal/Deposit/Withdraw/BuyFromGoal return, so
	// every goal-shaped response is enriched the same way from one place.
	Snapshot(ctx context.Context, goal domain.Goal) (GoalSnapshot, error)
}
