package domain

import (
	"time"

	"github.com/google/uuid"
)

// TermsAcceptance records that a user has accepted the Kapook terms and
// conditions - single blanket acceptance per user, no version or document
// tracking.
type TermsAcceptance struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	AcceptedAt time.Time
}

func (TermsAcceptance) TableName() string {
	return "kapook.terms_acceptances"
}
