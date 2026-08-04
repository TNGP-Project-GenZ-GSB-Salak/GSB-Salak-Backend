package clock_test

import (
	"testing"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	"github.com/stretchr/testify/assert"
)

func TestReal_ReturnsCurrentTimeInUTC(t *testing.T) {
	before := time.Now().UTC()
	got := (clock.Real{}).Now()
	after := time.Now().UTC()

	assert.Equal(t, time.UTC, got.Location())
	assert.False(t, got.Before(before))
	assert.False(t, got.After(after))
}

func TestFixed_AlwaysReturnsTheSameInstantInUTC(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	instant := time.Date(2026, 8, 4, 12, 0, 0, 0, ict)
	c := clock.Fixed(instant)

	got := c.Now()

	assert.True(t, instant.Equal(got), "expected %s, got %s", instant, got)
	assert.Equal(t, time.UTC, got.Location())

	// Calling it again returns the same instant - it never advances.
	assert.Equal(t, got, c.Now())
}
