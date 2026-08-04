package repository

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormTermsRepository struct {
	db *gorm.DB
}

func NewGormTermsRepository(db *gorm.DB) *GormTermsRepository {
	return &GormTermsRepository{db: db}
}

// Accept is idempotent (ON CONFLICT DO NOTHING on the user_id unique
// constraint) - accepting twice never errors and never creates a second row.
func (r *GormTermsRepository) Accept(ctx context.Context, userID uuid.UUID) error {
	a := &domain.TermsAcceptance{ID: uuid.New(), UserID: userID}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoNothing: true,
		}).
		Create(a).Error
}

func (r *GormTermsRepository) HasAccepted(ctx context.Context, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.TermsAcceptance{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count > 0, err
}
