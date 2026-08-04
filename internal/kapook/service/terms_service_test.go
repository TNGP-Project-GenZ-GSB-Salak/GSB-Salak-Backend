package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/service"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTermsRepo is a hand-rolled implementation of kapook.TermsRepository.
type fakeTermsRepo struct {
	accepted     map[uuid.UUID]bool
	acceptErr    error
	hasAcceptErr error

	lastAcceptedUserID    uuid.UUID
	lastHasAcceptedUserID uuid.UUID
	acceptCallCount       int
}

func newFakeTermsRepo() *fakeTermsRepo {
	return &fakeTermsRepo{accepted: map[uuid.UUID]bool{}}
}

func (f *fakeTermsRepo) Accept(ctx context.Context, userID uuid.UUID) error {
	f.acceptCallCount++
	f.lastAcceptedUserID = userID
	if f.acceptErr != nil {
		return f.acceptErr
	}
	f.accepted[userID] = true
	return nil
}

func (f *fakeTermsRepo) HasAccepted(ctx context.Context, userID uuid.UUID) (bool, error) {
	f.lastHasAcceptedUserID = userID
	if f.hasAcceptErr != nil {
		return false, f.hasAcceptErr
	}
	return f.accepted[userID], nil
}

func assertAppErrKind(t *testing.T, err error, kind apperror.Kind) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, kind, appErr.Kind)
}

func TestTermsService_Accept(t *testing.T) {
	userID := uuid.New()

	t.Run("success records acceptance", func(t *testing.T) {
		repo := newFakeTermsRepo()
		svc := service.NewTermsService(repo)

		err := svc.Accept(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, userID, repo.lastAcceptedUserID)
		assert.True(t, repo.accepted[userID])
	})

	t.Run("accepting twice is idempotent - no error, repo called both times", func(t *testing.T) {
		repo := newFakeTermsRepo()
		svc := service.NewTermsService(repo)

		require.NoError(t, svc.Accept(context.Background(), userID))
		require.NoError(t, svc.Accept(context.Background(), userID))
		assert.Equal(t, 2, repo.acceptCallCount, "the service itself doesn't short-circuit a repeat accept - idempotency is the repo's job")
	})

	t.Run("repo error returns internal error", func(t *testing.T) {
		repo := newFakeTermsRepo()
		repo.acceptErr = errors.New("db down")
		svc := service.NewTermsService(repo)

		err := svc.Accept(context.Background(), userID)
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

func TestTermsService_HasAccepted(t *testing.T) {
	userID := uuid.New()

	t.Run("false before acceptance", func(t *testing.T) {
		repo := newFakeTermsRepo()
		svc := service.NewTermsService(repo)

		got, err := svc.HasAccepted(context.Background(), userID)
		require.NoError(t, err)
		assert.False(t, got)
		assert.Equal(t, userID, repo.lastHasAcceptedUserID)
	})

	t.Run("true after acceptance", func(t *testing.T) {
		repo := newFakeTermsRepo()
		require.NoError(t, repo.Accept(context.Background(), userID))
		svc := service.NewTermsService(repo)

		got, err := svc.HasAccepted(context.Background(), userID)
		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("repo error returns internal error", func(t *testing.T) {
		repo := newFakeTermsRepo()
		repo.hasAcceptErr = errors.New("db down")
		svc := service.NewTermsService(repo)

		_, err := svc.HasAccepted(context.Background(), userID)
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}
