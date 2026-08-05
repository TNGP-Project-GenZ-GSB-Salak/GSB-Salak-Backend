package kapook

// Stable, machine-readable codes for the ~8-10 Kapook errors a real customer
// action can trigger (internal/kapook/service/kapook_service.go's
// .WithCode(...) call sites) - see
// .scratch/mvp1-frontend-integration-build/issues/01-error-surface-and-error-codes.md.
// Named here, rather than left as string literals at each call site, so the
// producer (kapook_service.go) and every asserter (kapook_service_test.go,
// and the frontend's src/lib/kapookErrorMessages.ts) can't drift apart on a
// typo - the Go side, at least; the frontend copy still has to match these
// values by hand, per the frontend's own no-codegen convention.
const (
	CodeTermsNotAccepted                    = "kapook_terms_not_accepted"
	CodeGoalAlreadyExists                   = "kapook_goal_already_exists"
	CodeAmountMustBePositive                = "kapook_amount_must_be_positive"
	CodeDepositExceedsTarget                = "kapook_deposit_exceeds_target"
	CodeWithdrawalExceedsBalance            = "kapook_withdrawal_exceeds_balance"
	CodeWithdrawalMustBeFullDuringCountdown = "kapook_withdrawal_must_be_full_during_countdown"
	CodeBalanceBelowMinimumPurchase         = "kapook_balance_below_minimum_purchase"
	CodeBuyAmountExceedsBalance             = "kapook_buy_amount_exceeds_balance"
)
