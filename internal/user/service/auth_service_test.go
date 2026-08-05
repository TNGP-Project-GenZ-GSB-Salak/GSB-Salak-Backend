package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/user/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/user/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
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

// createdAccount records one fakeAccountService.Create call, in order, so a
// test can assert what Register asked for (type, starting balance, primary
// flag) without a real account.Service/Postgres behind it.
type createdAccount struct {
	UserID          uuid.UUID
	Type            accountdomain.Type
	StartingBalance decimal.Decimal
	IsPrimary       bool
}

// fakeAccountService is a hand-rolled implementation of account.Service;
// AuthService.Register only calls Create, so every other method is a stub.
type fakeAccountService struct {
	createErr error
	created   []createdAccount
}

func (f *fakeAccountService) Create(ctx context.Context, tx *gorm.DB, userID uuid.UUID, accountType accountdomain.Type, startingBalance decimal.Decimal, isPrimary bool) (accountdomain.Account, error) {
	if f.createErr != nil {
		return accountdomain.Account{}, f.createErr
	}
	f.created = append(f.created, createdAccount{UserID: userID, Type: accountType, StartingBalance: startingBalance, IsPrimary: isPrimary})
	return accountdomain.Account{ID: uuid.New(), UserID: userID, Type: accountType, Balance: startingBalance, IsPrimaryAccount: isPrimary}, nil
}

func (f *fakeAccountService) ListByUser(ctx context.Context, userID uuid.UUID) ([]accountdomain.Account, error) {
	return nil, nil
}
func (f *fakeAccountService) GetByID(ctx context.Context, userID, accountID uuid.UUID) (accountdomain.Account, error) {
	return accountdomain.Account{}, nil
}
func (f *fakeAccountService) GetByIDUnscoped(ctx context.Context, accountID uuid.UUID) (accountdomain.Account, error) {
	return accountdomain.Account{}, nil
}
func (f *fakeAccountService) GetPrimaryAccount(ctx context.Context, userID uuid.UUID) (accountdomain.Account, error) {
	return accountdomain.Account{}, nil
}
func (f *fakeAccountService) Debit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	return decimal.Zero, nil
}
func (f *fakeAccountService) Credit(ctx context.Context, tx *gorm.DB, accountID uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error) {
	return decimal.Zero, nil
}
func (f *fakeAccountService) LockForUpdate(ctx context.Context, tx *gorm.DB, accountID uuid.UUID) (accountdomain.Account, error) {
	return accountdomain.Account{}, nil
}

func newSigner() *jwtutil.Signer {
	return jwtutil.NewSigner("test-secret", 60)
}

// newSQLMockDB backs a real *gorm.DB with sqlmock so Register's own
// db.Transaction(...) has BEGIN/COMMIT/ROLLBACK to talk to - mirrors
// internal/kapook/service/kapook_service_test.go's helper of the same name.
func newSQLMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return gdb, mock
}

