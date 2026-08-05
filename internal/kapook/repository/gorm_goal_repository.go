package repository

import (
	"context"
	"time"

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

func (r *GormGoalRepository) FindByID(ctx context.Context, goalID uuid.UUID) (domain.Goal, error) {
	var g domain.Goal
	err := r.db.WithContext(ctx).First(&g, "id = ?", goalID).Error
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

func (r *GormGoalRepository) UpdateAfterPurchase(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSalakAmount decimal.Decimal, stillActive bool) error {
	return tx.WithContext(ctx).
		Model(&domain.Goal{}).
		Where("id = ?", goalID).
		Updates(map[string]interface{}{
			"salak_amount":                 newSalakAmount,
			"is_active":                    stillActive,
			"auto_purchase_deferred_until": nil,
		}).Error
}

func (r *GormGoalRepository) UpdateAfterExpiration(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSalakAmount decimal.Decimal) error {
	return tx.WithContext(ctx).
		Model(&domain.Goal{}).
		Where("id = ?", goalID).
		Update("salak_amount", newSalakAmount).Error
}

func (r *GormGoalRepository) UpdateAfterWithdrawal(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, newSavingAmount decimal.Decimal, stillActive bool) error {
	return tx.WithContext(ctx).
		Model(&domain.Goal{}).
		Where("id = ?", goalID).
		Updates(map[string]interface{}{"saving_amount": newSavingAmount, "is_active": stillActive}).Error
}

func (r *GormGoalRepository) MarkGoalReached(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, reachedAt time.Time) error {
	return tx.WithContext(ctx).
		Model(&domain.Goal{}).
		Where("id = ?", goalID).
		Update("goal_reached_at", reachedAt).Error
}

func (r *GormGoalRepository) ClaimDueGoals(ctx context.Context, tx *gorm.DB, cutoff, today time.Time, limit int) ([]domain.Goal, error) {
	var goals []domain.Goal
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("is_active AND goal_reached_at IS NOT NULL AND goal_reached_at <= ?", cutoff).
		Where("auto_purchase_deferred_until IS NULL OR auto_purchase_deferred_until <= ?", today).
		Order("goal_reached_at ASC").
		Limit(limit).
		Find(&goals).Error
	if err != nil {
		return nil, err
	}
	return goals, nil
}

func (r *GormGoalRepository) SetAutoPurchaseDeferral(ctx context.Context, tx *gorm.DB, goalID uuid.UUID, until time.Time) error {
	return tx.WithContext(ctx).
		Model(&domain.Goal{}).
		Where("id = ?", goalID).
		Update("auto_purchase_deferred_until", until).Error
}
