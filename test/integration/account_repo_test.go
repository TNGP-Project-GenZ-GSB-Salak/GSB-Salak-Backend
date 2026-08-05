//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	accountrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/account/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountRepo_CreateAndFindByID_RoundTrip(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")

	repo := accountrepo.NewGormAccountRepository(tx)
	want := &accountdomain.Account{
		ID:            uuid.New(),
		UserID:        user.ID,
		AccountNumber: uniqueAccountNumber(),
		Type:          accountdomain.TypeSavings,
		Balance:       decimal.RequireFromString("1234.56"),
		Currency:      "THB",
	}
	require.NoError(t, repo.Create(context.Background(), tx, want))

	got, err := repo.FindByID(context.Background(), want.ID)
	require.NoError(t, err)
	assert.Equal(t, want.AccountNumber, got.AccountNumber)
	assert.Equal(t, accountdomain.TypeSavings, got.Type)
	assert.True(t, decimal.RequireFromString("1234.56").Equal(got.Balance), "decimal precision must round-trip exactly, got %s", got.Balance)
}

func TestAccountRepo_Create_DuplicateAccountNumberRejected(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")
	repo := accountrepo.NewGormAccountRepository(tx)

	number := uniqueAccountNumber()
	first := &accountdomain.Account{ID: uuid.New(), UserID: user.ID, AccountNumber: number, Type: accountdomain.TypeSavings, Currency: "THB"}
	require.NoError(t, repo.Create(context.Background(), tx, first))

	second := &accountdomain.Account{ID: uuid.New(), UserID: user.ID, AccountNumber: number, Type: accountdomain.TypeSavings, Currency: "THB"}
	err := repo.Create(context.Background(), tx, second)
	requirePgErrorCode(t, err, sqlStateUniqueViolation)
}

func TestAccountRepo_Create_InvalidTypeRejectedByCheckConstraint(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")
	repo := accountrepo.NewGormAccountRepository(tx)

	a := &accountdomain.Account{
		ID:            uuid.New(),
		UserID:        user.ID,
		AccountNumber: uniqueAccountNumber(),
		Type:          accountdomain.Type("checking"), // not "savings" or "salak"
		Currency:      "THB",
	}
	err := repo.Create(context.Background(), tx, a)
	requirePgErrorCode(t, err, sqlStateCheckViolation)
}

func TestAccountRepo_Create_KapookTypeAccepted(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")
	repo := accountrepo.NewGormAccountRepository(tx)

	a := &accountdomain.Account{
		ID:            uuid.New(),
		UserID:        user.ID,
		AccountNumber: uniqueAccountNumber(),
		Type:          accountdomain.TypeKapook,
		Currency:      "THB",
	}
	require.NoError(t, repo.Create(context.Background(), tx, a))

	got, err := repo.FindByID(context.Background(), a.ID)
	require.NoError(t, err)
	assert.Equal(t, accountdomain.TypeKapook, got.Type)
}

func TestAccountRepo_Create_NegativeBalanceRejectedByCheckConstraint(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")
	repo := accountrepo.NewGormAccountRepository(tx)

	a := &accountdomain.Account{
		ID:            uuid.New(),
		UserID:        user.ID,
		AccountNumber: uniqueAccountNumber(),
		Type:          accountdomain.TypeSavings,
		Balance:       decimal.RequireFromString("-0.01"),
		Currency:      "THB",
	}
	err := repo.Create(context.Background(), tx, a)
	requirePgErrorCode(t, err, sqlStateCheckViolation)
}

func TestAccountRepo_Create_IsPrimaryAccountRoundTrips(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")
	repo := accountrepo.NewGormAccountRepository(tx)

	a := &accountdomain.Account{
		ID:               uuid.New(),
		UserID:           user.ID,
		AccountNumber:    uniqueAccountNumber(),
		Type:             accountdomain.TypeSavings,
		Currency:         "THB",
		IsPrimaryAccount: true,
	}
	require.NoError(t, repo.Create(context.Background(), tx, a))

	got, err := repo.FindByID(context.Background(), a.ID)
	require.NoError(t, err)
	assert.True(t, got.IsPrimaryAccount)
}

func TestAccountRepo_Create_SecondPrimaryAccountRejectedByUniqueIndex(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")
	repo := accountrepo.NewGormAccountRepository(tx)

	first := &accountdomain.Account{
		ID: uuid.New(), UserID: user.ID, AccountNumber: uniqueAccountNumber(),
		Type: accountdomain.TypeSavings, Currency: "THB", IsPrimaryAccount: true,
	}
	require.NoError(t, repo.Create(context.Background(), tx, first))

	second := &accountdomain.Account{
		ID: uuid.New(), UserID: user.ID, AccountNumber: uniqueAccountNumber(),
		Type: accountdomain.TypeSavings, Currency: "THB", IsPrimaryAccount: true,
	}
	err := repo.Create(context.Background(), tx, second)
	requirePgErrorCode(t, err, sqlStateUniqueViolation)
}

