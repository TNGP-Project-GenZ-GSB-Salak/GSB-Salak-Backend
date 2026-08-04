package repository

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
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
