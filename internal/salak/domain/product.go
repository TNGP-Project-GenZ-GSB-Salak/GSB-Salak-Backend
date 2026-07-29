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
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Product) TableName() string {
	return "salak.products"
}
