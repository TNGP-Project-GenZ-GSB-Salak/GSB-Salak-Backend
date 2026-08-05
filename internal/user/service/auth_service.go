package service

import (
	"context"
	"errors"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account"
	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/user"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/user/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	repo     user.Repository
	signer   *jwtutil.Signer
	accounts account.Service
	db       *gorm.DB
	// savingsStartingBalance funds the savings account Register opens, from
	// config.Config.RegistrationSavingsStartingBalance (0 by default).
	savingsStartingBalance decimal.Decimal
}

func NewAuthService(repo user.Repository, signer *jwtutil.Signer, accounts account.Service, db *gorm.DB, savingsStartingBalance decimal.Decimal) *AuthService {
	return &AuthService{repo: repo, signer: signer, accounts: accounts, db: db, savingsStartingBalance: savingsStartingBalance}
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

	// user orchestrates: the user row and all three accounts open atomically,
	// following the codebase's precedent that the domain owning a flow
	// orchestrates it (transaction orchestrates account+salak, kapook
	// orchestrates account+salak+transaction; registration is user's flow).
	// Without all three, the Kapook flow fails partway rather than not at
	// all - see the linked decision ticket for why savings alone isn't enough.
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.Create(ctx, tx, u); err != nil {
			return apperror.Internal("failed to create user", err)
		}
		if _, err := s.accounts.Create(ctx, tx, u.ID, accountdomain.TypeSavings, s.savingsStartingBalance, true); err != nil {
			return apperror.Internal("failed to create savings account", err)
		}
		if _, err := s.accounts.Create(ctx, tx, u.ID, accountdomain.TypeSalak, decimal.Zero, false); err != nil {
			return apperror.Internal("failed to create salak account", err)
		}
		if _, err := s.accounts.Create(ctx, tx, u.ID, accountdomain.TypeKapook, decimal.Zero, false); err != nil {
			return apperror.Internal("failed to create kapook account", err)
		}
		return nil
	})
	if err != nil {
		return domain.User{}, err
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
