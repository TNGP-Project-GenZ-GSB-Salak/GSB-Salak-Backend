package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Product struct {
	ID          uuid.UUID
	Code        string
	Name        string
	TermMonths  int
	UnitPrice   decimal.Decimal
	MinPurchase decimal.Decimal
	MaxPurchase decimal.Decimal
	StepAmount  decimal.Decimal
	IsActive    bool
	// MaturityInterestPerUnit is the baht payout per unit at maturity - the
	// sales sheets lead with this figure directly, so interest = units *
	// MaturityInterestPerUnit is exact with no rounding decision needed
	// (see migration 000020).
	MaturityInterestPerUnit decimal.Decimal
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func (Product) TableName() string {
	return "salak.products"
}
