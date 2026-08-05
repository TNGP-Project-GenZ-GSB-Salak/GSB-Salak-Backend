package account

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Repository is implemented by the gorm repository and consumed by the service.
type Repository interface {
	Create(ctx context.Context, tx *gorm.DB, a *domain.Account) error
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Account, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.Account, error)
	FindForUpdate(ctx context.Context, tx *gorm.DB, id uuid.UUID) (domain.Account, error)
	// FindPrimaryByUserID returns gorm.ErrRecordNotFound if userID has no
	// account flagged is_primary_account - the zero case the partial unique
	// index permits (it prevents two, not zero).
	FindPrimaryByUserID(ctx context.Context, userID uuid.UUID) (domain.Account, error)
	UpdateBalance(ctx context.Context, tx *gorm.DB, id uuid.UUID, newBalance decimal.Decimal) error
	// NextAccountNumber reserves the next number from accountType's Postgres
	// sequence, formatted with a type-specific prefix - deterministic and
	// collision-free by construction, mirroring salak.ticket_sequence's
	// idiom. tx may be nil to use the ambient handle; a sequence's nextval()
	// is never rolled back by Postgres regardless.
	NextAccountNumber(ctx context.Context, tx *gorm.DB, accountType domain.Type) (string, error)
}

// Service is the public surface the transaction domain (and http layer) depend on.
// Mutating methods take an explicit tx so callers can compose them into one
// Postgres transaction that spans multiple domains/schemas.
type Service interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Account, error)
	GetByID(ctx context.Context, userID, accountID uuid.UUID) (domain.Account, error)
	// GetByIDUnscoped looks up accountID with no ownership check - for
	// trusted, unattended system callers only (the kapook worker, to
	// resolve a claimed goal's owning user), never exposed over HTTP.
	GetByIDUnscoped(ctx context.Context, accountID uuid.UUID) (domain.Account, error)
	// Create opens a new accountType account for userID, numbered from that
	// type's sequence. isPrimary must only ever be true for a savings-type
	// account (the caller's responsibility; a check constraint backs it).
	// Takes an explicit tx so a caller (currently only user.AuthService's
	// registration flow) can compose it into one transaction alongside the
	// user row and its sibling accounts.
	Create(ctx context.Context, tx *gorm.DB, userID uuid.UUID, accountType domain.Type, startingBalance decimal.Decimal, isPrimary bool) (domain.Account, error)
	// GetPrimaryAccount is the only reader of is_primary_account - the seam
	// MVP#1 ticket 11 required, so a real core-banking lookup could replace
	// the column later without touching call sites. Returns apperror.NotFound
	// if userID has none; "every user has one" is established at
	// registration, not the schema, so callers must handle this case rather
	// than assume it away.
	GetPrimaryAccount(ctx context.Context, userID uuid.UUID) (domain.Account, error)
	Debit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error)
	Credit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error)
	// LockForUpdate takes a row lock on accountID without changing its
	// balance, so an orchestrator that debits one account and credits
	// another can lock them in a fixed, money-flow-independent order (lock
	// this account first, regardless of whether its own leg turns out to be
	// the debit or the credit) - avoiding an AB-BA deadlock against another
	// orchestration that touches the same two accounts in the opposite
	// role. Re-locking a row this same tx already holds is a no-op in
	// Postgres, so calling this before Debit/Credit on the same account is
	// always safe.
	LockForUpdate(ctx context.Context, tx *gorm.DB, accountID uuid.UUID) (domain.Account, error)
}
