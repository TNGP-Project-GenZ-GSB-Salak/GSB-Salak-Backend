package salak

// Stable, machine-readable codes for the two salak.Service errors that
// internal/kapook/service/kapook_service.go's CreateGoal/BuyFromGoal reach
// via ValidatePurchase/EnsureNotDrawDay - the decision doc behind
// .scratch/mvp1-frontend-integration-build/issues/01-error-surface-and-error-codes.md
// names both ("not a multiple of the step amount", "the draw-day closure")
// among the ~8-10 customer-facing Kapook errors that need a code, even
// though they're constructed here rather than in the kapook package.
const (
	CodeAmountNotStepMultiple  = "salak_amount_not_step_multiple"
	CodeDrawDayPurchaseBlocked = "salak_draw_day_purchase_blocked"
)
