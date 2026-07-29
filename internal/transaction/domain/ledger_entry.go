package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type EntryType string

const (
	EntryDebit  EntryType = "debit"
	EntryCredit EntryType = "credit"
)

type LedgerEntry struct {
	ID            uuid.UUID
	AccountID     uuid.UUID
	HoldingID     *uuid.UUID
	Type          EntryType
	Amount        decimal.Decimal
	BalanceAfter  decimal.Decimal
	ReferenceType string
	ReferenceID   uuid.UUID
	Description   string
	CreatedAt     time.Time
}

func (LedgerEntry) TableName() string {
	return "transaction.ledger_entries"
}
