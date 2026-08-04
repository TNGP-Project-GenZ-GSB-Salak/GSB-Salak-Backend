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
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
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

// freeWithdrawalsPerWindow is the first-two-are-free allowance within a
// goal's current rolling 12-month window (domain.WithdrawalWindow).
const freeWithdrawalsPerWindow = 2

// withdrawalFeeRate is the fee on a withdrawal beyond the free allowance,
// derived at read time from the pre-fee amount - never stored.
var withdrawalFeeRate = decimal.RequireFromString("0.02")

// withdrawalCountedTypes are the kapook_transaction types that consume a
// goal's free-withdrawal allowance.
var withdrawalCountedTypes = []domain.TransactionType{domain.TransactionWithdraw, domain.TransactionWithdrawWithFee}

type KapookService struct {
	terms        kapook.TermsRepository
	goals        kapook.GoalRepository
	salakSvc     salak.Service
	accounts     account.Service
	db           *gorm.DB
	ledgerRepo   transaction.LedgerRepository
	transactions kapook.TransactionRepository
	clk          clock.Clock
	buySalakSvc  transaction.Service
}

func NewKapookService(terms kapook.TermsRepository, goals kapook.GoalRepository, salakSvc salak.Service, accounts account.Service, db *gorm.DB, ledgerRepo transaction.LedgerRepository, transactions kapook.TransactionRepository, clk clock.Clock, buySalakSvc transaction.Service) *KapookService {
	return &KapookService{terms: terms, goals: goals, salakSvc: salakSvc, accounts: accounts, db: db, ledgerRepo: ledgerRepo, transactions: transactions, clk: clk, buySalakSvc: buySalakSvc}
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

		// Lock order is money-flow-independent: the kapook account is always
		// locked first, regardless of which leg (debit or credit) it plays
		// in this particular operation. Withdraw locks the same account
		// first too, so the two can never hold these two rows in opposite
		// order against each other - that would deadlock. Re-locking the
		// kapook row inside the Credit call below is a no-op since this tx
		// already holds it.
		if _, err := s.accounts.LockForUpdate(ctx, tx, kapookAccountID); err != nil {
			return err
		}

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
		now := s.clk.Now()
		kapookTx := &domain.Transaction{
			ID:               refID,
			Type:             domain.TransactionDeposit,
			Amount:           amount,
			KapookAccountID:  kapookAccountID,
			GoalID:           goal.ID,
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

// withdrawalAllowance counts goal's withdrawals in the rolling-12-month
// window (anchored on goal.CreatedAt) containing s.clk.Now(), using tx if
// given (the authoritative, locked path) or the ambient handle otherwise
// (the unlocked preview path). It returns the window bounds alongside the
// count so both callers can derive the rest without recomputing "now" twice.
func (s *KapookService) withdrawalAllowance(ctx context.Context, tx *gorm.DB, goal domain.Goal) (used int, windowStart, windowEnd time.Time, err error) {
	windowStart, windowEnd = domain.WithdrawalWindow(goal.CreatedAt, s.clk.Now())
	used, err = s.transactions.CountByGoalAndTypesInWindow(ctx, tx, goal.ID, withdrawalCountedTypes, windowStart, windowEnd)
	if err != nil {
		return 0, time.Time{}, time.Time{}, apperror.Internal("failed to count prior withdrawals", err)
	}
	return used, windowStart, windowEnd, nil
}

// GetWithdrawalStatus previews the free/fee outcome a withdrawal would have
// right now, without locking anything - a concurrent withdrawal can still
// change the answer before the customer acts on it, which is fine for a
// preview; Withdraw itself re-checks under lock.
func (s *KapookService) GetWithdrawalStatus(ctx context.Context, userID, kapookAccountID uuid.UUID) (kapook.WithdrawalStatus, error) {
	if _, err := s.kapookAccount(ctx, userID, kapookAccountID); err != nil {
		return kapook.WithdrawalStatus{}, err
	}

	goal, err := s.goals.FindActiveByAccountID(ctx, kapookAccountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return kapook.WithdrawalStatus{}, apperror.NotFound("no active goal for this account")
	}
	if err != nil {
		return kapook.WithdrawalStatus{}, apperror.Internal("failed to look up active goal", err)
	}

	used, windowStart, windowEnd, err := s.withdrawalAllowance(ctx, nil, goal)
	if err != nil {
		return kapook.WithdrawalStatus{}, err
	}

	remaining := freeWithdrawalsPerWindow - used
	if remaining < 0 {
		remaining = 0
	}
	return kapook.WithdrawalStatus{
		WindowStart:              windowStart,
		WindowEnd:                windowEnd,
		FreeWithdrawalsUsed:      used,
		FreeWithdrawalsRemaining: remaining,
		NextWithdrawalIsFree:     used < freeWithdrawalsPerWindow,
	}, nil
}

// Withdraw moves money from kapookAccountID back to savingsAccountID,
// atomically, for any amount up to the active goal's SavingAmount. The
// goal's IsActive is never touched here - even a full withdrawal leaves the
// goal open to keep saving toward the same target. The free-allowance check
// runs inside this transaction against the goal row FindActiveByAccountIDForUpdate
// already locked, so two concurrent withdrawals against the same goal
// serialize rather than racing to read the same "1 used" count.
func (s *KapookService) Withdraw(ctx context.Context, userID, kapookAccountID, savingsAccountID uuid.UUID, amount decimal.Decimal) (kapook.WithdrawResult, error) {
	if kapookAccountID == savingsAccountID {
		return kapook.WithdrawResult{}, apperror.Validation("kapook_account_id and savings_account_id must be different")
	}

	if _, err := s.kapookAccount(ctx, userID, kapookAccountID); err != nil {
		return kapook.WithdrawResult{}, err
	}

	savingsAcc, err := s.accounts.GetByID(ctx, userID, savingsAccountID)
	if err != nil {
		return kapook.WithdrawResult{}, err
	}
	if savingsAcc.Type != accountdomain.TypeSavings {
		return kapook.WithdrawResult{}, apperror.Validation("savings_account_id must reference a savings-type account")
	}

	if amount.LessThanOrEqual(decimal.Zero) {
		return kapook.WithdrawResult{}, apperror.Validation("amount must be greater than zero")
	}

	var result kapook.WithdrawResult
	err = s.db.Transaction(func(tx *gorm.DB) error {
		goal, err := s.goals.FindActiveByAccountIDForUpdate(ctx, tx, kapookAccountID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.NotFound("no active goal for this account")
		}
		if err != nil {
			return apperror.Internal("failed to lock active goal", err)
		}

		if amount.GreaterThan(goal.AvailableBalance()) {
			return apperror.Validation("withdrawal amount exceeds the kapook balance")
		}

		used, _, _, err := s.withdrawalAllowance(ctx, tx, goal)
		if err != nil {
			return err
		}
		feeCharged := used >= freeWithdrawalsPerWindow

		feeAmount := decimal.Zero
		if feeCharged {
			feeAmount = amount.Mul(withdrawalFeeRate)
		}
		netCredited := amount.Sub(feeAmount)

		// Lock order matches Deposit's: the kapook account first,
		// regardless of debit/credit role, so the two operations can never
		// deadlock against each other over the same account pair.
		if _, err := s.accounts.LockForUpdate(ctx, tx, kapookAccountID); err != nil {
			return err
		}

		kapookBalanceAfter, err := s.accounts.Debit(ctx, tx, kapookAccountID, amount)
		if err != nil {
			return err
		}

		savingsBalanceAfter, err := s.accounts.Credit(ctx, tx, savingsAccountID, netCredited)
		if err != nil {
			return err
		}

		newSavingAmount := goal.SavingAmount.Sub(amount)
		if err := s.goals.UpdateSavingAmount(ctx, tx, goal.ID, newSavingAmount); err != nil {
			return apperror.Internal("failed to update goal saving amount", err)
		}

		txType := domain.TransactionWithdraw
		if feeCharged {
			txType = domain.TransactionWithdrawWithFee
		}

		refID := uuid.New()
		now := s.clk.Now()
		kapookTx := &domain.Transaction{
			ID:               refID,
			Type:             txType,
			Amount:           amount,
			KapookAccountID:  kapookAccountID,
			GoalID:           goal.ID,
			SavingsAccountID: &savingsAccountID,
		}
		if err := s.transactions.Create(ctx, tx, kapookTx); err != nil {
			return apperror.Internal("failed to record kapook transaction", err)
		}

		description := "Kapook withdrawal"
		if feeCharged {
			description = "Kapook withdrawal (2% fee)"
		}
		debitEntry := &txdomain.LedgerEntry{
			ID:            uuid.New(),
			AccountID:     kapookAccountID,
			Type:          txdomain.EntryDebit,
			Amount:        amount,
			BalanceAfter:  kapookBalanceAfter,
			ReferenceType: "kapook_transaction",
			ReferenceID:   refID,
			Description:   description,
			CreatedAt:     now,
		}
		creditEntry := &txdomain.LedgerEntry{
			ID:            uuid.New(),
			AccountID:     savingsAccountID,
			Type:          txdomain.EntryCredit,
			Amount:        netCredited,
			BalanceAfter:  savingsBalanceAfter,
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
		result = kapook.WithdrawResult{
			Goal:        goal,
			Amount:      amount,
			FeeCharged:  feeCharged,
			FeeAmount:   feeAmount,
			NetCredited: netCredited,
		}
		return nil
	})
	if err != nil {
		return kapook.WithdrawResult{}, err
	}

	return result, nil
}

// BuyFromGoal converts amount of the active goal's AvailableBalance into
// its own product, via transaction.Service.BuySalakForKapook - the same
// debit/mint/credit/ledger-pair core the public buy-salak endpoint uses,
// just funded from the kapook account instead of a savings one. It writes
// no ledger pair of its own; the one BuySalakForKapook writes is the whole
// record. SavingAmount is untouched here (see domain.Goal.AvailableBalance);
// only SalakAmount grows, and only a purchase that fully satisfies
// GoalAmount deactivates the goal.
func (s *KapookService) BuyFromGoal(ctx context.Context, userID, kapookAccountID, salakAccountID uuid.UUID, amount decimal.Decimal) (kapook.BuyFromGoalResult, error) {
	if kapookAccountID == salakAccountID {
		return kapook.BuyFromGoalResult{}, apperror.Validation("kapook_account_id and salak_account_id must be different")
	}

	if _, err := s.kapookAccount(ctx, userID, kapookAccountID); err != nil {
		return kapook.BuyFromGoalResult{}, err
	}

	salakAcc, err := s.accounts.GetByID(ctx, userID, salakAccountID)
	if err != nil {
		return kapook.BuyFromGoalResult{}, err
	}
	if salakAcc.Type != accountdomain.TypeSalak {
		return kapook.BuyFromGoalResult{}, apperror.Validation("salak_account_id must reference a salak-type account")
	}

	if amount.LessThanOrEqual(decimal.Zero) {
		return kapook.BuyFromGoalResult{}, apperror.Validation("amount must be greater than zero")
	}

	var result kapook.BuyFromGoalResult
	err = s.db.Transaction(func(tx *gorm.DB) error {
		goal, err := s.goals.FindActiveByAccountIDForUpdate(ctx, tx, kapookAccountID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperror.NotFound("no active goal for this account")
		}
		if err != nil {
			return apperror.Internal("failed to lock active goal", err)
		}

		product, err := s.salakSvc.GetProduct(ctx, goal.ProductID)
		if err != nil {
			return err
		}

		available := goal.AvailableBalance()
		if available.LessThan(product.MinPurchase) {
			return apperror.Validation("kapook balance must be at least the product's minimum purchase amount to buy Salak")
		}
		if err := s.salakSvc.ValidatePurchase(product, amount); err != nil {
			return err
		}
		if amount.GreaterThan(available) {
			return apperror.Validation("amount exceeds the kapook balance")
		}

		receipt, err := s.buySalakSvc.BuySalakForKapook(ctx, tx, userID, kapookAccountID, salakAccountID, goal.ProductID, amount)
		if err != nil {
			return err
		}

		newSalakAmount := goal.SalakAmount.Add(amount)
		goalCompleted := !newSalakAmount.LessThan(goal.GoalAmount)
		if err := s.goals.UpdateAfterPurchase(ctx, tx, goal.ID, newSalakAmount, !goalCompleted); err != nil {
			return apperror.Internal("failed to update goal after purchase", err)
		}

		holdingID := receipt.HoldingID
		kapookTx := &domain.Transaction{
			ID:              uuid.New(),
			Type:            domain.TransactionBuySalak,
			Amount:          amount,
			KapookAccountID: kapookAccountID,
			GoalID:          goal.ID,
			HoldingID:       &holdingID,
		}
		if err := s.transactions.Create(ctx, tx, kapookTx); err != nil {
			return apperror.Internal("failed to record kapook transaction", err)
		}

		goal.SalakAmount = newSalakAmount
		goal.IsActive = !goalCompleted
		result = kapook.BuyFromGoalResult{Goal: goal, Receipt: receipt, GoalCompleted: goalCompleted}
		return nil
	})
	if err != nil {
		return kapook.BuyFromGoalResult{}, err
	}

	return result, nil
}
