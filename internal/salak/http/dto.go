package http

import (
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type productResponse struct {
	ID          uuid.UUID       `json:"id"`
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	TermMonths  int             `json:"term_months"`
	UnitPrice   decimal.Decimal `json:"unit_price"`
	MinPurchase decimal.Decimal `json:"min_purchase"`
	MaxPurchase decimal.Decimal `json:"max_purchase"`
	StepAmount  decimal.Decimal `json:"step_amount"`
}

func toProductResponse(p domain.Product) productResponse {
	return productResponse{
		ID:          p.ID,
		Code:        p.Code,
		Name:        p.Name,
		TermMonths:  p.TermMonths,
		UnitPrice:   p.UnitPrice,
		MinPurchase: p.MinPurchase,
		MaxPurchase: p.MaxPurchase,
		StepAmount:  p.StepAmount,
	}
}

func toProductResponses(products []domain.Product) []productResponse {
	out := make([]productResponse, 0, len(products))
	for _, p := range products {
		out = append(out, toProductResponse(p))
	}
	return out
}
