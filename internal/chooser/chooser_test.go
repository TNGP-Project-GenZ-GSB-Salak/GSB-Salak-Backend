package chooser_test

import (
	"math/rand/v2"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/chooser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChooser_EmptyWeightsRejected(t *testing.T) {
	_, err := chooser.NewChooser(nil)
	assert.Error(t, err)
}

func TestNewChooser_NegativeWeightRejected(t *testing.T) {
	_, err := chooser.NewChooser([]float64{1, -0.5, 2})
	assert.Error(t, err)
}

func TestNewChooser_AllZeroWeightsRejected(t *testing.T) {
	_, err := chooser.NewChooser([]float64{0, 0, 0})
	assert.Error(t, err)
}

func TestNewChooser_SingleWeightAlwaysPicksIndexZero(t *testing.T) {
	c, err := chooser.NewChooser([]float64{5})
	require.NoError(t, err)

	r := rand.New(rand.NewPCG(1, 1))
	for i := 0; i < 100; i++ {
		assert.Equal(t, 0, c.Pick(r))
	}
}

func TestNewChooser_ZeroWeightEntryIsNeverPicked(t *testing.T) {
	// A mix of a zero-weight and positive-weight entry is valid (total > 0);
	// only the positive-weight index should ever be picked.
	c, err := chooser.NewChooser([]float64{0, 1})
	require.NoError(t, err)

	r := rand.New(rand.NewPCG(2, 2))
	for i := 0; i < 1000; i++ {
		assert.Equal(t, 1, c.Pick(r))
	}
}

func TestChooser_Pick_DistributionMatchesWeights(t *testing.T) {
	c, err := chooser.NewChooser([]float64{0.5, 0.3, 0.2})
	require.NoError(t, err)

	const n = 10_000
	counts := make([]int, 3)
	r := rand.New(rand.NewPCG(3, 3))
	for i := 0; i < n; i++ {
		counts[c.Pick(r)]++
	}

	assertWithinTolerance := func(got int, want float64, tolerance float64) {
		t.Helper()
		lo := want * (1 - tolerance)
		hi := want * (1 + tolerance)
		assert.True(t, float64(got) >= lo && float64(got) <= hi, "count %d outside expected range [%.0f, %.0f]", got, lo, hi)
	}

	assertWithinTolerance(counts[0], n*0.5, 0.05)
	assertWithinTolerance(counts[1], n*0.3, 0.05)
	assertWithinTolerance(counts[2], n*0.2, 0.05)
}
