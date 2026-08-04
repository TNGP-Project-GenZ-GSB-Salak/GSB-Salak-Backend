//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	accountrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/account/repository"
	badgedomain "github.com/ciaabcdefg/gsb-salak-backend/internal/badge/domain"
	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	salakrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/repository"
	userdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/user/domain"
	userrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/user/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// mustCreateUser inserts a user with a random username via tx, failing the
// test immediately on error. Callers that need a distinct username per test
// case can pass one explicitly; an empty string generates one.
func mustCreateUser(t *testing.T, tx *gorm.DB, username string) userdomain.User {
	t.Helper()
	if username == "" {
		username = "it_" + uuid.New().String()
	}
	u := &userdomain.User{
		ID:           uuid.New(),
		Username:     username,
		PasswordHash: "not-a-real-hash",
		FullName:     "Integration Test User",
	}
	require.NoError(t, userrepo.NewGormUserRepository(tx).Create(context.Background(), tx, u))
	return *u
}

// mustCreateAccount inserts an account for the given user, failing the test
// immediately on error. accountType must be "savings" or "salak".
func mustCreateAccount(t *testing.T, tx *gorm.DB, userID uuid.UUID, accountType accountdomain.Type, balance decimal.Decimal) accountdomain.Account {
	t.Helper()
	a := &accountdomain.Account{
		ID:            uuid.New(),
		UserID:        userID,
		AccountNumber: uniqueAccountNumber(),
		Type:          accountType,
		Balance:       balance,
		Currency:      "THB",
	}
	require.NoError(t, accountrepo.NewGormAccountRepository(tx).Create(context.Background(), tx, a))
	return *a
}

// mustCreateProduct inserts an active salak product via Upsert (its only
// write path), failing the test immediately on error.
func mustCreateProduct(t *testing.T, tx *gorm.DB, code string, unitPrice, minPurchase, maxPurchase, stepAmount decimal.Decimal) salakdomain.Product {
	t.Helper()
	p := &salakdomain.Product{
		ID:          uuid.New(),
		Code:        code,
		Name:        "Integration Test Product " + code,
		TermMonths:  12,
		UnitPrice:   unitPrice,
		MinPurchase: minPurchase,
		MaxPurchase: maxPurchase,
		StepAmount:  stepAmount,
		IsActive:    true,
	}
	require.NoError(t, salakrepo.NewGormProductRepository(tx).Upsert(context.Background(), p))
	return *p
}

// mustCreateDrawDate inserts a draw_dates row via its only write path,
// failing the test immediately on error.
func mustCreateDrawDate(t *testing.T, tx *gorm.DB, productID uuid.UUID, date time.Time) salakdomain.DrawDate {
	t.Helper()
	d := &salakdomain.DrawDate{ID: uuid.New(), ProductID: productID, DrawDate: date}
	require.NoError(t, salakrepo.NewGormDrawDateRepository(tx).Create(context.Background(), d))
	return *d
}

// uniqueAccountNumber returns a syntactically-plausible, collision-free
// account number (varchar(20)) for fixtures - a truncated UUID is enough
// since only uniqueness matters, not realism.
func uniqueAccountNumber() string {
	return "IT" + uuid.New().String()[:18]
}

// uniqueProductCode returns a collision-free product code that fits
// salak.products.code's varchar(20) column (a plain "IT_"+uuid string would
// overflow it and hit a string-data-right-truncation error instead of
// whatever constraint a test is actually trying to exercise).
func uniqueProductCode() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")[:20]
}

// mustCreateBadge inserts an active badge catalog row directly via tx,
// failing the test immediately on error. Uses tx.Create directly rather than
// a repository method since badge.Repository intentionally exposes only
// UserOwnsBadge - fixture setup doesn't need to go through it.
func mustCreateBadge(t *testing.T, tx *gorm.DB, code string) badgedomain.Badge {
	t.Helper()
	b := &badgedomain.Badge{
		ID:       uuid.New(),
		Code:     code,
		Name:     "Integration Test Badge " + code,
		ImageURL: "https://example.com/badges/" + code + ".png",
		Weight:   1,
		IsActive: true,
	}
	require.NoError(t, tx.Create(b).Error)
	return *b
}

// mustGrantUserBadge inserts a UserBadge row so userID owns badgeID, failing
// the test immediately on error.
func mustGrantUserBadge(t *testing.T, tx *gorm.DB, userID, badgeID uuid.UUID) badgedomain.UserBadge {
	t.Helper()
	ub := &badgedomain.UserBadge{
		ID:      uuid.New(),
		UserID:  userID,
		BadgeID: badgeID,
	}
	require.NoError(t, tx.Create(ub).Error)
	return *ub
}
