package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Type string

const (
	TypeSavings Type = "savings"
	TypeSalak   Type = "salak"
)

type Account struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	AccountNumber string
	Type          Type
	Balance       decimal.Decimal
	Currency      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (Account) TableName() string {
	return "account.accounts"
}
