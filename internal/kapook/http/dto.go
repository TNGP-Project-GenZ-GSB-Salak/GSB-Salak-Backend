package http

import (
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
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
	// The read model's six derived fields (kapook.GoalSnapshot) - see
	// .scratch/mvp1-frontend-integration-build/issues/05-*.md. AvailableBalance
	// is what a withdrawal or purchase can draw on - NOT SavingAmount, which
	// is cumulative net contribution and never shrinks on a purchase.
	AvailableBalance          decimal.Decimal `json:"available_balance"`
	TargetReached             bool            `json:"target_reached"`
	CountdownRemainingSeconds *int            `json:"countdown_remaining_seconds,omitempty"`
	PurchasedUnits            int64           `json:"purchased_units"`
	PurchasedCount            int             `json:"purchased_count"`
	BuyEligible               bool            `json:"buy_eligible"`
}

func toGoalResponse(snap kapook.GoalSnapshot) goalResponse {
	g := snap.Goal
	return goalResponse{
		ID:                        g.ID,
		AccountID:                 g.AccountID,
		ProductID:                 g.ProductID,
		GoalAmount:                g.GoalAmount,
		SavingAmount:              g.SavingAmount,
		SalakAmount:               g.SalakAmount,
		IsActive:                  g.IsActive,
		GoalReachedAt:             g.GoalReachedAt,
		CreatedAt:                 g.CreatedAt,
		AvailableBalance:          snap.AvailableBalance,
		TargetReached:             snap.TargetReached,
		CountdownRemainingSeconds: snap.CountdownRemainingSeconds,
		PurchasedUnits:            snap.PurchasedUnits,
		PurchasedCount:            snap.PurchasedCount,
		BuyEligible:               snap.BuyEligible,
	}
}

type withdrawRequest struct {
	KapookAccountID uuid.UUID       `json:"kapook_account_id" validate:"required"`
	Amount          decimal.Decimal `json:"amount" validate:"required"`
}

type withdrawResponse struct {
	Goal        goalResponse    `json:"goal"`
	Amount      decimal.Decimal `json:"amount"`
	FeeCharged  bool            `json:"fee_charged"`
	FeeAmount   decimal.Decimal `json:"fee_amount"`
	NetCredited decimal.Decimal `json:"net_credited"`
}

func toWithdrawResponse(r kapook.WithdrawResult, snap kapook.GoalSnapshot) withdrawResponse {
	return withdrawResponse{
		Goal:        toGoalResponse(snap),
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
	// QuotedFeeAmount/QuotedNetAmount are omitted unless the request
	// supplied a candidate amount - see Handler.GetWithdrawalStatus.
	QuotedFeeAmount *decimal.Decimal `json:"quoted_fee_amount,omitempty"`
	QuotedNetAmount *decimal.Decimal `json:"quoted_net_amount,omitempty"`
}

func toWithdrawalStatusResponse(s kapook.WithdrawalStatus) withdrawalStatusResponse {
	resp := withdrawalStatusResponse{
		WindowStart:              s.WindowStart,
		WindowEnd:                s.WindowEnd,
		FreeWithdrawalsUsed:      s.FreeWithdrawalsUsed,
		FreeWithdrawalsRemaining: s.FreeWithdrawalsRemaining,
		NextWithdrawalIsFree:     s.NextWithdrawalIsFree,
	}
	if s.Quote != nil {
		resp.QuotedFeeAmount = &s.Quote.FeeAmount
		resp.QuotedNetAmount = &s.Quote.NetAmount
	}
	return resp
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

func toBuyFromGoalResponse(r kapook.BuyFromGoalResult, snap kapook.GoalSnapshot) buyFromGoalResponse {
	receipt := r.Receipt
	return buyFromGoalResponse{
		Goal:          toGoalResponse(snap),
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

// kapookTransactionResponse is one row of a goal's history.
// IsAutomaticPurchase is nil until ticket 10's worker starts populating it -
// omitted from the JSON envelope entirely rather than serialized as
// `false`, so a client can tell "not yet known" apart from "definitely a
// customer-initiated purchase" once that distinction starts to matter.
type kapookTransactionResponse struct {
	ID                  uuid.UUID       `json:"id"`
	Type                string          `json:"type"`
	Amount              decimal.Decimal `json:"amount"`
	FeeAmount           decimal.Decimal `json:"fee_amount"`
	NetAmount           decimal.Decimal `json:"net_amount"`
	IsAutomaticPurchase *bool           `json:"is_automatic_purchase,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
}

func toKapookTransactionResponse(e kapook.HistoryEntry) kapookTransactionResponse {
	t := e.Transaction
	return kapookTransactionResponse{
		ID:                  t.ID,
		Type:                string(t.Type),
		Amount:              t.Amount,
		FeeAmount:           e.Fee,
		NetAmount:           e.Net,
		IsAutomaticPurchase: t.IsAutomaticPurchase,
		CreatedAt:           t.CreatedAt,
	}
}

func toKapookTransactionResponses(entries []kapook.HistoryEntry) []kapookTransactionResponse {
	resp := make([]kapookTransactionResponse, len(entries))
	for i, e := range entries {
		resp[i] = toKapookTransactionResponse(e)
	}
	return resp
}
