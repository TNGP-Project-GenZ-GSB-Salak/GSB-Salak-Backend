package service

import (
	"context"
	"errors"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/user"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/user/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	repo   user.Repository
	signer *jwtutil.Signer
}

func NewAuthService(repo user.Repository, signer *jwtutil.Signer) *AuthService {
	return &AuthService{repo: repo, signer: signer}
}

var _ user.Service = (*AuthService)(nil)

func (s *AuthService) Register(ctx context.Context, username, password, fullName string) (domain.User, error) {
	if username == "" || password == "" || fullName == "" {
		return domain.User{}, apperror.Validation("username, password, and full_name are required")
	}
	if len(password) < 8 {
		return domain.User{}, apperror.Validation("password must be at least 8 characters")
	}

	if _, err := s.repo.FindByUsername(ctx, username); err == nil {
		return domain.User{}, apperror.Conflict("username already taken")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, apperror.Internal("failed to check username", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, apperror.Internal("failed to hash password", err)
	}

	u := &domain.User{
		ID:           uuid.New(),
		Username:     username,
		PasswordHash: string(hash),
		FullName:     fullName,
	}
	if err := s.repo.Create(ctx, nil, u); err != nil {
		return domain.User{}, apperror.Internal("failed to create user", err)
	}
	return *u, nil
}

func (s *AuthService) Login(ctx context.Context, username, password string) (domain.User, string, error) {
	u, err := s.repo.FindByUsername(ctx, username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, "", apperror.Unauthorized("invalid username or password")
	} else if err != nil {
		return domain.User{}, "", apperror.Internal("failed to look up user", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return domain.User{}, "", apperror.Unauthorized("invalid username or password")
	}

	token, err := s.signer.Sign(u.ID)
	if err != nil {
		return domain.User{}, "", apperror.Internal("failed to sign token", err)
	}
	return u, token, nil
}

func (s *AuthService) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, apperror.NotFound("user not found")
	} else if err != nil {
		return domain.User{}, apperror.Internal("failed to look up user", err)
	}
	return u, nil
}
