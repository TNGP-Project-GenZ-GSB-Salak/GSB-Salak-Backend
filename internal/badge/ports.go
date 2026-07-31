package badge

import (
	"context"

	"github.com/google/uuid"
)

// Repository is implemented by the gorm repository and consumed by the service.
type Repository interface {
	UserOwnsBadge(ctx context.Context, userID, badgeID uuid.UUID) (bool, error)
}

// Service is the public surface other domains (and the http layer) depend on.
type Service interface {
	UserOwnsBadge(ctx context.Context, userID, badgeID uuid.UUID) (bool, error)
}
