package main

import (
	"context"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
)

// drawDateCoverageWarnings checks how many days of draw-date calendar
// coverage remain for each active product, returning one warning per
// product whose furthest seeded date is closer than `within` to now (or has
// no rows at all). An exhausted salak.draw_dates table fails open - the
// guard just stops blocking anything, silently - so this is the startup
// check that catches a thin calendar before it runs out unnoticed.
func drawDateCoverageWarnings(ctx context.Context, products []domain.Product, drawDates salak.DrawDateRepository, now time.Time, within time.Duration) []string {
	var warnings []string
	for _, p := range products {
		furthest, ok, err := drawDates.FurthestDrawDate(ctx, p.ID)
		if err != nil {
			warnings = append(warnings, "failed to check draw-date coverage for "+p.Code+": "+err.Error())
			continue
		}
		if !ok {
			warnings = append(warnings, "product "+p.Code+" has no draw dates seeded at all - the draw-day guard is a no-op for it")
			continue
		}
		if furthest.Before(now.Add(within)) {
			warnings = append(warnings, "product "+p.Code+"'s draw-date calendar runs out "+furthest.Format("2006-01-02")+" - re-seed soon, or the guard fails open")
		}
	}
	return warnings
}
