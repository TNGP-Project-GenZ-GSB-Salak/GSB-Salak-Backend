package service

import (
	"context"
	"fmt"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account"
	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/badge"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction"
	txdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type BuySalakService struct {
	db         *gorm.DB
	accounts   account.Service
	salakSvc   salak.Service
	ledgerRepo transaction.LedgerRepository
	badgeSvc   badge.Service
	clock      clock.Clock
}

func NewBuySalakService(db *gorm.DB, accounts account.Service, salakSvc salak.Service, ledgerRepo transaction.LedgerRepository, badgeSvc badge.Service, clk clock.Clock) *BuySalakService {
	return &BuySalakService{db: db, accounts: accounts, salakSvc: salakSvc, ledgerRepo: ledgerRepo, badgeSvc: badgeSvc, clock: clk}
}

var _ transaction.Service = (*BuySalakService)(nil)

func (s *BuySalakService) BuySalak(ctx context.Context, userID, fundingAccountID, salakAccountID, productID uuid.UUID, badgeID *uuid.UUID, amount decimal.Decimal) (transaction.BuySalakReceipt, error) {
	if fundingAccountID == salakAccountID {
		return transaction.BuySalakReceipt{}, apperror.Validation("funding account and salak account must be different")
	}

	fundingAccount, err := s.accounts.GetByID(ctx, userID, fundingAccountID)
	if err != nil {
		return transaction.BuySalakReceipt{}, err
	}
	if fundingAccount.Type != accountdomain.TypeSavings {
		return transaction.BuySalakReceipt{}, apperror.Validation("funding_account_id must reference a savings-type account")
	}

	salakAccount, product, err := s.validateSalakSideAndProduct(ctx, userID, salakAccountID, productID, amount)
	if err != nil {
		return transaction.BuySalakReceipt{}, err
	}

	if badgeID != nil {
		owns, err := s.badgeSvc.UserOwnsBadge(ctx, userID, *badgeID)
		if err != nil {
			return transaction.BuySalakReceipt{}, apperror.Internal("failed to check badge ownership", err)
		}
		if !owns {
			return transaction.BuySalakReceipt{}, apperror.Forbidden("you do not own the specified badge")
		}
	}

	var receipt transaction.BuySalakReceipt
	err = s.db.Transaction(func(tx *gorm.DB) error {
		receipt, err = s.mintAndSettle(ctx, tx, fundingAccountID, salakAccount.ID, productID, amount, product)
		return err
	})
	if err != nil {
		return transaction.BuySalakReceipt{}, err
	}

	return receipt, nil
}

// BuySalakForKapook is the kapook-only variant documented on transaction.Service.
func (s *BuySalakService) BuySalakForKapook(ctx context.Context, tx *gorm.DB, userID, kapookAccountID, salakAccountID, productID uuid.UUID, amount decimal.Decimal) (transaction.BuySalakReceipt, error) {
	if kapookAccountID == salakAccountID {
		return transaction.BuySalakReceipt{}, apperror.Validation("funding account and salak account must be different")
	}

	kapookAccount, err := s.accounts.GetByID(ctx, userID, kapookAccountID)
	if err != nil {
		return transaction.BuySalakReceipt{}, err
	}
	if kapookAccount.Type != accountdomain.TypeKapook {
		return transaction.BuySalakReceipt{}, apperror.Validation("funding_account_id must reference a kapook-type account")
	}

	salakAccount, product, err := s.validateSalakSideAndProduct(ctx, userID, salakAccountID, productID, amount)
	if err != nil {
		return transaction.BuySalakReceipt{}, err
	}

	return s.mintAndSettle(ctx, tx, kapookAccountID, salakAccount.ID, productID, amount, product)
}

// validateSalakSideAndProduct is the half of BuySalak/BuySalakForKapook's
// pre-transaction validation that doesn't depend on which account type is
// allowed to fund the purchase: the salak account itself, the product, its
// purchase rules, and the draw-day guard.
func (s *BuySalakService) validateSalakSideAndProduct(ctx context.Context, userID, salakAccountID, productID uuid.UUID, amount decimal.Decimal) (accountdomain.Account, salakdomain.Product, error) {
	salakAccount, err := s.accounts.GetByID(ctx, userID, salakAccountID)
	if err != nil {
		return accountdomain.Account{}, salakdomain.Product{}, err
	}
	if salakAccount.Type != accountdomain.TypeSalak {
		return accountdomain.Account{}, salakdomain.Product{}, apperror.Validation("salak_account_id must reference a salak-type account")
	}

	product, err := s.salakSvc.GetProduct(ctx, productID)
	if err != nil {
		return accountdomain.Account{}, salakdomain.Product{}, err
	}
	if err := s.salakSvc.ValidatePurchase(product, amount); err != nil {
		return accountdomain.Account{}, salakdomain.Product{}, err
	}
	if err := s.salakSvc.EnsureNotDrawDay(ctx, product); err != nil {
		return accountdomain.Account{}, salakdomain.Product{}, err
	}

	return salakAccount, product, nil
}

// mintAndSettle is BuySalak/BuySalakForKapook's shared money-movement core:
// debit the funding account, mint the holding, credit the salak account,
// and write the one debit+credit ledger pair - always in that
// debit-before-credit lock order, regardless of which account type funded
// it. Callers own the enclosing transaction (BuySalak opens its own;
// BuySalakForKapook is handed one by the kapook service), so this never
// opens one itself.
func (s *BuySalakService) mintAndSettle(ctx context.Context, tx *gorm.DB, fundingAccountID, salakAccountID, productID uuid.UUID, amount decimal.Decimal, product salakdomain.Product) (transaction.BuySalakReceipt, error) {
	fundingBalanceAfter, err := s.accounts.Debit(ctx, tx, fundingAccountID, amount)
	if err != nil {
		return transaction.BuySalakReceipt{}, err
	}

	holding, err := s.salakSvc.MintHolding(ctx, tx, salakAccountID, productID, amount)
	if err != nil {
		return transaction.BuySalakReceipt{}, err
	}

	salakBalanceAfter, err := s.accounts.Credit(ctx, tx, salakAccountID, amount)
	if err != nil {
		return transaction.BuySalakReceipt{}, err
	}

	refID := uuid.New()
	now := s.clock.Now()
	description := fmt.Sprintf("Buy %s", product.Name)

	debitEntry := &txdomain.LedgerEntry{
		ID:            uuid.New(),
		AccountID:     fundingAccountID,
		HoldingID:     &holding.ID,
		Type:          txdomain.EntryDebit,
		Amount:        amount,
		BalanceAfter:  fundingBalanceAfter,
		ReferenceType: "buy_salak",
		ReferenceID:   refID,
		Description:   description,
		CreatedAt:     now,
	}
	creditEntry := &txdomain.LedgerEntry{
		ID:            uuid.New(),
		AccountID:     salakAccountID,
		HoldingID:     &holding.ID,
		Type:          txdomain.EntryCredit,
		Amount:        amount,
		BalanceAfter:  salakBalanceAfter,
		ReferenceType: "buy_salak",
		ReferenceID:   refID,
		Description:   description,
		CreatedAt:     now,
	}

	if err := s.ledgerRepo.Create(ctx, tx, debitEntry); err != nil {
		return transaction.BuySalakReceipt{}, err
	}
	if err := s.ledgerRepo.Create(ctx, tx, creditEntry); err != nil {
		return transaction.BuySalakReceipt{}, err
	}

	return transaction.BuySalakReceipt{
		ReferenceID:                refID,
		HoldingID:                  holding.ID,
		ProductName:                product.Name,
		Units:                      holding.Units,
		TicketStart:                holding.TicketStartID(),
		TicketEnd:                  holding.TicketEndID(),
		Amount:                     amount,
		FundingAccountBalanceAfter: fundingBalanceAfter,
		SalakAccountBalanceAfter:   salakBalanceAfter,
		PurchaseDate:               holding.PurchaseDate.Format("2006-01-02"),
		MaturityDate:               holding.MaturityDate.Format("2006-01-02"),
	}, nil
}

func (s *BuySalakService) ListHistory(ctx context.Context, userID, accountID uuid.UUID, limit, offset int) ([]txdomain.LedgerEntry, error) {
	if _, err := s.accounts.GetByID(ctx, userID, accountID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	entries, err := s.ledgerRepo.FindByAccountID(ctx, accountID, limit, offset)
	if err != nil {
		return nil, apperror.Internal("failed to list transaction history", err)
	}
	return entries, nil
}
