package service_test

import (
	"context"
	"testing"
	"time"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func maturedHolding(id, accountID, productID uuid.UUID) salakdomain.Holding {
	return salakdomain.Holding{
		ID:             id,
		AccountID:      accountID,
		ProductID:      productID,
		Units:          10,
		PurchaseAmount: decimal.RequireFromString("1000"),
		MaturityDate:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestBuySalakService_SettleMaturedHolding(t *testing.T) {
	t.Run("already settled returns conflict", func(t *testing.T) {
		holdingID, accountID, productID := uuid.New(), uuid.New(), uuid.New()
		settledAt := fixedNow.Add(-time.Hour)
		h := maturedHolding(holdingID, accountID, productID)
		h.SettledAt = &settledAt

		accounts := newFakeAccountService()
		salakSvc := &fakeSalakService{findHoldingForUpdateResult: h}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewBuySalakService(db, accounts, salakSvc, &fakeLedgerRepo{}, &fakeBadgeService{}, testClock())

		_, err := svc.SettleMaturedHolding(context.Background(), holdingID)
		var appErr *apperror.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperror.KindConflict, appErr.Kind)
		assert.Equal(t, transaction.CodeHoldingAlreadySettled, appErr.Code)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no primary account is a not-found error, no money moves", func(t *testing.T) {
		holdingID, accountID, productID, userID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		h := maturedHolding(holdingID, accountID, productID)

		accounts := newFakeAccountService()
		accounts.accounts[accountID] = accountdomain.Account{ID: accountID, UserID: userID, Type: accountdomain.TypeSalak}
		accounts.primaryAccountErr = apperror.NotFound("no primary account is on file for this customer")

		product := activeProduct()
		product.MaturityInterestPerUnit = mustDecimal(t, "0.15")
		salakSvc := &fakeSalakService{findHoldingForUpdateResult: h, getProductResult: product}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		svc := service.NewBuySalakService(db, accounts, salakSvc, &fakeLedgerRepo{}, &fakeBadgeService{}, testClock())

		_, err := svc.SettleMaturedHolding(context.Background(), holdingID)
		var appErr *apperror.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperror.KindNotFound, appErr.Kind)
		assert.Equal(t, transaction.CodeNoPrimaryAccount, appErr.Code)
		assert.Empty(t, accounts.debitCalls)
		assert.Empty(t, accounts.creditCalls)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success debits the salak account, credits primary with principal then interest, writes three ledger entries, marks settled", func(t *testing.T) {
		holdingID, accountID, productID, userID, primaryID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
		h := maturedHolding(holdingID, accountID, productID)

		accounts := newFakeAccountService()
		accounts.accounts[accountID] = accountdomain.Account{ID: accountID, UserID: userID, Type: accountdomain.TypeSalak}
		accounts.primaryAccountResult = accountdomain.Account{ID: primaryID, UserID: userID, Type: accountdomain.TypeSavings, IsPrimaryAccount: true}
		accounts.debitResult = mustDecimal(t, "0")
		accounts.creditResult = mustDecimal(t, "1001.5") // arbitrary; final leg's return value is what the receipt uses

		product := activeProduct()
		product.MaturityInterestPerUnit = mustDecimal(t, "0.15")
		salakSvc := &fakeSalakService{findHoldingForUpdateResult: h, getProductResult: product}
		ledger := &fakeLedgerRepo{}

		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		svc := service.NewBuySalakService(db, accounts, salakSvc, ledger, &fakeBadgeService{}, testClock())

		receipt, err := svc.SettleMaturedHolding(context.Background(), holdingID)
		require.NoError(t, err)

		assert.True(t, mustDecimal(t, "1000").Equal(receipt.Principal))
		assert.True(t, mustDecimal(t, "1.5").Equal(receipt.Interest)) // 10 units * 0.15
		assert.True(t, mustDecimal(t, "1001.5").Equal(receipt.Total))
		assert.Equal(t, primaryID, receipt.PrimaryAccountID)

		require.Len(t, accounts.debitCalls, 1)
		assert.True(t, mustDecimal(t, "1000").Equal(accounts.debitCalls[0]))
		require.Len(t, accounts.creditCalls, 2)
		assert.True(t, mustDecimal(t, "1000").Equal(accounts.creditCalls[0]), "principal leg")
		assert.True(t, mustDecimal(t, "1.5").Equal(accounts.creditCalls[1]), "interest leg")

		require.Len(t, ledger.created, 3)
		debit, principalCredit, interestCredit := ledger.created[0], ledger.created[1], ledger.created[2]
		assert.Equal(t, domain.EntryDebit, debit.Type)
		assert.Equal(t, accountID, debit.AccountID)
		assert.Equal(t, "salak_maturity", debit.ReferenceType)
		assert.True(t, mustDecimal(t, "1000").Equal(debit.Amount))

		assert.Equal(t, domain.EntryCredit, principalCredit.Type)
		assert.Equal(t, primaryID, principalCredit.AccountID)
		assert.Equal(t, "salak_maturity", principalCredit.ReferenceType)
		assert.True(t, mustDecimal(t, "1000").Equal(principalCredit.Amount))
		assert.Equal(t, debit.ReferenceID, principalCredit.ReferenceID, "principal debit/credit share one reference_id")

		assert.Equal(t, domain.EntryCredit, interestCredit.Type)
		assert.Equal(t, primaryID, interestCredit.AccountID)
		assert.Equal(t, "salak_maturity_interest", interestCredit.ReferenceType)
		assert.True(t, mustDecimal(t, "1.5").Equal(interestCredit.Amount))
		assert.Equal(t, debit.ReferenceID, interestCredit.ReferenceID, "interest credit shares the same reference_id too")

		assert.Equal(t, holdingID, salakSvc.lastMarkSettledID)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
