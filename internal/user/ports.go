package user

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/user/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository is implemented by the gorm repository and consumed by the service.
type Repository interface {
	Create(ctx context.Context, tx *gorm.DB, u *domain.User) error
	FindByUsername(ctx context.Context, username string) (domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

// Service is the public surface other domains/http layers may depend on.
type Service interface {
	Register(ctx context.Context, username, password, fullName string) (domain.User, error)
	Login(ctx context.Context, username, password string) (domain.User, string, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}
