package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateDrawDates_OneYearProduct_SixteenthEveryMonth(t *testing.T) {
	got := generateDrawDates(12, []int{2026})

	assert.Len(t, got, 12)
	for i, d := range got {
		want := time.Date(2026, time.Month(i+1), 16, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, want, d)
	}
}

func TestGenerateDrawDates_TwoYearProduct_FirstExceptJanuaryAndMayWhichAreSecond(t *testing.T) {
	got := generateDrawDates(24, []int{2026})

	assert.Len(t, got, 12)
	for i, d := range got {
		month := time.Month(i + 1)
		wantDay := 1
		if month == time.January || month == time.May {
			wantDay = 2
		}
		want := time.Date(2026, month, wantDay, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, want, d, "month %s", month)
	}
}

func TestGenerateDrawDates_MultipleYears_CoversEachYearInFull(t *testing.T) {
	got := generateDrawDates(12, []int{2026, 2027})

	assert.Len(t, got, 24)
	assert.Equal(t, time.Date(2026, time.January, 16, 0, 0, 0, 0, time.UTC), got[0])
	assert.Equal(t, time.Date(2027, time.December, 16, 0, 0, 0, 0, time.UTC), got[23])
}

func TestGenerateDrawDates_UnknownTerm_ProducesNoDates(t *testing.T) {
	got := generateDrawDates(6, []int{2026})
	assert.Empty(t, got)
}
