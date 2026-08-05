package repository

import (
	"context"
	"errors"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/admin/domain"
	"gorm.io/gorm"
)

type GormAdminRepository struct {
	db *gorm.DB
}

func NewGormAdminRepository(db *gorm.DB) *GormAdminRepository {
	return &GormAdminRepository{db: db}
}

func (r *GormAdminRepository) FindByUsername(ctx context.Context, username string) (domain.Admin, error) {
	var a domain.Admin
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Admin{}, err
	}
	return a, err
}
