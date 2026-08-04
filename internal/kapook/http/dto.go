package http

import (
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type termsStatusResponse struct {
	Accepted bool `json:"accepted"`
}

type createGoalRequest struct {
	AccountID  uuid.UUID       `json:"account_id" validate:"required"`
	ProductID  uuid.UUID       `json:"product_id" validate:"required"`
	GoalAmount decimal.Decimal `json:"goal_amount" validate:"required"`
}

type depositRequest struct {
	KapookAccountID  uuid.UUID       `json:"kapook_account_id" validate:"required"`
	SavingsAccountID uuid.UUID       `json:"savings_account_id" validate:"required"`
	Amount           decimal.Decimal `json:"amount" validate:"required"`
}

type goalResponse struct {
	ID            uuid.UUID       `json:"id"`
	AccountID     uuid.UUID       `json:"account_id"`
	ProductID     uuid.UUID       `json:"product_id"`
	GoalAmount    decimal.Decimal `json:"goal_amount"`
	SavingAmount  decimal.Decimal `json:"saving_amount"`
	SalakAmount   decimal.Decimal `json:"salak_amount"`
	IsActive      bool            `json:"is_active"`
	GoalReachedAt *time.Time      `json:"goal_reached_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

func toGoalResponse(g domain.Goal) goalResponse {
	return goalResponse{
		ID:            g.ID,
		AccountID:     g.AccountID,
		ProductID:     g.ProductID,
		GoalAmount:    g.GoalAmount,
		SavingAmount:  g.SavingAmount,
		SalakAmount:   g.SalakAmount,
		IsActive:      g.IsActive,
		GoalReachedAt: g.GoalReachedAt,
		CreatedAt:     g.CreatedAt,
	}
}
