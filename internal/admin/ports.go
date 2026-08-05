package admin

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/admin/domain"
)

// Repository is implemented by the gorm repository and consumed by the service.
type Repository interface {
	FindByUsername(ctx context.Context, username string) (domain.Admin, error)
}

// Service is the public surface the http layer depends on.
type Service interface {
	// Login returns apperror.Unauthorized on any username/password
	// mismatch, identically for "no such admin" and "wrong password" - same
	// convention as user.Service.Login.
	Login(ctx context.Context, username, password string) (domain.Admin, string, error)
}
