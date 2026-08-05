//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/config"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/db"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Postgres SQLSTATE error codes asserted against throughout this suite.
// https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	sqlStateUniqueViolation     = "23505"
	sqlStateForeignKeyViolation = "23503"
	sqlStateCheckViolation      = "23514"
	sqlStateExclusionViolation  = "23P01"
)

// requirePgErrorCode fails the test unless err is a *pgconn.PgError with the
// given SQLSTATE code - used to assert that a rejection came from a specific
// real Postgres constraint (unique/FK/check), not merely "some error".
func requirePgErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAsf(t, err, &pgErr, "expected a *pgconn.PgError, got: %v", err)
	require.Equal(t, code, pgErr.Code)
}

// sharedDB is opened once per test binary run (see TestMain) and reused by
// every test's own transaction (see newTestTx). It stays nil if Postgres is
// unreachable, so every test skips individually instead of the whole binary
// aborting at load time.
var sharedDB *gorm.DB

func TestMain(m *testing.M) {
	cfg := config.Load()
	if gdb, err := db.Open(cfg.DB); err == nil {
		if sqlDB, pingErr := gdb.DB(); pingErr == nil && sqlDB.Ping() == nil {
			sharedDB = gdb
		}
	}
	os.Exit(m.Run())
}

// newTestTx gives one test its own transaction, skipping gracefully if the
// integration DB is unreachable, and rolls back on cleanup so nothing the
// test does - including any row locks it takes - ever persists or leaks
// into another test.
func newTestTx(t *testing.T) *gorm.DB {
	t.Helper()
	if sharedDB == nil {
		t.Skip("integration DB unreachable; run `make docker-up migrate-up` first")
	}
	tx := sharedDB.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { tx.Rollback() })
	return tx
}
