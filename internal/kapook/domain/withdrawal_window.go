package domain

import "time"

// WithdrawalWindow returns the [start, end) bounds of the rolling 12-month
// free-withdrawal window containing now, anchored on a goal's creation
// instant. The window resets on each anniversary, so a five-year-old goal
// sits in its fifth window rather than perpetually spanning "the last 12
// months" - each anniversary starts a fresh allowance.
func WithdrawalWindow(anchor, now time.Time) (start, end time.Time) {
	start = anchor
	for !start.AddDate(1, 0, 0).After(now) {
		start = start.AddDate(1, 0, 0)
	}
	return start, start.AddDate(1, 0, 0)
}
