package repository

import (
	"context"
	"errors"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/user/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GormUserRepository struct {
	db *gorm.DB
}

func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) Create(ctx context.Context, tx *gorm.DB, u *domain.User) error {
	if tx == nil {
		tx = r.db
	}
	return tx.WithContext(ctx).Create(u).Error
}

func (r *GormUserRepository) FindByUsername(ctx context.Context, username string) (domain.User, error) {
	var u domain.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, err
	}
	return u, err
}

func (r *GormUserRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	var u domain.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, err
	}
	return u, err
}
