package domain

import (
	"time"

	"github.com/google/uuid"
)

type Badge struct {
	ID        uuid.UUID
	Code      string
	Name      string
	ImageURL  string
	Weight    float64
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Badge) TableName() string {
	return "badge.badges"
}

// SalakBadge is the immutable record of which background a Salak card
// (salak.holdings row) was granted - assigned once at mint time and never
// changed afterward. WeightAtAssignment snapshots the badge's probability
// weight at the moment of the random draw, independent of whatever
// badge.badges.weight is edited to later, so the odds behind any specific
// card's background stay auditable even after the live weights change.
type SalakBadge struct {
	ID                 uuid.UUID
	SalakID            uuid.UUID
	BadgeID            uuid.UUID
	WeightAtAssignment float64
	AssignedAt         time.Time
}

func (SalakBadge) TableName() string {
	return "badge.salak_badges"
}
