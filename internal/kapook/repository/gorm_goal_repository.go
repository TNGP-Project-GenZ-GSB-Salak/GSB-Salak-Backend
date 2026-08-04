package repository

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *GormGoalRepository) FindActiveByAccountIDForUpdate(ctx context.Context, tx *gorm.DB, accountID uuid.UUID) (domain.Goal, error) {
	var g domain.Goal
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("account_id = ? AND is_active", accountID).
		First(&g).Error
	if err != nil {
		return domain.Goal{}, err
	}
	return g, nil
}

func (r *GormGoalRepository) UpdateSavingAmount(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSavingAmount decimal.Decimal) error {
	return tx.WithContext(ctx).
		Model(&domain.Goal{}).
		Where("id = ?", goalID).
		Update("saving_amount", newSavingAmount).Error
}
