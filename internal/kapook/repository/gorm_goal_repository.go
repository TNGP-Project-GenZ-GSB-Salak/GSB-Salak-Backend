package repository

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GormGoalRepository struct {
	db *gorm.DB
}

func NewGormGoalRepository(db *gorm.DB) *GormGoalRepository {
	return &GormGoalRepository{db: db}
}

func (r *GormGoalRepository) Create(ctx context.Context, g *domain.Goal) error {
	return r.db.WithContext(ctx).Create(g).Error
}

func (r *GormGoalRepository) FindActiveByAccountID(ctx context.Context, accountID uuid.UUID) (domain.Goal, error) {
	var g domain.Goal
	err := r.db.WithContext(ctx).
		Where("account_id = ? AND is_active", accountID).
		First(&g).Error
	if err != nil {
		return domain.Goal{}, err
	}
	return g, nil
}
