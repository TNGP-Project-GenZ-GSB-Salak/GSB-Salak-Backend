//go:build integration

package integration

import (
	"context"
	"testing"

	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	accountrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/account/repository"
	accountservice "github.com/ciaabcdefg/gsb-salak-backend/internal/account/service"
	kapookrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/repository"
	kapookservice "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/service"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	salakrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/repository"
	salakservice "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/service"
	txrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/transaction/repository"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newKapookService wires the *real* KapookService - not fakes - the same
// way cmd/api/main.go does, with every repo constructed on the given tx
// (including the service's own s.db.Transaction call, which GORM turns into
// a SAVEPOINT since tx is already inside an open transaction - the same
// technique newBuySalakService in buy_salak_flow_test.go uses).
func newKapookService(tx *gorm.DB) (*kapookservice.KapookService, *kapookrepo.GormTermsRepository) {
	accountSvc := accountservice.NewAccountService(accountrepo.NewGormAccountRepository(tx))
	salakSvc := salakservice.NewSalakService(
		salakrepo.NewGormProductRepository(tx),
		salakrepo.NewGormHoldingRepository(tx),
		accountSvc,
		salakrepo.NewGormDrawDateRepository(tx),
		clock.Real{},
	)
	termsRepo := kapookrepo.NewGormTermsRepository(tx)
	goalRepo := kapookrepo.NewGormGoalRepository(tx)
	ledgerRepo := txrepo.NewGormLedgerRepository(tx)
	transactionRepo := kapookrepo.NewGormTransactionRepository(tx)
	return kapookservice.NewKapookService(termsRepo, goalRepo, salakSvc, accountSvc, tx, ledgerRepo, transactionRepo, clock.Real{}), termsRepo
}

func TestKapookGoalFlow_HappyPath_CreatesAndReadsBackActiveGoal(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	account := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))

	created, err := kapookSvc.CreateGoal(ctx, user.ID, account.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)
	assert.True(t, created.IsActive)
	assert.True(t, decimal.Zero.Equal(created.SavingAmount))

	fetched, err := kapookSvc.GetActiveGoal(ctx, user.ID, account.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)
}

func TestKapookGoalFlow_TermsNotAccepted_Rejected(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	account := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, _ := newKapookService(tx)

	_, err := kapookSvc.CreateGoal(ctx, user.ID, account.ID, product.ID, decimal.RequireFromString("5000"))
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindForbidden, appErr.Kind)
}

func TestKapookGoalFlow_SecondActiveGoal_Rejected(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	account := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))

	_, err := kapookSvc.CreateGoal(ctx, user.ID, account.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)

	_, err = kapookSvc.CreateGoal(ctx, user.ID, account.ID, product.ID, decimal.RequireFromString("3000"))
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindConflict, appErr.Kind)
}

// TestKapookGoalFlow_AnotherUsersGoal_NotReadable proves a goal is bound to
// its owning user - a different authenticated user gets the same 404 as
// "no goal at all", never a peek at someone else's goal.
func TestKapookGoalFlow_AnotherUsersGoal_NotReadable(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	owner := mustCreateUser(t, tx, "")
	other := mustCreateUser(t, tx, "")
	account := mustCreateAccount(t, tx, owner.ID, accountdomain.TypeKapook, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, owner.ID))

	_, err := kapookSvc.CreateGoal(ctx, owner.ID, account.ID, product.ID, decimal.RequireFromString("5000"))
	require.NoError(t, err)

	_, err = kapookSvc.GetActiveGoal(ctx, other.ID, account.ID)
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindNotFound, appErr.Kind)
}

func TestKapookGoalFlow_GoalAmountNotStepMultiple_Rejected(t *testing.T) {
	tx := newTestTx(t)
	ctx := context.Background()
	user := mustCreateUser(t, tx, "")
	account := mustCreateAccount(t, tx, user.ID, accountdomain.TypeKapook, decimal.Zero)
	product := mustCreateProduct(t, tx, uniqueProductCode(), decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	kapookSvc, termsRepo := newKapookService(tx)
	require.NoError(t, termsRepo.Accept(ctx, user.ID))

	_, err := kapookSvc.CreateGoal(ctx, user.ID, account.ID, product.ID, decimal.RequireFromString("1500"))
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.KindValidation, appErr.Kind)
}
