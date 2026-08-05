package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/admin/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/admin/service"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// fakeAdminRepo implements admin.Repository. There is no write path on the
// real Repository interface (admins are seeded, never registered through
// the service), so this fake is seeded directly via byUsername rather than
// through a Create method.
type fakeAdminRepo struct {
	byUsername        map[string]domain.Admin
	findByUsernameErr error
}

func newFakeAdminRepo() *fakeAdminRepo {
	return &fakeAdminRepo{byUsername: map[string]domain.Admin{}}
}

func (f *fakeAdminRepo) FindByUsername(ctx context.Context, username string) (domain.Admin, error) {
	if f.findByUsernameErr != nil {
		return domain.Admin{}, f.findByUsernameErr
	}
	a, ok := f.byUsername[username]
	if !ok {
		return domain.Admin{}, gorm.ErrRecordNotFound
	}
	return a, nil
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	return string(hash)
}

func newAdminSigner() *jwtutil.AdminSigner {
	return jwtutil.NewAdminSigner("test-admin-secret", 60)
}

func assertAppErrKind(t *testing.T, err error, kind apperror.Kind) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, kind, appErr.Kind)
}

func TestAdminService_Login(t *testing.T) {
	t.Run("success returns admin and a valid admin token", func(t *testing.T) {
		repo := newFakeAdminRepo()
		adminID := uuid.New()
		repo.byUsername["admin"] = domain.Admin{ID: adminID, Username: "admin", PasswordHash: mustHash(t, "correct-password")}
		signer := newAdminSigner()
		svc := service.NewAdminService(repo, signer)

		a, token, err := svc.Login(context.Background(), "admin", "correct-password")
		require.NoError(t, err)
		assert.Equal(t, adminID, a.ID)
		assert.NotEmpty(t, token)

		gotID, err := signer.Parse(token)
		require.NoError(t, err)
		assert.Equal(t, adminID, gotID)
	})

	t.Run("unknown username returns unauthorized, not a leaky not-found", func(t *testing.T) {
		repo := newFakeAdminRepo()
		svc := service.NewAdminService(repo, newAdminSigner())

		_, _, err := svc.Login(context.Background(), "ghost", "correct-password")
		assertAppErrKind(t, err, apperror.KindUnauthorized)
	})

	t.Run("wrong password returns unauthorized - same kind as unknown username", func(t *testing.T) {
		repo := newFakeAdminRepo()
		repo.byUsername["admin"] = domain.Admin{ID: uuid.New(), Username: "admin", PasswordHash: mustHash(t, "correct-password")}
		svc := service.NewAdminService(repo, newAdminSigner())

		_, _, err := svc.Login(context.Background(), "admin", "wrong-password")
		assertAppErrKind(t, err, apperror.KindUnauthorized)
	})

	t.Run("empty password never matches any real hash", func(t *testing.T) {
		repo := newFakeAdminRepo()
		repo.byUsername["admin"] = domain.Admin{ID: uuid.New(), Username: "admin", PasswordHash: mustHash(t, "correct-password")}
		svc := service.NewAdminService(repo, newAdminSigner())

		_, _, err := svc.Login(context.Background(), "admin", "")
		assertAppErrKind(t, err, apperror.KindUnauthorized)
	})

	t.Run("unexpected repo error returns internal error, distinguished from not-found", func(t *testing.T) {
		repo := newFakeAdminRepo()
		repo.findByUsernameErr = errors.New("connection reset")
		svc := service.NewAdminService(repo, newAdminSigner())

		_, _, err := svc.Login(context.Background(), "admin", "correct-password")
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}
