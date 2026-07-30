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

type holdingResponse struct {
	ID             uuid.UUID       `json:"id"`
	AccountID      uuid.UUID       `json:"account_id"`
	ProductID      uuid.UUID       `json:"product_id"`
	ProductName    string          `json:"product_name"`
	Units          int64           `json:"units"`
	TicketStart    int64           `json:"ticket_start"`
	TicketEnd      int64           `json:"ticket_end"`
	PurchaseAmount decimal.Decimal `json:"purchase_amount"`
	PurchaseDate   string          `json:"purchase_date"`
	MaturityDate   string          `json:"maturity_date"`
}

const dateOnlyLayout = "2006-01-02"

func toHoldingResponses(holdings []domain.Holding, productNames map[uuid.UUID]string) []holdingResponse {
	out := make([]holdingResponse, 0, len(holdings))
	for _, h := range holdings {
		out = append(out, holdingResponse{
			ID:             h.ID,
			AccountID:      h.AccountID,
			ProductID:      h.ProductID,
			ProductName:    productNames[h.ProductID],
			Units:          h.Units,
			TicketStart:    h.TicketStart,
			TicketEnd:      h.TicketEnd,
			PurchaseAmount: h.PurchaseAmount,
			PurchaseDate:   h.PurchaseDate.Format(dateOnlyLayout),
			MaturityDate:   h.MaturityDate.Format(dateOnlyLayout),
		})
	}
	return out
}
