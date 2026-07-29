package repository

import (
	"context"
	"errors"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormAccountRepository struct {
	db *gorm.DB
}

func NewGormAccountRepository(db *gorm.DB) *GormAccountRepository {
	return &GormAccountRepository{db: db}
}

func (r *GormAccountRepository) Create(ctx context.Context, tx *gorm.DB, a *domain.Account) error {
	if tx == nil {
		tx = r.db
	}
	return tx.WithContext(ctx).Create(a).Error
}

func (r *GormAccountRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	var accounts []domain.Account
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").Find(&accounts).Error
	return accounts, err
}

func (r *GormAccountRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	var a domain.Account
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Account{}, err
	}
	return a, err
}

func (r *GormAccountRepository) FindForUpdate(ctx context.Context, tx *gorm.DB, id uuid.UUID) (domain.Account, error) {
	var a domain.Account
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
		First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Account{}, err
	}
	return a, err
}

func (r *GormAccountRepository) UpdateBalance(ctx context.Context, tx *gorm.DB, id uuid.UUID, newBalance decimal.Decimal) error {
	return tx.WithContext(ctx).
		Model(&domain.Account{}).
		Where("id = ?", id).
		Update("balance", newBalance).Error
}
