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
	HoldingID                  uuid.UUID
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

// Service is the public surface the http layer depends on - except
// BuySalakForKapook, which exists only for the kapook domain's own
// orchestration (see its doc comment) and is never reachable from an HTTP
// route.
type Service interface {
	// badgeID is optional (nil means no badge supplied); when non-nil, the
	// caller must own that badge or the purchase is rejected. fundingAccountID
	// must be a savings-type account - this is the public path and stays
	// closed to a kapook-type account; BuySalakForKapook is the only door in
	// for those.
	BuySalak(ctx context.Context, userID, fundingAccountID, salakAccountID, productID uuid.UUID, badgeID *uuid.UUID, amount decimal.Decimal) (BuySalakReceipt, error)
	// BuySalakForKapook is the Kapook-callable variant used only by the
	// kapook domain's own orchestration: it permits kapookAccountID to be a
	// kapook-type account (BuySalak rejects one), and it runs inside tx
	// rather than opening its own, so the caller can span this purchase and
	// its own goal-state update in one atomic unit. It shares BuySalak's
	// validation and money-movement logic (including the draw-day guard and
	// the debit-before-credit lock order) and writes the same single
	// debit+credit ledger pair - no second pair, no badge support (Kapook
	// purchases don't carry a badge choice).
	BuySalakForKapook(ctx context.Context, tx *gorm.DB, userID, kapookAccountID, salakAccountID, productID uuid.UUID, amount decimal.Decimal) (BuySalakReceipt, error)
	ListHistory(ctx context.Context, userID, accountID uuid.UUID, limit, offset int) ([]domain.LedgerEntry, error)
}
