package salak

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ProductRepository is implemented by the gorm repository and consumed by the service.
type ProductRepository interface {
	ListActive(ctx context.Context) ([]domain.Product, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.Product, error)
	Upsert(ctx context.Context, p *domain.Product) error
}

// HoldingRepository owns holding persistence and the ticket-sequence counter.
type HoldingRepository interface {
	Create(ctx context.Context, tx *gorm.DB, h *domain.Holding) error
	FindByAccountID(ctx context.Context, accountID uuid.UUID) ([]domain.Holding, error)
	// ReserveTicketRange atomically reserves `units` contiguous ticket numbers
	// under a row lock on the ticket_sequence singleton, returning [start, end].
	ReserveTicketRange(ctx context.Context, tx *gorm.DB, units int64) (start, end int64, err error)
}

// Service is the public surface the transaction domain (and http layer) depend on.
type Service interface {
	ListProducts(ctx context.Context) ([]domain.Product, error)
	GetProduct(ctx context.Context, productID uuid.UUID) (domain.Product, error)
	ValidatePurchase(product domain.Product, amount decimal.Decimal) error
	MintHolding(ctx context.Context, tx *gorm.DB, accountID, productID uuid.UUID, amount decimal.Decimal) (domain.Holding, error)
}
