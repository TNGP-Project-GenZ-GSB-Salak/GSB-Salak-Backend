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
// e.g. "ก0007530" - the letter is assigned deterministically by
// ReserveTicketRange's per-product cursor (see TicketSequence), never
// randomized, and a holding's range never crosses a letter boundary, so
// both ends always share h.TicketLetter. The zero-padded number is the
// raw ticket_start/ticket_end.
func (h Holding) TicketStartID() string {
	return h.TicketLetter + fmt.Sprintf("%07d", h.TicketStart)
}

func (h Holding) TicketEndID() string {
	return h.TicketLetter + fmt.Sprintf("%07d", h.TicketEnd)
}

// TicketSequence is one product's allocation cursor - (NextTicketLetter,
// NextTicketNumber) together identify the next ticket ReserveTicketRange
// will hand out for that product. One row per product (ProductID is the
// primary key), locked under SELECT ... FOR UPDATE so concurrent
// purchases of the same product serialize; different products never
// contend with each other. NextTicketNumber is bounded 0..9999999 (a
// 7-digit block, enforced by a CHECK) - once a product's current letter
// can't fit a purchase, the cursor skips to the next letter (see
// NextLetter) at number 0, rather than letting the number roll over
// unbounded the way the old global singleton counter did.
type TicketSequence struct {
	ProductID        uuid.UUID `gorm:"primaryKey"`
	NextTicketLetter string
	NextTicketNumber int64
	UpdatedAt        time.Time
}

func (TicketSequence) TableName() string {
	return "salak.ticket_sequence"
}
