package kapook

import (
	"context"

	"github.com/google/uuid"
)

// TermsRepository owns the one-row-per-user terms & conditions acceptance
// record.
type TermsRepository interface {
	// Accept records userID's acceptance, idempotently - accepting twice
	// never errors and never creates a second row.
	Accept(ctx context.Context, userID uuid.UUID) error
	HasAccepted(ctx context.Context, userID uuid.UUID) (bool, error)
}

// Service is the public surface the http layer depends on.
type Service interface {
	Accept(ctx context.Context, userID uuid.UUID) error
	HasAccepted(ctx context.Context, userID uuid.UUID) (bool, error)
}
