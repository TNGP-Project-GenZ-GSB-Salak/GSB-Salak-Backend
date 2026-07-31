package service_test

import (
	"math/rand/v2"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/badge/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/badge/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWeightedRandomBadgeService_EmptyBadgesRejected(t *testing.T) {
	_, err := service.NewWeightedRandomBadgeService(rand.New(rand.NewPCG(0, 0)), nil)
	assert.Error(t, err)
}

func TestNewWeightedRandomBadgeService_AllZeroWeightBadgesRejected(t *testing.T) {
	badges := []domain.Badge{
		{ID: uuid.New(), Weight: 0},
		{ID: uuid.New(), Weight: 0},
	}
	_, err := service.NewWeightedRandomBadgeService(rand.New(rand.NewPCG(0, 0)), badges)
	assert.Error(t, err)
}

func TestNewWeightedRandomBadgeService_NegativeWeightRejected(t *testing.T) {
	badges := []domain.Badge{
		{ID: uuid.New(), Weight: -1},
		{ID: uuid.New(), Weight: 1},
	}
	_, err := service.NewWeightedRandomBadgeService(rand.New(rand.NewPCG(0, 0)), badges)
	assert.Error(t, err)
}

func TestRandomBadgeService_GetRandomBadge(t *testing.T) {
	id0 := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	id1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	badges := []domain.Badge{
		{ID: id0, Weight: 0.5},
		{ID: id1, Weight: 0.3},
		{ID: id2, Weight: 0.2},
	}

	r := rand.New(rand.NewPCG(0, 0))
	badgeService, err := service.NewWeightedRandomBadgeService(r, badges)
	require.NoError(t, err)

	t.Run("valid badge", func(t *testing.T) {
		selectedBadge, err := badgeService.GetRandomBadge()
		require.NoError(t, err)
		assert.Contains(t, badges, selectedBadge)
	})
}

func TestRandomBadgeService_GetRandomBadge_Probability(t *testing.T) {
	n := 10_000

	id0 := uuid.MustParse("00000000-0000-0000-0000-000000000000")
	id1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	badges := []domain.Badge{
		{ID: id0, Weight: 0.5},
		{ID: id1, Weight: 0.3},
		{ID: id2, Weight: 0.2},
	}

	r := rand.New(rand.NewPCG(0, 0))
	badgeService, err := service.NewWeightedRandomBadgeService(r, badges)
	require.NoError(t, err)

	selectionCount := map[uuid.UUID]int{
		id0: 0,
		id1: 0,
		id2: 0,
	}

	for i := 0; i < n; i++ {
		selectedBadge, err := badgeService.GetRandomBadge()
		require.NoError(t, err)
		selectionCount[selectedBadge.ID]++
	}

	assertWithinTolerance := func(got, want int, tolerance float64) {
		t.Helper()
		lo := float64(want) * (1 - tolerance)
		hi := float64(want) * (1 + tolerance)
		if float64(got) < lo || float64(got) > hi {
			t.Errorf("count %d outside expected range [%.0f, %.0f]", got, lo, hi)
		}
	}

	assertWithinTolerance(selectionCount[id0], int(float64(n)*badges[0].Weight), 0.05)
	assertWithinTolerance(selectionCount[id1], int(float64(n)*badges[1].Weight), 0.05)
	assertWithinTolerance(selectionCount[id2], int(float64(n)*badges[2].Weight), 0.05)
}
