package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type TransactionType string

// Five values, not six - an exempt-window bail-out withdrawal was
// considered and dropped, since it carries no fee or quota exemption.
const (
	TransactionDeposit         TransactionType = "deposit"
	TransactionWithdraw        TransactionType = "withdraw"
	TransactionWithdrawWithFee TransactionType = "withdraw_with_fee"
	TransactionBuySalak        TransactionType = "buy_salak"
	TransactionSalakExpiration TransactionType = "salak_expiration"
)

// Transaction is one movement of money into or out of a Kapook, or Kapook-
// side bookkeeping for a Salak purchase/expiration. SavingsAccountID and
// HoldingID are nullable since no single Type uses both - deposit/withdraw/
// withdraw_with_fee set SavingsAccountID only, buy_salak/salak_expiration
// set HoldingID only. GoalID is which goal the movement belongs to - needed
// because one kapook account hosts many goals over its life, so counting a
// goal's free-withdrawal allowance by KapookAccountID alone would conflate
// withdrawals made against a different, earlier goal on the same account.
type Transaction struct {
	ID               uuid.UUID
	Type             TransactionType
	Amount           decimal.Decimal
	KapookAccountID  uuid.UUID
	GoalID           uuid.UUID
	SavingsAccountID *uuid.UUID
	HoldingID        *uuid.UUID
	// IsAutomaticPurchase is nil for every row except a buy_salak one the
	// worker performed unattended - ticket 09's expand step, populated by
	// ticket 10's worker; nothing sets it yet.
	IsAutomaticPurchase *bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (Transaction) TableName() string {
	return "kapook.kapook_transactions"
}
