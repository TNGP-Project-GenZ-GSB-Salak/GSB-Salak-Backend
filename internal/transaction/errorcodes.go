package transaction

// Stable, machine-readable codes for transaction-domain errors, mirroring
// internal/kapook/errorcodes.go's convention.
const (
	CodeNoPrimaryAccount      = "transaction_no_primary_account"
	CodeHoldingAlreadySettled = "transaction_holding_already_settled"
)
