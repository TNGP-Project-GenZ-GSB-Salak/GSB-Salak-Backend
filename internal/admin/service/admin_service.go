package service

import (
	"context"
	"errors"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/admin"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/admin/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AdminService struct {
	repo   admin.Repository
	signer *jwtutil.AdminSigner
}

func NewAdminService(repo admin.Repository, signer *jwtutil.AdminSigner) *AdminService {
	return &AdminService{repo: repo, signer: signer}
}

var _ admin.Service = (*AdminService)(nil)

func (s *AdminService) Login(ctx context.Context, username, password string) (domain.Admin, string, error) {
	a, err := s.repo.FindByUsername(ctx, username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Admin{}, "", apperror.Unauthorized("invalid username or password")
	} else if err != nil {
		return domain.Admin{}, "", apperror.Internal("failed to look up admin", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)); err != nil {
		return domain.Admin{}, "", apperror.Unauthorized("invalid username or password")
	}

	token, err := s.signer.Sign(a.ID)
	if err != nil {
		return domain.Admin{}, "", apperror.Internal("failed to sign token", err)
	}
	return a, token, nil
}
