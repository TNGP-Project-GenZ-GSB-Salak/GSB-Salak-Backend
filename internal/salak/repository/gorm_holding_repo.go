package repository

import (
	"context"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

func (r *GormHoldingRepository) ReserveTicketRange(ctx context.Context, tx *gorm.DB, units int64) (int64, int64, error) {
	var seq domain.TicketSequence
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", 1).
		First(&seq).Error; err != nil {
		return 0, 0, err
	}

	start := seq.NextTicketNumber
	end := start + units - 1

	if err := tx.WithContext(ctx).
		Model(&domain.TicketSequence{}).
		Where("id = ?", 1).
		Update("next_ticket_number", end+1).Error; err != nil {
		return 0, 0, err
	}

	return start, end, nil
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
