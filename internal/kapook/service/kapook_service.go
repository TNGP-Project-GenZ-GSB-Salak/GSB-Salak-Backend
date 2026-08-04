package service

import (
	"context"
	"errors"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account"
	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// sqlStateUniqueViolation is Postgres's SQLSTATE for a unique/exclusion
// constraint violation. https://www.postgresql.org/docs/current/errcodes-appendix.html
const sqlStateUniqueViolation = "23505"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == sqlStateUniqueViolation
}

type KapookService struct {
	terms    kapook.TermsRepository
	goals    kapook.GoalRepository
	salakSvc salak.Service
	accounts account.Service
}

func NewKapookService(terms kapook.TermsRepository, goals kapook.GoalRepository, salakSvc salak.Service, accounts account.Service) *KapookService {
	return &KapookService{terms: terms, goals: goals, salakSvc: salakSvc, accounts: accounts}
}

var _ kapook.Service = (*KapookService)(nil)

func (s *KapookService) Accept(ctx context.Context, userID uuid.UUID) error {
	if err := s.terms.Accept(ctx, userID); err != nil {
		return apperror.Internal("failed to record terms acceptance", err)
	}
	return nil
}

func (s *KapookService) HasAccepted(ctx context.Context, userID uuid.UUID) (bool, error) {
	accepted, err := s.terms.HasAccepted(ctx, userID)
	if err != nil {
		return false, apperror.Internal("failed to check terms acceptance", err)
	}
	return accepted, nil
}

// kapookAccount verifies accountID is owned by userID and is a kapook-type
// account, shared by every goal method since each needs the same check.
func (s *KapookService) kapookAccount(ctx context.Context, userID, accountID uuid.UUID) (accountdomain.Account, error) {
	acc, err := s.accounts.GetByID(ctx, userID, accountID)
	if err != nil {
		return accountdomain.Account{}, err
	}
	if acc.Type != accountdomain.TypeKapook {
		return accountdomain.Account{}, apperror.Validation("account_id must reference a kapook-type account")
	}
	return acc, nil
}

func (s *KapookService) CreateGoal(ctx context.Context, userID, accountID, productID uuid.UUID, goalAmount decimal.Decimal) (domain.Goal, error) {
	if _, err := s.kapookAccount(ctx, userID, accountID); err != nil {
		return domain.Goal{}, err
	}

	accepted, err := s.terms.HasAccepted(ctx, userID)
	if err != nil {
		return domain.Goal{}, apperror.Internal("failed to check terms acceptance", err)
	}
	if !accepted {
		return domain.Goal{}, apperror.Forbidden("you must accept the Kapook terms and conditions before creating a goal")
	}

	_, err = s.goals.FindActiveByAccountID(ctx, accountID)
	if err == nil {
		return domain.Goal{}, apperror.Conflict("an active goal already exists for this account")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Goal{}, apperror.Internal("failed to check for an existing active goal", err)
	}

	product, err := s.salakSvc.GetProduct(ctx, productID)
	if err != nil {
		return domain.Goal{}, err
	}
	if err := s.salakSvc.ValidatePurchase(product, goalAmount); err != nil {
		return domain.Goal{}, err
	}

	goal := &domain.Goal{
		ID:           uuid.New(),
		AccountID:    accountID,
		ProductID:    productID,
		GoalAmount:   goalAmount,
		SavingAmount: decimal.Zero,
		SalakAmount:  decimal.Zero,
		IsActive:     true,
	}
	if err := s.goals.Create(ctx, goal); err != nil {
		// The FindActiveByAccountID pre-check above is the common path;
		// this catches the rare concurrent double-create the pre-check
		// can't - the partial unique index is the actual race-safe
		// authority, so a violation here means the same "already exists"
		// conflict, not a server fault.
		if isUniqueViolation(err) {
			return domain.Goal{}, apperror.Conflict("an active goal already exists for this account")
		}
		return domain.Goal{}, apperror.Internal("failed to create goal", err)
	}

	return *goal, nil
}

func (s *KapookService) GetActiveGoal(ctx context.Context, userID, accountID uuid.UUID) (domain.Goal, error) {
	if _, err := s.kapookAccount(ctx, userID, accountID); err != nil {
		return domain.Goal{}, err
	}

	goal, err := s.goals.FindActiveByAccountID(ctx, accountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Goal{}, apperror.NotFound("no active goal for this account")
	}
	if err != nil {
		return domain.Goal{}, apperror.Internal("failed to look up active goal", err)
	}
	return goal, nil
}
