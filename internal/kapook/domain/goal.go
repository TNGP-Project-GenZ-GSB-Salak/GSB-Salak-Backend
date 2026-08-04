package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Goal is a customer's Kapook savings goal. AccountID is NOT unique - one
// kapook account hosts many goals over its life, but at most one may be
// active, enforced by a partial unique index rather than a plain UNIQUE.
//
// A goal ends in exactly two ways: a purchase that fully satisfies
// GoalAmount, or (slice 09) a full withdrawal during a live countdown.
// There is no cancel path - a mis-set GoalAmount is permanent, an accepted
// risk recorded deliberately in the spec.
type Goal struct {
	ID            uuid.UUID
	AccountID     uuid.UUID
	ProductID     uuid.UUID
	GoalAmount    decimal.Decimal
	SavingAmount  decimal.Decimal
	SalakAmount   decimal.Decimal
	IsActive      bool
	GoalReachedAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (Goal) TableName() string {
	return "kapook.kapook_goals"
}

// AvailableBalance is the cash still sitting in the kapook, not yet
// converted to Salak - what a withdrawal or a purchase can actually draw
// on. SavingAmount tracks total net contribution (deposits minus
// withdrawals) regardless of form; a purchase moves value from cash to
// Salak without changing that total, so it grows SalakAmount rather than
// shrinking SavingAmount. This is also the invariant behind the
// `salak_amount <= saving_amount` CHECK constraint: it only holds if
// purchases leave SavingAmount alone.
func (g Goal) AvailableBalance() decimal.Decimal {
	return g.SavingAmount.Sub(g.SalakAmount)
}
