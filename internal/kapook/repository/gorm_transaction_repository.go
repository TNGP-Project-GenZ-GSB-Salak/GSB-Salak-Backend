package repository

import (
	"context"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GormTransactionRepository struct {
	db *gorm.DB
}

func NewGormTransactionRepository(db *gorm.DB) *GormTransactionRepository {
	return &GormTransactionRepository{db: db}
}

func (r *GormTransactionRepository) Create(ctx context.Context, tx *gorm.DB, t *domain.Transaction) error {
	if tx == nil {
		tx = r.db
	}
	return tx.WithContext(ctx).Create(t).Error
}

func (r *GormTransactionRepository) CountByGoalAndTypesInWindow(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, types []domain.TransactionType, from, to time.Time) (int, error) {
	if tx == nil {
		tx = r.db
	}
	var count int64
	err := tx.WithContext(ctx).
		Model(&domain.Transaction{}).
		Where("goal_id = ? AND type IN ? AND created_at >= ? AND created_at < ?", goalID, types, from, to).
		Count(&count).Error
	return int(count), err
}
