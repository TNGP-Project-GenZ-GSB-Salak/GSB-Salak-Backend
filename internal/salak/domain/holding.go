package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Holding struct {
	ID             uuid.UUID
	AccountID      uuid.UUID
	ProductID      uuid.UUID
	Units          int64
	TicketLetter   string
	TicketStart    int64
	TicketEnd      int64
	PurchaseAmount decimal.Decimal
	PurchaseDate   time.Time
	MaturityDate   time.Time
	// SettledAt is nil until SettleMaturedHolding has paid out this
	// holding's principal + interest - the idempotency guard against
	// settling the same holding twice, and the eventual claim predicate
	// for an automated worker, if one is ever added.
	SettledAt *time.Time
	CreatedAt time.Time
}

func (Holding) TableName() string {
	return "salak.holdings"
}

// TicketStartID and TicketEndID render the full displayed ticket ID,
// e.g. "ก0007530" - the letter is randomized once per holding, the
// zero-padded number is the raw ticket_start/ticket_end.
func (h Holding) TicketStartID() string {
	return h.TicketLetter + fmt.Sprintf("%07d", h.TicketStart)
}

func (h Holding) TicketEndID() string {
	return h.TicketLetter + fmt.Sprintf("%07d", h.TicketEnd)
}

// TicketSequence is the singleton row used to atomically reserve
// contiguous ticket-number ranges under a row lock.
type TicketSequence struct {
	ID               int `gorm:"primaryKey"`
	NextTicketNumber int64
	UpdatedAt        time.Time
}

func (TicketSequence) TableName() string {
	return "salak.ticket_sequence"
}
