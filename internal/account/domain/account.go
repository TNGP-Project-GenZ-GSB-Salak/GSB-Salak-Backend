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
	// TypeKapook is a real เผื่อเรียก deposit account, opened once per user
	// and reused for every goal. This is our own role label, not a mirror
	// of the bank's product code - the same way TypeSavings/TypeSalak are.
	TypeKapook Type = "kapook"
)

type Account struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	AccountNumber string
	Type          Type
	Balance       decimal.Decimal
	Currency      string
	// IsPrimaryAccount flags the บัญชีคู่โอน - at most one per user (a
	// partial unique index), and only ever true for a savings-type account
	// (a check constraint). Reach it only through account.Service -
	// GetPrimaryAccount for lookup, Create's isPrimary parameter for
	// provisioning - never read this field directly outside the domain.
	IsPrimaryAccount bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (Account) TableName() string {
	return "account.accounts"
}
