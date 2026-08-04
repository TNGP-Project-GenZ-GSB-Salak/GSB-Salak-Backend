package domain_test

import (
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestGoal_AvailableBalance(t *testing.T) {
	tests := []struct {
		name     string
		saving   string
		salak    string
		expected string
	}{
		{"nothing saved or converted", "0", "0", "0"},
		{"saved but nothing converted yet", "3000", "0", "3000"},
		{"partially converted", "3000", "2000", "1000"},
		{"fully converted", "5000", "5000", "0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := domain.Goal{
				SavingAmount: decimal.RequireFromString(tc.saving),
				SalakAmount:  decimal.RequireFromString(tc.salak),
			}
			assert.True(t, decimal.RequireFromString(tc.expected).Equal(g.AvailableBalance()))
		})
	}
}
