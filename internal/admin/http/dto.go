package http

import (
	"time"

	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
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

type stuckGoalResponse struct {
	GoalID          uuid.UUID  `json:"goal_id"`
	AccountID       uuid.UUID  `json:"account_id"`
	Attempts        int        `json:"attempts"`
	LastError       string     `json:"last_error,omitempty"`
	LastAttemptedAt *time.Time `json:"last_attempted_at,omitempty"`
	GoalReachedAt   *time.Time `json:"goal_reached_at,omitempty"`
}

func toStuckGoalResponse(g kapookdomain.Goal) stuckGoalResponse {
	var lastError string
	if g.AutoPurchaseLastError != nil {
		lastError = *g.AutoPurchaseLastError
	}
	return stuckGoalResponse{
		GoalID:          g.ID,
		AccountID:       g.AccountID,
		Attempts:        g.AutoPurchaseAttempts,
		LastError:       lastError,
		LastAttemptedAt: g.AutoPurchaseLastAttemptedAt,
		GoalReachedAt:   g.GoalReachedAt,
	}
}

func toStuckGoalResponses(goals []kapookdomain.Goal) []stuckGoalResponse {
	resp := make([]stuckGoalResponse, len(goals))
	for i, g := range goals {
		resp[i] = toStuckGoalResponse(g)
	}
	return resp
}
