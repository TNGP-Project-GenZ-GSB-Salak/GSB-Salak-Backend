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
// set HoldingID only.
type Transaction struct {
	ID               uuid.UUID
	Type             TransactionType
	Amount           decimal.Decimal
	KapookAccountID  uuid.UUID
	SavingsAccountID *uuid.UUID
	HoldingID        *uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (Transaction) TableName() string {
	return "kapook.kapook_transactions"
}
