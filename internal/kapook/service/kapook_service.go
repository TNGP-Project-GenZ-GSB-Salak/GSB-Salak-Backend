package service

import (
	"context"
	"errors"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account"
	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction"
	txdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/domain"
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
	terms        kapook.TermsRepository
	goals        kapook.GoalRepository
	salakSvc     salak.Service
	accounts     account.Service
	db           *gorm.DB
	ledgerRepo   transaction.LedgerRepository
	transactions kapook.TransactionRepository
}

func NewKapookService(terms kapook.TermsRepository, goals kapook.GoalRepository, salakSvc salak.Service, accounts account.Service, db *gorm.DB, ledgerRepo transaction.LedgerRepository, transactions kapook.TransactionRepository) *KapookService {
	return &KapookService{terms: terms, goals: goals, salakSvc: salakSvc, accounts: accounts, db: db, ledgerRepo: ledgerRepo, transactions: transactions}
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

// Deposit moves money from savingsAccountID into kapookAccountID atomically,
// then bumps the account's active goal's SavingAmount. Any positive amount
// is accepted, no minimum - but a deposit that would push SavingAmount past
// GoalAmount is rejected.
func (s *KapookService) Deposit(ctx context.Context, userID, kapookAccountID, savingsAccountID uuid.UUID, amount decimal.Decimal) (domain.Goal, error) {
	if kapookAccountID == savingsAccountID {
		return domain.Goal{}, apperror.Validation("kapook_account_id and savings_account_id must be different")
	}

	if _, err := s.kapookAccount(ctx, userID, kapookAccountID); err != nil {
		return domain.Goal{}, err
	}

	savingsAcc, err := s.accounts.GetByID(ctx, userID, savingsAccountID)
	if err != nil {
		return domain.Goal{}, err
	}
	if savingsAcc.Type != accountdomain.TypeSavings {
		return domain.Goal{}, apperror.Validation("savings_account_id must reference a savings-type account")
	}

	if amount.LessThanOrEqual(decimal.Zero) {
		return domain.Goal{}, apperror.Validation("amount must be greater than zero")
	}

	var updatedGoal domain.Goal
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Locked and validated against the target before anything moves,
		// so a deposit that would overshoot is rejected without touching
		// either account's balance.
		goal, err := s.goals.FindActiveByAccountIDForUpdate(ctx, tx, kapookAccountID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.NotFound("no active goal for this account")
		}
		if err != nil {
			return apperror.Internal("failed to lock active goal", err)
		}

		newSavingAmount := goal.SavingAmount.Add(amount)
		if newSavingAmount.GreaterThan(goal.GoalAmount) {
			return apperror.Validation("deposit would exceed the goal's target amount")
		}

		// Lock order here is savings-account-then-kapook-account (debit
		// before credit, per the documented convention). A future withdraw
		// moves money the other way, but MUST NOT simply reverse this to
		// "debit kapook, credit savings" - that would lock the same two
		// accounts in the opposite order, and a concurrent deposit +
		// withdraw on the same pair would deadlock. Whichever ticket adds
		// withdraw needs a money-flow-independent lock order (e.g. always
		// lock the kapook account first, regardless of debit/credit role).
		savingsBalanceAfter, err := s.accounts.Debit(ctx, tx, savingsAccountID, amount)
		if err != nil {
			return err
		}

		kapookBalanceAfter, err := s.accounts.Credit(ctx, tx, kapookAccountID, amount)
		if err != nil {
			return err
		}

		if err := s.goals.UpdateSavingAmount(ctx, tx, goal.ID, newSavingAmount); err != nil {
			return apperror.Internal("failed to update goal saving amount", err)
		}

		// The kapook_transaction's own id doubles as the ledger pair's
		// shared reference_id - reference_type/reference_id carry no
		// CHECK/FK, so this needs no ledger migration.
		refID := uuid.New()
		now := time.Now().UTC()
		kapookTx := &domain.Transaction{
			ID:               refID,
			Type:             domain.TransactionDeposit,
			Amount:           amount,
			KapookAccountID:  kapookAccountID,
			SavingsAccountID: &savingsAccountID,
		}
		if err := s.transactions.Create(ctx, tx, kapookTx); err != nil {
			return apperror.Internal("failed to record kapook transaction", err)
		}

		description := "Kapook deposit"
		debitEntry := &txdomain.LedgerEntry{
			ID:            uuid.New(),
			AccountID:     savingsAccountID,
			Type:          txdomain.EntryDebit,
			Amount:        amount,
			BalanceAfter:  savingsBalanceAfter,
			ReferenceType: "kapook_transaction",
			ReferenceID:   refID,
			Description:   description,
			CreatedAt:     now,
		}
		creditEntry := &txdomain.LedgerEntry{
			ID:            uuid.New(),
			AccountID:     kapookAccountID,
			Type:          txdomain.EntryCredit,
			Amount:        amount,
			BalanceAfter:  kapookBalanceAfter,
			ReferenceType: "kapook_transaction",
			ReferenceID:   refID,
			Description:   description,
			CreatedAt:     now,
		}
		if err := s.ledgerRepo.Create(ctx, tx, debitEntry); err != nil {
			return apperror.Internal("failed to write ledger entry", err)
		}
		if err := s.ledgerRepo.Create(ctx, tx, creditEntry); err != nil {
			return apperror.Internal("failed to write ledger entry", err)
		}

		goal.SavingAmount = newSavingAmount
		updatedGoal = goal
		return nil
	})
	if err != nil {
		return domain.Goal{}, err
	}

	return updatedGoal, nil
}
