package domain

import "github.com/google/uuid"

type Badge struct {
	ID       uuid.UUID
	ImageURL string
	Weight   float64
}

type SalakBadge struct {
	SalakID uuid.UUID
	BadgeID uuid.UUID
}
