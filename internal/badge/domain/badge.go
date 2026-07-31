package domain

import "github.com/google/uuid"

type Badge struct {
	ID     uuid.UUID
	Weight float64
}
