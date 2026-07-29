package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Holding struct {
	ID             uuid.UUID
	AccountID      uuid.UUID
	ProductID      uuid.UUID
	Units          int64
	TicketStart    int64
	TicketEnd      int64
	PurchaseAmount decimal.Decimal
	PurchaseDate   time.Time
	MaturityDate   time.Time
	CreatedAt      time.Time
}

func (Holding) TableName() string {
	return "salak.holdings"
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
