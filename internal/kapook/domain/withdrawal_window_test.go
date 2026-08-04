package domain_test

import (
	"testing"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/stretchr/testify/assert"
)

func TestWithdrawalWindow(t *testing.T) {
	anchor := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		now           time.Time
		expectedStart time.Time
		expectedEnd   time.Time
	}{
		{
			name:          "at anchor itself",
			now:           anchor,
			expectedStart: anchor,
			expectedEnd:   anchor.AddDate(1, 0, 0),
		},
		{
			name:          "mid-way through the first year",
			now:           anchor.AddDate(0, 6, 0),
			expectedStart: anchor,
			expectedEnd:   anchor.AddDate(1, 0, 0),
		},
		{
			name:          "just before the first anniversary",
			now:           anchor.AddDate(1, 0, 0).Add(-time.Second),
			expectedStart: anchor,
			expectedEnd:   anchor.AddDate(1, 0, 0),
		},
		{
			name:          "exactly at the first anniversary resets the window",
			now:           anchor.AddDate(1, 0, 0),
			expectedStart: anchor.AddDate(1, 0, 0),
			expectedEnd:   anchor.AddDate(2, 0, 0),
		},
		{
			name:          "several anniversaries later",
			now:           anchor.AddDate(3, 6, 0),
			expectedStart: anchor.AddDate(3, 0, 0),
			expectedEnd:   anchor.AddDate(4, 0, 0),
		},
		{
			name:          "now before anchor is treated defensively as the first window",
			now:           anchor.AddDate(0, -1, 0),
			expectedStart: anchor,
			expectedEnd:   anchor.AddDate(1, 0, 0),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end := domain.WithdrawalWindow(anchor, tc.now)
			assert.True(t, tc.expectedStart.Equal(start), "start: expected %s, got %s", tc.expectedStart, start)
			assert.True(t, tc.expectedEnd.Equal(end), "end: expected %s, got %s", tc.expectedEnd, end)
		})
	}
}