func TestAccountRepo_Create_TwoUsersEachHaveTheirOwnPrimaryAccount(t *testing.T) {
	tx := newTestTx(t)
	userA := mustCreateUser(t, tx, "")
	userB := mustCreateUser(t, tx, "")
	repo := accountrepo.NewGormAccountRepository(tx)

	for _, userID := range []uuid.UUID{userA.ID, userB.ID} {
		a := &accountdomain.Account{
			ID: uuid.New(), UserID: userID, AccountNumber: uniqueAccountNumber(),
			Type: accountdomain.TypeSavings, Currency: "THB", IsPrimaryAccount: true,
		}
		require.NoError(t, repo.Create(context.Background(), tx, a), "the partial unique index is scoped per user_id, not global")
	}
}

func TestAccountRepo_Create_PrimaryOnNonSavingsRejectedByCheckConstraint(t *testing.T) {
	tx := newTestTx(t)
	user := mustCreateUser(t, tx, "")
	repo := accountrepo.NewGormAccountRepository(tx)

	a := &accountdomain.Account{
		ID: uuid.New(), UserID: user.ID, AccountNumber: uniqueAccountNumber(),
		Type: accountdomain.TypeKapook, Currency: "THB", IsPrimaryAccount: true,
	}
	err := repo.Create(context.Background(), tx, a)
	requirePgErrorCode(t, err, sqlStateCheckViolation)
}

func TestAccountRepo_NextAccountNumber_IsUniqueAndTypePrefixed(t *testing.T) {
	tx := newTestTx(t)
	repo := accountrepo.NewGormAccountRepository(tx)

	savings1, err := repo.NextAccountNumber(context.Background(), tx, accountdomain.TypeSavings)
	require.NoError(t, err)
	savings2, err := repo.NextAccountNumber(context.Background(), tx, accountdomain.TypeSavings)
	require.NoError(t, err)
	salak, err := repo.NextAccountNumber(context.Background(), tx, accountdomain.TypeSalak)
	require.NoError(t, err)
	kapook, err := repo.NextAccountNumber(context.Background(), tx, accountdomain.TypeKapook)
	require.NoError(t, err)

	assert.NotEqual(t, savings1, savings2, "sequential calls must not repeat a number")
	assert.True(t, strings.HasPrefix(savings1, "61"))
	assert.True(t, strings.HasPrefix(savings2, "61"))
	assert.True(t, strings.HasPrefix(salak, "62"))
	assert.True(t, strings.HasPrefix(kapook, "63"))
}

func TestAccountRepo_Create_UnknownUserIDRejectedByForeignKey(t *testing.T) {
	tx := newTestTx(t)
	repo := accountrepo.NewGormAccountRepository(tx)

	a := &accountdomain.Account{
		ID:            uuid.New(),
		UserID:        uuid.New(), // no matching "user".users row
		AccountNumber: uniqueAccountNumber(),
		Type:          accountdomain.TypeSavings,
		Currency:      "THB",
	}
	err := repo.Create(context.Background(), tx, a)
	requirePgErrorCode(t, err, sqlStateForeignKeyViolation)
}

// TestAccountRepo_Debit_NoLostUpdateAcrossSequentialTransactions proves
// FindForUpdate + UpdateBalance don't drop an update when two "debits" run
// as separate, sequentially-committed transactions against the same row -
// this needs a fixture that's actually committed (not the rollback-per-test
// tx), since it must persist across two independent Begin()/Commit() pairs.
func TestAccountRepo_Debit_NoLostUpdateAcrossSequentialTransactions(t *testing.T) {
	if sharedDB == nil {
		t.Skip("integration DB unreachable; run `make docker-up migrate-up` first")
	}

	setupTx := sharedDB.Begin()
	require.NoError(t, setupTx.Error)
	user := mustCreateUser(t, setupTx, "")
	account := mustCreateAccount(t, setupTx, user.ID, accountdomain.TypeSavings, decimal.RequireFromString("100.00"))
	require.NoError(t, setupTx.Commit().Error)
	t.Cleanup(func() {
		sharedDB.Exec(`DELETE FROM account.accounts WHERE id = ?`, account.ID)
		sharedDB.Exec(`DELETE FROM "user".users WHERE id = ?`, user.ID)
	})

	repo := accountrepo.NewGormAccountRepository(sharedDB)
	ctx := context.Background()

	tx1 := sharedDB.Begin()
	require.NoError(t, tx1.Error)
	got1, err := repo.FindForUpdate(ctx, tx1, account.ID)
	require.NoError(t, err)
	require.NoError(t, repo.UpdateBalance(ctx, tx1, account.ID, got1.Balance.Sub(decimal.RequireFromString("30.00"))))
	require.NoError(t, tx1.Commit().Error)

	tx2 := sharedDB.Begin()
	require.NoError(t, tx2.Error)
	got2, err := repo.FindForUpdate(ctx, tx2, account.ID)
	require.NoError(t, err)
	require.NoError(t, repo.UpdateBalance(ctx, tx2, account.ID, got2.Balance.Sub(decimal.RequireFromString("20.00"))))
	require.NoError(t, tx2.Commit().Error)

	final, err := repo.FindByID(ctx, account.ID)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("50.00").Equal(final.Balance), "expected 100-30-20=50, got %s", final.Balance)
}
