package domain_test

import (
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/stretchr/testify/assert"
)

func TestHolding_TicketStartID(t *testing.T) {
	cases := []struct {
		name   string
		letter string
		number int64
		want   string
	}{
		{"typical mid-range number", "ก", 7530, "ก0007530"},
		{"zero pads to full width", "ก", 0, "ก0000000"},
		{"exactly 7 digits fills the field with no padding", "ข", 1234567, "ข1234567"},
		{"more than 7 digits is not truncated", "ฮ", 123456789, "ฮ123456789"},
		{"single digit", "ค", 5, "ค0000005"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := domain.Holding{TicketLetter: tc.letter, TicketStart: tc.number}
			assert.Equal(t, tc.want, h.TicketStartID())
		})
	}
}

func TestHolding_TicketEndID(t *testing.T) {
	h := domain.Holding{TicketLetter: "ก", TicketEnd: 42}
	assert.Equal(t, "ก0000042", h.TicketEndID())
}

func TestHolding_StartAndEndShareTheSameLetter(t *testing.T) {
	h := domain.Holding{TicketLetter: "ง", TicketStart: 100, TicketEnd: 105}
	assert.Equal(t, "ง0000100", h.TicketStartID())
	assert.Equal(t, "ง0000105", h.TicketEndID())
}

func TestHolding_TableName(t *testing.T) {
	assert.Equal(t, "salak.holdings", domain.Holding{}.TableName())
}

func TestTicketSequence_TableName(t *testing.T) {
	assert.Equal(t, "salak.ticket_sequence", domain.TicketSequence{}.TableName())
}
