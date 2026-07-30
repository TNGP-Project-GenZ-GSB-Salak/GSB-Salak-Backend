//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	userdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/user/domain"
	userrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/user/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserRepo_CreateAndFind_RoundTrip(t *testing.T) {
	tx := newTestTx(t)
	repo := userrepo.NewGormUserRepository(tx)

	want := &userdomain.User{
		ID:           uuid.New(),
		Username:     "it_" + uuid.New().String(),
		PasswordHash: "hash-value",
		FullName:     "Integration Test User",
	}
	require.NoError(t, repo.Create(context.Background(), tx, want))

	byUsername, err := repo.FindByUsername(context.Background(), want.Username)
	require.NoError(t, err)
	assert.Equal(t, want.ID, byUsername.ID)
	assert.Equal(t, want.FullName, byUsername.FullName)

	byID, err := repo.FindByID(context.Background(), want.ID)
	require.NoError(t, err)
	assert.Equal(t, want.Username, byID.Username)
}

func TestUserRepo_Create_DuplicateUsernameRejected(t *testing.T) {
	tx := newTestTx(t)
	repo := userrepo.NewGormUserRepository(tx)

	username := "it_" + uuid.New().String()
	first := &userdomain.User{ID: uuid.New(), Username: username, PasswordHash: "h", FullName: "First"}
	require.NoError(t, repo.Create(context.Background(), tx, first))

	second := &userdomain.User{ID: uuid.New(), Username: username, PasswordHash: "h", FullName: "Second"}
	err := repo.Create(context.Background(), tx, second)
	requirePgErrorCode(t, err, sqlStateUniqueViolation)
}

func TestUserRepo_FindByUsername_NotFoundReturnsGormSentinel(t *testing.T) {
	tx := newTestTx(t)
	repo := userrepo.NewGormUserRepository(tx)

	_, err := repo.FindByUsername(context.Background(), "no-such-user-"+uuid.New().String())
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestUserRepo_FindByID_NotFoundReturnsGormSentinel(t *testing.T) {
	tx := newTestTx(t)
	repo := userrepo.NewGormUserRepository(tx)

	_, err := repo.FindByID(context.Background(), uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}
