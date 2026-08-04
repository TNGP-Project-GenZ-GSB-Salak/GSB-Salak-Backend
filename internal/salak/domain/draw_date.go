package domain

import (
	"time"

	"github.com/google/uuid"
)

// DrawDate is one day a product cannot be purchased on
// (หยุดรับฝากทุกวันที่ออกรางวัล). Deriving this from a product's TermMonths
// is correct for both of today's products but is a coincidence of today's
// two งวด, not a rule - hence an explicit calendar row per day rather than a
// formula read at guard time.
type DrawDate struct {
	ID        uuid.UUID
	ProductID uuid.UUID
	DrawDate  time.Time
	CreatedAt time.Time
}

func (DrawDate) TableName() string {
	return "salak.draw_dates"
}
