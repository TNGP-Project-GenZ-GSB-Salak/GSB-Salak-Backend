package transaction

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// LedgerRepository is implemented by the gorm repository and consumed by the service.
type LedgerRepository interface {
	Create(ctx context.Context, tx *gorm.DB, e *domain.LedgerEntry) error
	FindByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]domain.LedgerEntry, error)
}

// BuySalakReceipt is returned to the caller after a successful buy-salak transaction.
type BuySalakReceipt struct {
	ReferenceID                uuid.UUID
	ProductName                string
	Units                      int64
	TicketStart                string
	TicketEnd                  string
	Amount                     decimal.Decimal
	FundingAccountBalanceAfter decimal.Decimal
	SalakAccountBalanceAfter   decimal.Decimal
	PurchaseDate               string
	MaturityDate               string
}

// Service is the public surface the http layer depends on.
type Service interface {
	BuySalak(ctx context.Context, userID, fundingAccountID, salakAccountID, productID uuid.UUID, amount decimal.Decimal) (BuySalakReceipt, error)
	ListHistory(ctx context.Context, userID, accountID uuid.UUID, limit, offset int) ([]domain.LedgerEntry, error)
}
