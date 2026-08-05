package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// accountNumberSequences/accountNumberPrefixes back NextAccountNumber. Each
// prefix is disjoint from every seeded account number's leading digits
// (1234009012, 4001000111, 5001000111 - see cmd/seed/main.go), so a
// generated number can never collide regardless of how far its sequence has
// advanced.
var accountNumberSequences = map[domain.Type]string{
	domain.TypeSavings: "account.savings_account_number_seq",
	domain.TypeSalak:   "account.salak_account_number_seq",
	domain.TypeKapook:  "account.kapook_account_number_seq",
}

var accountNumberPrefixes = map[domain.Type]string{
	domain.TypeSavings: "61",
	domain.TypeSalak:   "62",
	domain.TypeKapook:  "63",
}

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

func (r *GormAccountRepository) FindPrimaryByUserID(ctx context.Context, userID uuid.UUID) (domain.Account, error) {
	var a domain.Account
	err := r.db.WithContext(ctx).Where("user_id = ? AND is_primary_account", userID).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Account{}, err
	}
	return a, err
}

func (r *GormAccountRepository) NextAccountNumber(ctx context.Context, tx *gorm.DB, accountType domain.Type) (string, error) {
	if tx == nil {
		tx = r.db
	}
	seq, ok := accountNumberSequences[accountType]
	if !ok {
		return "", fmt.Errorf("no account-number sequence for type %q", accountType)
	}
	var next int64
	if err := tx.WithContext(ctx).Raw("SELECT nextval(?::regclass)", seq).Scan(&next).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%08d", accountNumberPrefixes[accountType], next), nil
}
