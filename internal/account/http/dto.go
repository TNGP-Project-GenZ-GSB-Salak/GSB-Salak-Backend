package http

import (
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type accountResponse struct {
	ID               uuid.UUID       `json:"id"`
	AccountNumber    string          `json:"account_number"`
	Type             domain.Type     `json:"type"`
	Balance          decimal.Decimal `json:"balance"`
	Currency         string          `json:"currency"`
	IsPrimaryAccount bool            `json:"is_primary_account"`
	CreatedAt        time.Time       `json:"created_at"`
}

func toAccountResponse(a domain.Account) accountResponse {
	return accountResponse{
		ID:               a.ID,
		AccountNumber:    a.AccountNumber,
		Type:             a.Type,
		Balance:          a.Balance,
		Currency:         a.Currency,
		IsPrimaryAccount: a.IsPrimaryAccount,
		CreatedAt:        a.CreatedAt,
	}
}

func toAccountResponses(accounts []domain.Account) []accountResponse {
	out := make([]accountResponse, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, toAccountResponse(a))
	}
	return out
}
