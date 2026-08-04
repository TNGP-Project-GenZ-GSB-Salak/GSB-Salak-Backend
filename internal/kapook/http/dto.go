package http

import (
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
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

type withdrawRequest struct {
	KapookAccountID  uuid.UUID       `json:"kapook_account_id" validate:"required"`
	SavingsAccountID uuid.UUID       `json:"savings_account_id" validate:"required"`
	Amount           decimal.Decimal `json:"amount" validate:"required"`
}

type withdrawResponse struct {
	Goal        goalResponse    `json:"goal"`
	Amount      decimal.Decimal `json:"amount"`
	FeeCharged  bool            `json:"fee_charged"`
	FeeAmount   decimal.Decimal `json:"fee_amount"`
	NetCredited decimal.Decimal `json:"net_credited"`
}

func toWithdrawResponse(r kapook.WithdrawResult) withdrawResponse {
	return withdrawResponse{
		Goal:        toGoalResponse(r.Goal),
		Amount:      r.Amount,
		FeeCharged:  r.FeeCharged,
		FeeAmount:   r.FeeAmount,
		NetCredited: r.NetCredited,
	}
}

type withdrawalStatusResponse struct {
	WindowStart              time.Time `json:"window_start"`
	WindowEnd                time.Time `json:"window_end"`
	FreeWithdrawalsUsed      int       `json:"free_withdrawals_used"`
	FreeWithdrawalsRemaining int       `json:"free_withdrawals_remaining"`
	NextWithdrawalIsFree     bool      `json:"next_withdrawal_is_free"`
}

func toWithdrawalStatusResponse(s kapook.WithdrawalStatus) withdrawalStatusResponse {
	return withdrawalStatusResponse{
		WindowStart:              s.WindowStart,
		WindowEnd:                s.WindowEnd,
		FreeWithdrawalsUsed:      s.FreeWithdrawalsUsed,
		FreeWithdrawalsRemaining: s.FreeWithdrawalsRemaining,
		NextWithdrawalIsFree:     s.NextWithdrawalIsFree,
	}
}

type buyFromGoalRequest struct {
	KapookAccountID uuid.UUID       `json:"kapook_account_id" validate:"required"`
	SalakAccountID  uuid.UUID       `json:"salak_account_id" validate:"required"`
	Amount          decimal.Decimal `json:"amount" validate:"required"`
}

type buyFromGoalResponse struct {
	Goal          goalResponse    `json:"goal"`
	GoalCompleted bool            `json:"goal_completed"`
	ReferenceID   uuid.UUID       `json:"reference_id"`
	ProductName   string          `json:"product_name"`
	Units         int64           `json:"units"`
	TicketStart   string          `json:"ticket_start"`
	TicketEnd     string          `json:"ticket_end"`
	Amount        decimal.Decimal `json:"amount"`
	PurchaseDate  string          `json:"purchase_date"`
	MaturityDate  string          `json:"maturity_date"`
}

func toBuyFromGoalResponse(r kapook.BuyFromGoalResult) buyFromGoalResponse {
	receipt := r.Receipt
	return buyFromGoalResponse{
		Goal:          toGoalResponse(r.Goal),
		GoalCompleted: r.GoalCompleted,
		ReferenceID:   receipt.ReferenceID,
		ProductName:   receipt.ProductName,
		Units:         receipt.Units,
		TicketStart:   receipt.TicketStart,
		TicketEnd:     receipt.TicketEnd,
		Amount:        receipt.Amount,
		PurchaseDate:  receipt.PurchaseDate,
		MaturityDate:  receipt.MaturityDate,
	}
}
