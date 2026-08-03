package repository

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/badge/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GormBadgeRepository struct {
	db *gorm.DB
}

func NewGormBadgeRepository(db *gorm.DB) *GormBadgeRepository {
	return &GormBadgeRepository{db: db}
}

func (r *GormBadgeRepository) UserOwnsBadge(ctx context.Context, userID, badgeID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.UserBadge{}).
		Where("user_id = ? AND badge_id = ?", userID, badgeID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
