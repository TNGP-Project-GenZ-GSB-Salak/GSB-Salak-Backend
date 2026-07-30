package http

import (
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type buySalakRequest struct {
	FundingAccountID uuid.UUID       `json:"funding_account_id" validate:"required"`
	SalakAccountID   uuid.UUID       `json:"salak_account_id" validate:"required"`
	ProductID        uuid.UUID       `json:"product_id" validate:"required"`
	Amount           decimal.Decimal `json:"amount" validate:"required"`
}

type buySalakResponse struct {
	ReferenceID                uuid.UUID       `json:"reference_id"`
	ProductName                string          `json:"product_name"`
	Units                      int64           `json:"units"`
	TicketStart                string          `json:"ticket_start"`
	TicketEnd                  string          `json:"ticket_end"`
	Amount                     decimal.Decimal `json:"amount"`
	FundingAccountBalanceAfter decimal.Decimal `json:"funding_account_balance_after"`
	SalakAccountBalanceAfter   decimal.Decimal `json:"salak_account_balance_after"`
	PurchaseDate               string          `json:"purchase_date"`
	MaturityDate               string          `json:"maturity_date"`
}

func toBuySalakResponse(r transaction.BuySalakReceipt) buySalakResponse {
	return buySalakResponse{
		ReferenceID:                r.ReferenceID,
		ProductName:                r.ProductName,
		Units:                      r.Units,
		TicketStart:                r.TicketStart,
		TicketEnd:                  r.TicketEnd,
		Amount:                     r.Amount,
		FundingAccountBalanceAfter: r.FundingAccountBalanceAfter,
		SalakAccountBalanceAfter:   r.SalakAccountBalanceAfter,
		PurchaseDate:               r.PurchaseDate,
		MaturityDate:               r.MaturityDate,
	}
}

type ledgerEntryResponse struct {
	ID           uuid.UUID       `json:"id"`
	AccountID    uuid.UUID       `json:"account_id"`
	HoldingID    *uuid.UUID      `json:"holding_id,omitempty"`
	Type         string          `json:"type"`
	Amount       decimal.Decimal `json:"amount"`
	BalanceAfter decimal.Decimal `json:"balance_after"`
	ReferenceID  uuid.UUID       `json:"reference_id"`
	Description  string          `json:"description"`
	CreatedAt    time.Time       `json:"created_at"`
}

func toLedgerEntryResponse(e domain.LedgerEntry) ledgerEntryResponse {
	return ledgerEntryResponse{
		ID:           e.ID,
		AccountID:    e.AccountID,
		HoldingID:    e.HoldingID,
		Type:         string(e.Type),
		Amount:       e.Amount,
		BalanceAfter: e.BalanceAfter,
		ReferenceID:  e.ReferenceID,
		Description:  e.Description,
		CreatedAt:    e.CreatedAt,
	}
}

func toLedgerEntryResponses(entries []domain.LedgerEntry) []ledgerEntryResponse {
	out := make([]ledgerEntryResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, toLedgerEntryResponse(e))
	}
	return out
}
