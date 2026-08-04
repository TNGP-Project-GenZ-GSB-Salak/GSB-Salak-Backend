package domain_test

import (
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/stretchr/testify/assert"
)

func TestDrawDate_TableName(t *testing.T) {
	assert.Equal(t, "salak.draw_dates", domain.DrawDate{}.TableName())
}
