package http

import (
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type loginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type settlementResponse struct {
	HoldingID           uuid.UUID       `json:"holding_id"`
	Principal           decimal.Decimal `json:"principal"`
	Interest            decimal.Decimal `json:"interest"`
	Total               decimal.Decimal `json:"total"`
	PrimaryAccountID    uuid.UUID       `json:"primary_account_id"`
	PrimaryBalanceAfter decimal.Decimal `json:"primary_balance_after"`
	SettledAt           string          `json:"settled_at"`
}

func toSettlementResponse(r transaction.SettlementReceipt) settlementResponse {
	return settlementResponse{
		HoldingID:           r.HoldingID,
		Principal:           r.Principal,
		Interest:            r.Interest,
		Total:               r.Total,
		PrimaryAccountID:    r.PrimaryAccountID,
		PrimaryBalanceAfter: r.PrimaryBalanceAfter,
		SettledAt:           r.SettledAt,
	}
}