// newAuthService wires an AuthService for tests against a committing
// transaction (success paths use this with mock.ExpectCommit()).
func newAuthService(repo *fakeUserRepo, accounts *fakeAccountService, db *gorm.DB, startingBalance decimal.Decimal) *service.AuthService {
	return service.NewAuthService(repo, newSigner(), accounts, db, startingBalance)
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
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		u, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, u.ID)
		assert.Equal(t, "alice", u.Username)
		assert.Equal(t, "Alice Smith", u.FullName)
		assert.NotEqual(t, "password123", u.PasswordHash)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("password123")))

		require.NotNil(t, repo.lastCreated)
		assert.Equal(t, u.ID, repo.lastCreated.ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("password exactly at 8-char minimum boundary succeeds", func(t *testing.T) {
		repo := newFakeUserRepo()
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		_, err := svc.Register(context.Background(), "bob", "12345678", "Bob Jones")
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
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
			accounts := &fakeAccountService{}
			// Rejected before the transaction ever opens - no Begin/Commit
			// expected.
			db, mock := newSQLMockDB(t)
			svc := newAuthService(repo, accounts, db, decimal.Zero)

			_, err := svc.Register(context.Background(), tc.username, tc.password, tc.fullName)
			assertAppErrKind(t, err, apperror.KindValidation)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	t.Run("password below 8-char minimum is rejected", func(t *testing.T) {
		repo := newFakeUserRepo()
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		_, err := svc.Register(context.Background(), "alice", "1234567", "Alice Smith")
		assertAppErrKind(t, err, apperror.KindValidation)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("username already taken returns conflict", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.byUsername["alice"] = domain.User{ID: uuid.New(), Username: "alice"}
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		assertAppErrKind(t, err, apperror.KindConflict)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("unexpected error checking username returns internal error", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.findByUsernameErr = errors.New("connection reset")
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		assertAppErrKind(t, err, apperror.KindInternal)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("repo create failure rolls back the transaction", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.createErr = errors.New("write failed")
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		assertAppErrKind(t, err, apperror.KindInternal)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("creates savings, salak and kapook accounts, in that order", func(t *testing.T) {
		repo := newFakeUserRepo()
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()
		svc := newAuthService(repo, accounts, db, decimal.RequireFromString("1000000"))

		u, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())

		require.Len(t, accounts.created, 3)
		for _, a := range accounts.created {
			assert.Equal(t, u.ID, a.UserID)
		}

		savings := accounts.created[0]
		assert.Equal(t, accountdomain.TypeSavings, savings.Type)
		assert.True(t, decimal.RequireFromString("1000000").Equal(savings.StartingBalance))
		assert.True(t, savings.IsPrimary, "the savings account Register opens must be flagged primary")

		salak := accounts.created[1]
		assert.Equal(t, accountdomain.TypeSalak, salak.Type)
		assert.True(t, decimal.Zero.Equal(salak.StartingBalance))
		assert.False(t, salak.IsPrimary)

		kapook := accounts.created[2]
		assert.Equal(t, accountdomain.TypeKapook, kapook.Type)
		assert.True(t, decimal.Zero.Equal(kapook.StartingBalance))
		assert.False(t, kapook.IsPrimary)
	})

	t.Run("committed default starting balance is zero", func(t *testing.T) {
		repo := newFakeUserRepo()
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()
		// NewAuthService's caller (cmd/api/main.go) wires this from
		// config.Config.RegistrationSavingsStartingBalance, whose own
		// zero-value default is what this test exercises directly.
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())

		require.Len(t, accounts.created, 3)
		assert.True(t, decimal.Zero.Equal(accounts.created[0].StartingBalance))
	})

	t.Run("savings account creation failure rolls back before salak/kapook are attempted", func(t *testing.T) {
		repo := newFakeUserRepo()
		accounts := &fakeAccountService{createErr: errors.New("sequence exhausted")}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		assertAppErrKind(t, err, apperror.KindInternal)
		assert.Empty(t, accounts.created, "the failing Create call itself never records anything")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAuthService_Login(t *testing.T) {
	t.Run("success returns user and a valid token", func(t *testing.T) {
		repo := newFakeUserRepo()
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()
		svc := newAuthService(repo, accounts, db, decimal.Zero)

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
		accounts := &fakeAccountService{}
		db, _ := newSQLMockDB(t)
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		_, _, err := svc.Login(context.Background(), "ghost", "password123")
		assertAppErrKind(t, err, apperror.KindUnauthorized)
	})

	t.Run("wrong password returns unauthorized", func(t *testing.T) {
		repo := newFakeUserRepo()
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()
		svc := newAuthService(repo, accounts, db, decimal.Zero)
		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		require.NoError(t, err)

		_, _, err = svc.Login(context.Background(), "alice", "wrong-password")
		assertAppErrKind(t, err, apperror.KindUnauthorized)
	})

	t.Run("unexpected repo error returns internal error", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.findByUsernameErr = errors.New("connection reset")
		accounts := &fakeAccountService{}
		db, _ := newSQLMockDB(t)
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		_, _, err := svc.Login(context.Background(), "alice", "password123")
		assertAppErrKind(t, err, apperror.KindInternal)
	})

	t.Run("empty password never matches any hash", func(t *testing.T) {
		repo := newFakeUserRepo()
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()
		svc := newAuthService(repo, accounts, db, decimal.Zero)
		_, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		require.NoError(t, err)

		_, _, err = svc.Login(context.Background(), "alice", "")
		assertAppErrKind(t, err, apperror.KindUnauthorized)
	})
}

func TestAuthService_GetByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := newFakeUserRepo()
		accounts := &fakeAccountService{}
		db, mock := newSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectCommit()
		svc := newAuthService(repo, accounts, db, decimal.Zero)
		registered, err := svc.Register(context.Background(), "alice", "password123", "Alice Smith")
		require.NoError(t, err)

		u, err := svc.GetByID(context.Background(), registered.ID)
		require.NoError(t, err)
		assert.Equal(t, registered.ID, u.ID)
	})

	t.Run("not found", func(t *testing.T) {
		repo := newFakeUserRepo()
		accounts := &fakeAccountService{}
		db, _ := newSQLMockDB(t)
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		_, err := svc.GetByID(context.Background(), uuid.New())
		assertAppErrKind(t, err, apperror.KindNotFound)
	})

	t.Run("unexpected repo error returns internal error", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.findByIDErr = errors.New("connection reset")
		accounts := &fakeAccountService{}
		db, _ := newSQLMockDB(t)
		svc := newAuthService(repo, accounts, db, decimal.Zero)

		_, err := svc.GetByID(context.Background(), uuid.New())
		assertAppErrKind(t, err, apperror.KindInternal)
	})
}

func TestAuthService_Register_TrimsNoWhitespace(t *testing.T) {
	// Documents current behavior: whitespace-only fields are NOT treated as
	// blank by the "== \"\"" check, so they pass validation as-is. This guards
	// against a future accidental strings.TrimSpace regressing silently.
	repo := newFakeUserRepo()
	accounts := &fakeAccountService{}
	db, mock := newSQLMockDB(t)
	mock.ExpectBegin()
	mock.ExpectCommit()
	svc := newAuthService(repo, accounts, db, decimal.Zero)

	u, err := svc.Register(context.Background(), "   ", "password123", "Full Name")
	require.NoError(t, err)
	assert.True(t, strings.TrimSpace(u.Username) == "")
}
