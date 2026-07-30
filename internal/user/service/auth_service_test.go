package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/user/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/user/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// fakeUserRepo is a hand-rolled in-memory implementation of user.Repository.
type fakeUserRepo struct {
	byUsername map[string]domain.User
	byID       map[uuid.UUID]domain.User

	findByUsernameErr error
	findByIDErr       error
	createErr         error

	lastCreated *domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byUsername: map[string]domain.User{},
		byID:       map[uuid.UUID]domain.User{},
	}
}

func (f *fakeUserRepo) Create(ctx context.Context, tx *gorm.DB, u *domain.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.lastCreated = u
	f.byUsername[u.Username] = *u
	f.byID[u.ID] = *u
	return nil
}

func (f *fakeUserRepo) FindByUsername(ctx context.Context, username string) (domain.User, error) {
	if f.findByUsernameErr != nil {
		return domain.User{}, f.findByUsernameErr
	}
	u, ok := f.byUsername[username]
	if !ok {
		return domain.User{}, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) FindByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	if f.findByIDErr != nil {
		return domain.User{}, f.findByIDErr
	}
	u, ok := f.byID[id]
	if !ok {
		return domain.User{}, gorm.ErrRecordNotFound
	}
	return u, nil
}

func newSigner() *jwtutil.Signer {
	return jwtutil.NewSigner("test-secret", 60)
}

func assertAppErrKind(t *testing.T, err error, kind apperror.Kind) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, kind, appErr.Kind)
}

func TestAuthService_Register(t *testing.T) {
	t.Run("success hashes password and persists user", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := service.NewAuthService(repo, newSigner())

		u, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, u.ID)
		assert.Equal(t, "alice", u.Username)
		assert.Equal(t, "Alice Smith", u.FullName)
		assert.NotEqual(t, "password123", u.PasswordHash)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("password123")))

		require.NotNil(t, repo.lastCreated)
		assert.Equal(t, u.ID, repo.lastCreated.ID)
	})

	t.Run("password exactly at 8-char minimum boundary succeeds", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := service.NewAuthService(repo, newSigner())

		_, err := svc.Register(context.Background(), "bob", "12345678", "Bob Jones")
		require.NoError(t, err)
	})

	cases := []struct {
		name     string
		username string
		password string
		fullName string
	}{
		{"blank username", "", "password123", "Full Name"},
		{"blank password", "alice", "", "Full Name"},
		{"blank full name", "alice", "password123", ""},
		{"all blank", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeUserRepo()
			svc := service.NewAuthService(repo, newSigner())

			_, err := svc.Register(context.Background(), tc.username, tc.password, tc.fullName)
			assertAppErrKind(t, err, apperror.KindValidation)
		})
	}

	t.Run("password below 8-char minimum is rejected", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := service.NewAuthService(repo, newSigner())

		_, err := svc.Register(context.Background(), "alice", "1234567", "Alice Smith")
		assertAppErrKind(t, err, apperror.KindValidation)
	})

	t.Run("username already taken returns conflict", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.byUsername["alice"] = domain.User{ID: uuid.New(), Username: "alice"}
		svc := service.NewAuthService(repo, newSigner())

		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		assertAppErrKind(t, err, apperror.KindConflict)
	})

	t.Run("unexpected error checking username returns internal error", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.findByUsernameErr = errors.New("connection reset")
		svc := service.NewAuthService(repo, newSigner())

		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("repo create failure returns internal error", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.createErr = errors.New("write failed")
		svc := service.NewAuthService(repo, newSigner())

		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

func TestAuthService_Login(t *testing.T) {
	t.Run("success returns user and a valid token", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := service.NewAuthService(repo, newSigner())

		registered, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		require.NoError(t, err)

		u, token, err := svc.Login(context.Background(), "alice", "password123")
		require.NoError(t, err)
		assert.Equal(t, registered.ID, u.ID)
		assert.NotEmpty(t, token)

		signer := newSigner()
		gotID, err := signer.Parse(token)
		require.NoError(t, err)
		assert.Equal(t, registered.ID, gotID)
	})

	t.Run("unknown username returns unauthorized, not a leaky not-found", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := service.NewAuthService(repo, newSigner())

		_, _, err := svc.Login(context.Background(), "ghost", "password123")
		assertAppErrKind(t, err, apperror.KindUnauthorized)
	})

	t.Run("wrong password returns unauthorized", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := service.NewAuthService(repo, newSigner())
		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		require.NoError(t, err)

		_, _, err = svc.Login(context.Background(), "alice", "wrong-password")
		assertAppErrKind(t, err, apperror.KindUnauthorized)
	})

	t.Run("unexpected repo error returns internal error", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.findByUsernameErr = errors.New("connection reset")
		svc := service.NewAuthService(repo, newSigner())

		_, _, err := svc.Login(context.Background(), "alice", "password123")
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("empty password never matches any hash", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := service.NewAuthService(repo, newSigner())
		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		require.NoError(t, err)

		_, _, err = svc.Login(context.Background(), "alice", "")
		assertAppErrKind(t, err, apperror.KindUnauthorized)
	})
}

func TestAuthService_GetByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := service.NewAuthService(repo, newSigner())
		registered, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		require.NoError(t, err)

		u, err := svc.GetByID(context.Background(), registered.ID)
		require.NoError(t, err)
		assert.Equal(t, registered.ID, u.ID)
	})

	t.Run("not found", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := service.NewAuthService(repo, newSigner())

		_, err := svc.GetByID(context.Background(), uuid.New())
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("unexpected repo error returns internal error", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.findByIDErr = errors.New("connection reset")
		svc := service.NewAuthService(repo, newSigner())

		_, err := svc.GetByID(context.Background(), uuid.New())
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

func TestAuthService_Register_TrimsNoWhitespace(t *testing.T) {
	// Documents current behavior: whitespace-only fields are NOT treated as
	// blank by the "== \"\"" check, so they pass validation as-is. This guards
	// against a future accidental strings.TrimSpace regressing silently.
	repo := newFakeUserRepo()
	svc := service.NewAuthService(repo, newSigner())

	u, err := svc.Register(context.Background(), "   ", "password123", "Full Name")
	require.NoError(t, err)
	assert.True(t, strings.TrimSpace(u.Username) == "")
}
