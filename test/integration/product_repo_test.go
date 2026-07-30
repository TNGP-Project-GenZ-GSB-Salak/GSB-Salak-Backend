//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	salakdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	salakrepo "github.com/ciaabcdefg/gsb-salak-backend/internal/salak/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProductRepo_Upsert_SecondCallUpdatesExistingRowOnConflict(t *testing.T) {
	tx := newTestTx(t)
	repo := salakrepo.NewGormProductRepository(tx)
	code := uniqueProductCode()

	original := mustCreateProduct(t, tx, code, decimal.RequireFromString("100"), decimal.RequireFromString("1000"), decimal.RequireFromString("10000"), decimal.RequireFromString("1000"))

	updated := &salakdomain.Product{
		ID:          uuid.New(), // deliberately different - must be ignored in favor of the existing row's id
		Code:        code,
		Name:        "Updated Name",
		TermMonths:  24,
		UnitPrice:   decimal.RequireFromString("200"),
		MinPurchase: decimal.RequireFromString("2000"),
		MaxPurchase: decimal.RequireFromString("20000"),
		StepAmount:  decimal.RequireFromString("2000"),
		IsActive:    true,
	}
	require.NoError(t, repo.Upsert(context.Background(), updated))

	got, err := repo.FindByID(context.Background(), original.ID)
	require.NoError(t, err, "the original row's id must still exist and now carry the updated fields")
	assert.Equal(t, "Updated Name", got.Name)
	assert.Equal(t, 24, got.TermMonths)
	assert.True(t, decimal.RequireFromString("200").Equal(got.UnitPrice))

	_, err = repo.FindByID(context.Background(), updated.ID)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound), "the second call's own id must never have been inserted as a separate row")
}

func TestProductRepo_Upsert_InvalidTermMonthsRejectedByCheckConstraint(t *testing.T) {
	tx := newTestTx(t)
	repo := salakrepo.NewGormProductRepository(tx)

	p := &salakdomain.Product{
		ID:          uuid.New(),
		Code:        uniqueProductCode(),
		Name:        "Bad Term",
		TermMonths:  18, // only 12 or 24 allowed
		UnitPrice:   decimal.RequireFromString("100"),
		MinPurchase: decimal.RequireFromString("1000"),
		MaxPurchase: decimal.RequireFromString("10000"),
		StepAmount:  decimal.RequireFromString("1000"),
		IsActive:    true,
	}
	err := repo.Upsert(context.Background(), p)
	requirePgErrorCode(t, err, sqlStateCheckViolation)
}

func TestProductRepo_Upsert_MaxLessThanMinRejectedByCheckConstraint(t *testing.T) {
	tx := newTestTx(t)
	repo := salakrepo.NewGormProductRepository(tx)

	p := &salakdomain.Product{
		ID:          uuid.New(),
		Code:        uniqueProductCode(),
		Name:        "Bad Range",
		TermMonths:  12,
		UnitPrice:   decimal.RequireFromString("100"),
		MinPurchase: decimal.RequireFromString("10000"),
		MaxPurchase: decimal.RequireFromString("1000"), // max < min
		StepAmount:  decimal.RequireFromString("1000"),
		IsActive:    true,
	}
	err := repo.Upsert(context.Background(), p)
	requirePgErrorCode(t, err, sqlStateCheckViolation)
}

func TestProductRepo_ListActive_FiltersAndOrdersByTermMonths(t *testing.T) {
	tx := newTestTx(t)
	repo := salakrepo.NewGormProductRepository(tx)

	newProduct := func(termMonths int, isActive bool) *salakdomain.Product {
		return &salakdomain.Product{
			ID:          uuid.New(),
			Code:        uniqueProductCode(),
			Name:        "Test Product",
			TermMonths:  termMonths,
			UnitPrice:   decimal.RequireFromString("100"),
			MinPurchase: decimal.RequireFromString("1000"),
			MaxPurchase: decimal.RequireFromString("10000"),
			StepAmount:  decimal.RequireFromString("1000"),
			IsActive:    isActive,
		}
	}

	active24 := newProduct(24, true)
	require.NoError(t, repo.Upsert(context.Background(), active24))
	active12 := newProduct(12, true)
	require.NoError(t, repo.Upsert(context.Background(), active12))
	inactive := newProduct(12, false)
	require.NoError(t, repo.Upsert(context.Background(), inactive))

	got, err := repo.ListActive(context.Background())
	require.NoError(t, err)

	byID := map[uuid.UUID]bool{}
	for _, p := range got {
		byID[p.ID] = true
		assert.True(t, p.IsActive)
	}
	assert.True(t, byID[active24.ID])
	assert.True(t, byID[active12.ID])
	assert.False(t, byID[inactive.ID], "inactive product must not be returned")

	// Among just the two fixtures we control, 12-month must sort before 24-month.
	idx12, idx24 := -1, -1
	for i, p := range got {
		if p.ID == active12.ID {
			idx12 = i
		}
		if p.ID == active24.ID {
			idx24 = i
		}
	}
	require.NotEqual(t, -1, idx12)
	require.NotEqual(t, -1, idx24)
	assert.Less(t, idx12, idx24, "ListActive must order by term_months ASC")
}

func TestProductRepo_FindByID_NotFoundReturnsGormSentinel(t *testing.T) {
	tx := newTestTx(t)
	repo := salakrepo.NewGormProductRepository(tx)

	_, err := repo.FindByID(context.Background(), uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}
