package repository

import (
	"context"
	"errors"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormProductRepository struct {
	db *gorm.DB
}

func NewGormProductRepository(db *gorm.DB) *GormProductRepository {
	return &GormProductRepository{db: db}
}

func (r *GormProductRepository) ListActive(ctx context.Context) ([]domain.Product, error) {
	var products []domain.Product
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Order("term_months ASC").Find(&products).Error
	return products, err
}

func (r *GormProductRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Product, error) {
	var p domain.Product
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Product{}, err
	}
	return p, err
}

func (r *GormProductRepository) Upsert(ctx context.Context, p *domain.Product) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "term_months", "unit_price", "min_purchase", "max_purchase", "step_amount", "is_active", "updated_at"}),
		}).
		Create(p).Error
}
