package repository

import (
	"context"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ticketsPerLetter is a letter's block capacity - 0..9999999 inclusive,
// the 7-digit display width. Fixed by docs/GAPS.md §2.3, not configurable:
// changing it would silently reinterpret every already-issued ticket
// number's letter boundary.
const ticketsPerLetter = 10_000_000

type GormHoldingRepository struct {
	db *gorm.DB
}

func NewGormHoldingRepository(db *gorm.DB) *GormHoldingRepository {
	return &GormHoldingRepository{db: db}
}

func (r *GormHoldingRepository) Create(ctx context.Context, tx *gorm.DB, h *domain.Holding) error {
	if tx == nil {
		tx = r.db
	}
	return tx.WithContext(ctx).Create(h).Error
}

// FindByAccountID excludes settled holdings - once SettleMaturedHolding pays
// one out, its value is already gone from the account's own balance, so
// listing it alongside still-live holdings would show a ticket whose money
// isn't there anymore. This is the only caller of this query today (the
// customer-facing holdings list); a future "matured/redeemed history" view
// would need its own method rather than relaxing this filter.
func (r *GormHoldingRepository) FindByAccountID(ctx context.Context, accountID uuid.UUID) ([]domain.Holding, error) {
	var holdings []domain.Holding
	err := r.db.WithContext(ctx).
		Where("account_id = ? AND settled_at IS NULL", accountID).
		Order("purchase_date DESC").
		Find(&holdings).Error
	return holdings, err
}

func (r *GormHoldingRepository) ReserveTicketRange(ctx context.Context, tx *gorm.DB, productID uuid.UUID, units int64) (string, int64, int64, error) {
	if units > ticketsPerLetter {
		return "", 0, 0, salak.ErrUnitsExceedLetterCapacity
	}

	var seq domain.TicketSequence
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("product_id = ?", productID).
		First(&seq).Error; err != nil {
		return "", 0, 0, err
	}

	letter := seq.NextTicketLetter
	number := seq.NextTicketNumber

	// The range never crosses a letter boundary: if this letter's block
	// doesn't have room for the whole purchase, the entire reservation
	// moves to the next letter's 0 instead of splitting - abandoning that
	// block's leftover tail (at most units-1 tickets, once per 10M).
	if remaining := ticketsPerLetter - number; units > remaining {
		next, err := domain.NextLetter(letter)
		if err != nil {
			return "", 0, 0, err
		}
		letter = next
		number = 0
	}

	start := number
	end := start + units - 1

	if err := tx.WithContext(ctx).
		Model(&domain.TicketSequence{}).
		Where("product_id = ?", productID).
		Updates(map[string]interface{}{
			"next_ticket_letter": letter,
			"next_ticket_number": end + 1,
		}).Error; err != nil {
		return "", 0, 0, err
	}

	return letter, start, end, nil
}

func (r *GormHoldingRepository) FindForUpdate(ctx context.Context, tx *gorm.DB, id uuid.UUID) (domain.Holding, error) {
	var h domain.Holding
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
		First(&h).Error
	return h, err
}

func (r *GormHoldingRepository) MarkSettled(ctx context.Context, tx *gorm.DB, id uuid.UUID, settledAt time.Time) error {
	return tx.WithContext(ctx).
		Model(&domain.Holding{}).
		Where("id = ?", id).
		Update("settled_at", settledAt).Error
}
