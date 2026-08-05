package domain

import (
	"time"

	"github.com/google/uuid"
)

// Admin is a deliberately minimal internal-ops identity - username + bcrypt
// hash, no roles or permissions. Separate from "user".users: admin and
// customer identity are unrelated concepts that happen to both be
// username/password, never joined against each other.
type Admin struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

func (Admin) TableName() string {
	return "admin.admins"
}
