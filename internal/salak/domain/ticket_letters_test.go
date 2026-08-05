package domain_test

import (
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextLetter_AdvancesNormally(t *testing.T) {
	got, err := domain.NextLetter("ก")
	require.NoError(t, err)
	assert.Equal(t, "ข", got)
}

func TestNextLetter_SkipsExcludedVowelRu(t *testing.T) {
	got, err := domain.NextLetter("ร")
	require.NoError(t, err)
	assert.Equal(t, "ล", got, "ฤ (U+0E24) must be skipped, not landed on")
}

func TestNextLetter_SkipsExcludedVowelLu(t *testing.T) {
	got, err := domain.NextLetter("ล")
	require.NoError(t, err)
	assert.Equal(t, "ว", got, "ฦ (U+0E26) must be skipped, not landed on")
}

func TestNextLetter_PastLastLetterErrors(t *testing.T) {
	_, err := domain.NextLetter("ฮ")
	assert.Error(t, err)
}

func TestNextLetter_InvalidInputErrors(t *testing.T) {
	cases := []string{"", "ab", "ฤ", "ฦ", "a"}
	for _, c := range cases {
		_, err := domain.NextLetter(c)
		assert.Errorf(t, err, "expected an error for input %q", c)
	}
}
