package main

import "time"

// generateDrawDates computes the draw-day calendar for a product across the
// given years, per its term. Deriving this from termMonths is correct for
// today's two products but is a coincidence of today's two งวด, not a rule -
// a future product with a different term needs its own explicit case here,
// not an extension of this formula. Unknown terms produce no dates, since
// there is no rule to guess at.
func generateDrawDates(termMonths int, years []int) []time.Time {
	var dates []time.Time
	for _, year := range years {
		for month := time.January; month <= time.December; month++ {
			day, ok := drawDayFor(termMonths, month)
			if !ok {
				continue
			}
			dates = append(dates, time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
		}
	}
	return dates
}

// drawDayFor returns the day of the month a draw falls on for a product
// term, per the official sales sheets: the 16th for the 1-year product; the
// 1st for the 2-year, except January and May which move to the 2nd.
func drawDayFor(termMonths int, month time.Month) (int, bool) {
	switch termMonths {
	case 12:
		return 16, true
	case 24:
		if month == time.January || month == time.May {
			return 2, true
		}
		return 1, true
	default:
		return 0, false
	}
}
