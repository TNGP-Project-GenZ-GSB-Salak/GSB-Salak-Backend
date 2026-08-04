//go:build integration

package integration

import (
	"context"
	"testing"

	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	kapookrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTermsRepo_HasAccepted_FalseBeforeTrueAfter(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	repo := kapookrepo.NewGormTermsRepository(tx)

	got, err := repo.HasAccepted(ctx, user.ID)
	require.NoError(t, err)
	assert.False(t, got)

	require.NoError(t, repo.Accept(ctx, user.ID))

	got, err = repo.HasAccepted(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, got)
}

// TestTermsRepo_Accept_TwiceIsIdempotent proves the repo's own write path
// (ON CONFLICT DO NOTHING) never errors and never creates a second row on a
// repeat accept - the app-level half of "accepting twice does not create a
// duplicate".
func TestTermsRepo_Accept_TwiceIsIdempotent(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	repo := kapookrepo.NewGormTermsRepository(tx)

	require.NoError(t, repo.Accept(ctx, user.ID))
	require.NoError(t, repo.Accept(ctx, user.ID))

	var count int64
	require.NoError(t, tx.Model(&kapookdomain.TermsAcceptance{}).Where("user_id = ?", user.ID).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

// TestTermsRepo_Create_DuplicateUserIDRejectedByUniqueConstraint proves the
// database-level half: a raw insert that bypasses the repo's ON CONFLICT
// clause still can't produce a second row for the same user, since the
// unique constraint is real, not just app-level discipline.
func TestTermsRepo_Create_DuplicateUserIDRejectedByUniqueConstraint(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")

	first := &kapookdomain.TermsAcceptance{ID: uuid.New(), UserID: user.ID}
	require.NoError(t, tx.Create(first).Error)

	second := &kapookdomain.TermsAcceptance{ID: uuid.New(), UserID: user.ID}
	err := tx.Create(second).Error
	requirePgErrorCode(t, err, sqlStateUniqueViolation)
}
