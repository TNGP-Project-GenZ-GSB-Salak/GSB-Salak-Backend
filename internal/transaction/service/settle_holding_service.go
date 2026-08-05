package service

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction"
	txdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// SettleMaturedHolding opens its own top-level transaction around
// SettleMaturedHoldingInTx - see that method for the actual logic, and
// transaction.Service's doc comment for when this runs.
func (s *BuySalakService) SettleMaturedHolding(ctx context.Context, holdingID uuid.UUID) (transaction.SettlementReceipt, error) {
	var receipt transaction.SettlementReceipt
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		receipt, err = s.SettleMaturedHoldingInTx(ctx, tx, holdingID)
		return err
	})
	if err != nil {
		return transaction.SettlementReceipt{}, err
	}
	return receipt, nil
}

// SettleMaturedHoldingInTx pays out holdingID's principal + interest to its
// owning user's primary account, inside the caller's own transaction -
// this method itself never checks MaturityDate, only SettledAt (the
// idempotency guard).
//
// Money movement mirrors mintAndSettle's shape but with one extra leg:
// interest has no debit counterpart (it's bank-created, not moved from a
// live account balance), so it's its own unmatched credit rather than
// folded into the principal's debit+credit pair. Splitting the two keeps
// "debit and credit amounts match" true for the principal leg, and makes
// "total interest ever paid out" a plain SUM over ReferenceType.
func (s *BuySalakService) SettleMaturedHoldingInTx(ctx context.Context, tx *gorm.DB, holdingID uuid.UUID) (transaction.SettlementReceipt, error) {
	holding, err := s.salakSvc.FindHoldingForUpdate(ctx, tx, holdingID)
	if err != nil {
		return transaction.SettlementReceipt{}, err
	}
	if holding.SettledAt != nil {
		return transaction.SettlementReceipt{}, apperror.Conflict("holding has already been settled").WithCode(transaction.CodeHoldingAlreadySettled)
	}

	product, err := s.salakSvc.GetProduct(ctx, holding.ProductID)
	if err != nil {
		return transaction.SettlementReceipt{}, err
	}
	interest := product.MaturityInterestPerUnit.Mul(decimal.NewFromInt(holding.Units))

	salakAccount, err := s.accounts.GetByIDUnscoped(ctx, holding.AccountID)
	if err != nil {
		return transaction.SettlementReceipt{}, err
	}

	primary, err := s.accounts.GetPrimaryAccount(ctx, salakAccount.UserID)
	if err != nil {
		return transaction.SettlementReceipt{}, apperror.NotFound("no primary account is on file for this customer").WithCode(transaction.CodeNoPrimaryAccount)
	}

	salakBalanceAfter, err := s.accounts.Debit(ctx, tx, holding.AccountID, holding.PurchaseAmount)
	if err != nil {
		return transaction.SettlementReceipt{}, err
	}
	primaryBalanceAfterPrincipal, err := s.accounts.Credit(ctx, tx, primary.ID, holding.PurchaseAmount)
	if err != nil {
		return transaction.SettlementReceipt{}, err
	}
	primaryBalanceAfterInterest, err := s.accounts.Credit(ctx, tx, primary.ID, interest)
	if err != nil {
		return transaction.SettlementReceipt{}, err
	}

	refID := uuid.New()
	now := s.clock.Now()

	principalDebit := &txdomain.LedgerEntry{
		ID: uuid.New(), AccountID: holding.AccountID, HoldingID: &holding.ID,
		Type: txdomain.EntryDebit, Amount: holding.PurchaseAmount, BalanceAfter: salakBalanceAfter,
		ReferenceType: "salak_maturity", ReferenceID: refID, Description: "สลากครบกำหนด - เงินต้น", CreatedAt: now,
	}
	principalCredit := &txdomain.LedgerEntry{
		ID: uuid.New(), AccountID: primary.ID, HoldingID: &holding.ID,
		Type: txdomain.EntryCredit, Amount: holding.PurchaseAmount, BalanceAfter: primaryBalanceAfterPrincipal,
		ReferenceType: "salak_maturity", ReferenceID: refID, Description: "สลากครบกำหนด - เงินต้น", CreatedAt: now,
	}
	interestCredit := &txdomain.LedgerEntry{
		ID: uuid.New(), AccountID: primary.ID, HoldingID: &holding.ID,
		Type: txdomain.EntryCredit, Amount: interest, BalanceAfter: primaryBalanceAfterInterest,
		ReferenceType: "salak_maturity_interest", ReferenceID: refID, Description: "สลากครบกำหนด - ดอกเบี้ย", CreatedAt: now,
	}
	for _, entry := range []*txdomain.LedgerEntry{principalDebit, principalCredit, interestCredit} {
		if err := s.ledgerRepo.Create(ctx, tx, entry); err != nil {
			return transaction.SettlementReceipt{}, err
		}
	}

	if err := s.salakSvc.MarkHoldingSettled(ctx, tx, holding.ID, now); err != nil {
		return transaction.SettlementReceipt{}, err
	}

	return transaction.SettlementReceipt{
		HoldingID:           holding.ID,
		Principal:           holding.PurchaseAmount,
		Interest:            interest,
		Total:               holding.PurchaseAmount.Add(interest),
		PrimaryAccountID:    primary.ID,
		PrimaryBalanceAfter: primaryBalanceAfterInterest,
		SettledAt:           now.Format("2006-01-02T15:04:05Z"),
	}, nil
}
