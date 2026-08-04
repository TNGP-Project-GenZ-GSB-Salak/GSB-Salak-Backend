package domain_test

import (
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/stretchr/testify/assert"
)

func TestTransaction_TableName(t *testing.T) {
	assert.Equal(t, "kapook.kapook_transactions", domain.Transaction{}.TableName())
}
