package repository

import (
	"context"
	"errors"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormDrawDateRepository struct {
	db *gorm.DB
}

func NewGormDrawDateRepository(db *gorm.DB) *GormDrawDateRepository {
	return &GormDrawDateRepository{db: db}
}

func (r *GormDrawDateRepository) IsDrawDay(ctx context.Context, productID uuid.UUID, date time.Time) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.DrawDate{}).
		Where("product_id = ? AND draw_date = ?", productID, date).
		Count(&count).Error
	return count > 0, err
}

func (r *GormDrawDateRepository) FurthestDrawDate(ctx context.Context, productID uuid.UUID) (time.Time, bool, error) {
	var d domain.DrawDate
	err := r.db.WithContext(ctx).
		Where("product_id = ?", productID).
		Order("draw_date DESC").
		First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return d.DrawDate, true, nil
}

// Create is idempotent (ON CONFLICT DO NOTHING on the (product_id,
// draw_date) unique constraint), matching ProductRepository.Upsert's
// safe-to-rerun style since cmd/seed calls this on every seed run.
func (r *GormDrawDateRepository) Create(ctx context.Context, d *domain.DrawDate) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "product_id"}, {Name: "draw_date"}},
			DoNothing: true,
		}).
		Create(d).Error
}
