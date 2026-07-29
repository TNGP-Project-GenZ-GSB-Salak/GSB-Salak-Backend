package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account"
	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
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
}

func NewBuySalakService(db *gorm.DB, accounts account.Service, salakSvc salak.Service, ledgerRepo transaction.LedgerRepository) *BuySalakService {
	return &BuySalakService{db: db, accounts: accounts, salakSvc: salakSvc, ledgerRepo: ledgerRepo}
}

var _ transaction.Service = (*BuySalakService)(nil)

func (s *BuySalakService) BuySalak(ctx context.Context, userID, fundingAccountID, salakAccountID, productID uuid.UUID, amount decimal.Decimal) (transaction.BuySalakReceipt, error) {
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

	salakAccount, err := s.accounts.GetByID(ctx, userID, salakAccountID)
	if err != nil {
		return transaction.BuySalakReceipt{}, err
	}
	if salakAccount.Type != accountdomain.TypeSalak {
		return transaction.BuySalakReceipt{}, apperror.Validation("salak_account_id must reference a salak-type account")
	}

	product, err := s.salakSvc.GetProduct(ctx, productID)
	if err != nil {
		return transaction.BuySalakReceipt{}, err
	}
	if err := s.salakSvc.ValidatePurchase(product, amount); err != nil {
		return transaction.BuySalakReceipt{}, err
	}

	var receipt transaction.BuySalakReceipt
	err = s.db.Transaction(func(tx *gorm.DB) error {
		fundingBalanceAfter, err := s.accounts.Debit(ctx, tx, fundingAccountID, amount)
		if err != nil {
			return err
		}

		holding, err := s.salakSvc.MintHolding(ctx, tx, salakAccountID, productID, amount)
		if err != nil {
			return err
		}

		salakBalanceAfter, err := s.accounts.Credit(ctx, tx, salakAccountID, amount)
		if err != nil {
			return err
		}

		refID := uuid.New()
		now := time.Now().UTC()
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
			return err
		}
		if err := s.ledgerRepo.Create(ctx, tx, creditEntry); err != nil {
			return err
		}

		receipt = transaction.BuySalakReceipt{
			ReferenceID:                refID,
			ProductName:                product.Name,
			Units:                      holding.Units,
			TicketStart:                holding.TicketStart,
			TicketEnd:                  holding.TicketEnd,
			Amount:                     amount,
			FundingAccountBalanceAfter: fundingBalanceAfter,
			SalakAccountBalanceAfter:   salakBalanceAfter,
			PurchaseDate:               holding.PurchaseDate.Format("2006-01-02"),
			MaturityDate:               holding.MaturityDate.Format("2006-01-02"),
		}
		return nil
	})
	if err != nil {
		return transaction.BuySalakReceipt{}, err
	}

	return receipt, nil
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
