package domain_test

import (
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/stretchr/testify/assert"
)

func TestTermsAcceptance_TableName(t *testing.T) {
	assert.Equal(t, "kapook.terms_acceptances", domain.TermsAcceptance{}.TableName())
}
