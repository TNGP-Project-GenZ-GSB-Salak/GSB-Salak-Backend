package salak

import (
	"context"
	"errors"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ErrDrawDay is the sentinel a draw-day rejection wraps, so a caller (the
// future worker in particular) can tell "draw day, retry later" apart from
// a fatal failure like insufficient funds via errors.Is, regardless of the
// apperror.Kind - both are KindValidation, since both are the customer's
// input being currently unactionable, not a system fault.
var ErrDrawDay = errors.New("salak: product is closed for purchases on this draw day")

// ErrUnitsExceedLetterCapacity is what ReserveTicketRange returns when
// units alone can never fit in a single letter's 10,000,000-number block,
// so no rollover could ever satisfy the request - the service layer turns
// this into apperror.Validation.
var ErrUnitsExceedLetterCapacity = errors.New("salak: units exceeds a single letter's ticket capacity")

// ProductRepository is implemented by the gorm repository and consumed by the service.
type ProductRepository interface {
	ListActive(ctx context.Context) ([]domain.Product, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.Product, error)
	// FindByCode looks up a product by its unique code. Upsert doesn't return
	// the row it kept on a conflict-update path, so this is how a caller
	// (cmd/seed, to attach draw dates to the right product) gets the real ID.
	FindByCode(ctx context.Context, code string) (domain.Product, error)
	Upsert(ctx context.Context, p *domain.Product) error
}

// DrawDateRepository owns the draw-date calendar - the explicit days a
// product cannot be purchased on.
type DrawDateRepository interface {
	IsDrawDay(ctx context.Context, productID uuid.UUID, date time.Time) (bool, error)
	Create(ctx context.Context, d *domain.DrawDate) error
	// FurthestDrawDate returns the latest seeded draw_date for productID,
	// and false if none exist. Lets a startup check catch a calendar
	// that's about to run out - an exhausted table fails open (it just
	// stops blocking anything, silently), which is the wrong direction
	// for a compliance rule.
	FurthestDrawDate(ctx context.Context, productID uuid.UUID) (date time.Time, ok bool, err error)
}

// HoldingRepository owns holding persistence and the ticket-sequence counter.
type HoldingRepository interface {
	Create(ctx context.Context, tx *gorm.DB, h *domain.Holding) error
	FindByAccountID(ctx context.Context, accountID uuid.UUID) ([]domain.Holding, error)
	// ReserveTicketRange atomically reserves `units` contiguous ticket
	// numbers for productID under a row lock on that product's own
	// ticket_sequence cursor row, returning the letter and [start, end].
	// The range never crosses a letter boundary: if the current letter's
	// 10,000,000-number block doesn't have room left, the whole
	// reservation moves to the next letter's 0, abandoning that block's
	// leftover tail rather than splitting the purchase. Rejects units
	// greater than a single letter's capacity (10,000,000) up front, since
	// no letter block could ever satisfy it.
	ReserveTicketRange(ctx context.Context, tx *gorm.DB, productID uuid.UUID, units int64) (letter string, start, end int64, err error)
	// FindForUpdate locks and returns holdingID - the settlement service's
	// idempotency check (SettledAt != nil) and its money movement must see
	// a consistent row, so this is always called from inside a tx.
	FindForUpdate(ctx context.Context, tx *gorm.DB, id uuid.UUID) (domain.Holding, error)
	// MarkSettled records that a holding's principal + interest have been
	// paid out. Re-marking an already-settled holding never happens - the
	// caller checks SettledAt from the same locked row first.
	MarkSettled(ctx context.Context, tx *gorm.DB, id uuid.UUID, settledAt time.Time) error
}

// Service is the public surface the transaction domain (and http layer) depend on.
type Service interface {
	ListProducts(ctx context.Context) ([]domain.Product, error)
	GetProduct(ctx context.Context, productID uuid.UUID) (domain.Product, error)
	ValidatePurchase(product domain.Product, amount decimal.Decimal) error
	// EnsureNotDrawDay rejects a purchase falling on product's draw day,
	// wrapping ErrDrawDay. Called from transaction.BuySalak's stage-1
	// validation, covering the public endpoint, Kapook and the worker from
	// one site.
	EnsureNotDrawDay(ctx context.Context, product domain.Product) error
	// NextAvailableDate finds the earliest date strictly after today that
	// isn't product's draw day - what a caller blocked by EnsureNotDrawDay
	// (the Kapook worker, to persist a deferred goal's retry date) uses to
	// tell the customer when the purchase will actually happen.
	NextAvailableDate(ctx context.Context, product domain.Product) (time.Time, error)
	MintHolding(ctx context.Context, tx *gorm.DB, accountID, productID uuid.UUID, amount decimal.Decimal) (domain.Holding, error)
	// ListHoldingsByAccount verifies userID owns accountID (via account.Service)
	// before returning that account's holdings.
	ListHoldingsByAccount(ctx context.Context, userID, accountID uuid.UUID) ([]domain.Holding, error)
	// FindHoldingForUpdate and MarkHoldingSettled are the settlement
	// service's (internal/transaction) only way to touch a holding's
	// SettledAt state - kept on Service rather than exposing
	// HoldingRepository itself cross-domain, matching how every other
	// cross-domain caller only ever depends on salak.Service.
	FindHoldingForUpdate(ctx context.Context, tx *gorm.DB, id uuid.UUID) (domain.Holding, error)
	MarkHoldingSettled(ctx context.Context, tx *gorm.DB, id uuid.UUID, settledAt time.Time) error
}
