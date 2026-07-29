package repository

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GormLedgerRepository struct {
	db *gorm.DB
}

func NewGormLedgerRepository(db *gorm.DB) *GormLedgerRepository {
	return &GormLedgerRepository{db: db}
}

func (r *GormLedgerRepository) Create(ctx context.Context, tx *gorm.DB, e *domain.LedgerEntry) error {
	if tx == nil {
		tx = r.db
	}
	return tx.WithContext(ctx).Create(e).Error
}

func (r *GormLedgerRepository) FindByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]domain.LedgerEntry, error) {
	var entries []domain.LedgerEntry
	err := r.db.WithContext(ctx).
		Where("account_id = ?", accountID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&entries).Error
	return entries, err
}
