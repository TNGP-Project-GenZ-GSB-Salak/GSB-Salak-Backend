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

func (r *GormProductRepository) FindByCode(ctx context.Context, code string) (domain.Product, error) {
	var p domain.Product
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Product{}, err
	}
	return p, err
}

// Upsert opens its own transaction - it takes no tx parameter, so this is
// the only way to keep the product write and its ticket_sequence cursor
// row atomic. There is exactly one product-creation path today
// (cmd/seed), and the failure mode if the cursor row were ever forgotten
// is a customer's first purchase of that product failing, late and
// user-facing - putting provisioning here means any future caller (an
// admin endpoint, a fixture, a second seeder) gets it for free.
func (r *GormProductRepository) Upsert(ctx context.Context, p *domain.Product) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "code"}},
				DoUpdates: clause.AssignmentColumns([]string{"name", "term_months", "unit_price", "min_purchase", "max_purchase", "step_amount", "maturity_interest_per_unit", "is_active", "updated_at"}),
			}).
			Create(p).Error; err != nil {
			return err
		}

		// p.ID doesn't reflect the pre-existing row's real id on a
		// conflict-update path (see FindByCode's doc comment on
		// ports.go's ProductRepository) - re-read the actual row by code
		// so the cursor is provisioned against the real product_id, never
		// a throwaway id that was never persisted.
		var real domain.Product
		if err := tx.WithContext(ctx).Where("code = ?", p.Code).First(&real).Error; err != nil {
			return err
		}

		// ON CONFLICT DO NOTHING keyed on product_id makes this idempotent
		// across repeated seed runs - re-upserting an existing product
		// never resets an already-advanced cursor.
		return tx.WithContext(ctx).
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "product_id"}},
				DoNothing: true,
			}).
			Create(&domain.TicketSequence{ProductID: real.ID, NextTicketLetter: "ก", NextTicketNumber: 0}).Error
	})
}
