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

func (r *GormTransactionRepository) SumPurchasedUnitsAndCount(ctx context.Context, tx *gorm.DB, goalID uuid.UUID) (int64, int, error) {
	if tx == nil {
		tx = r.db
	}
	var result struct {
		Units int64
		Count int
	}
	err := tx.WithContext(ctx).
		Table("kapook.kapook_transactions AS kt").
		Select("COALESCE(SUM(h.units), 0) AS units, COUNT(kt.id) AS count").
		Joins("JOIN salak.holdings h ON h.id = kt.holding_id").
		Where("kt.goal_id = ? AND kt.type = ?", goalID, domain.TransactionBuySalak).
		Scan(&result).Error
	return result.Units, result.Count, err
}
