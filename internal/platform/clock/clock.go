// Package clock abstracts time.Now so time-dependent business logic (goal
// countdowns, maturity dates, anniversary fee windows) can be tested against
// a fixed instant instead of the real wall clock.
package clock

import "time"

// Clock returns the current time, always in UTC. UTC+7 conversion happens
// at the display layer only.
type Clock interface {
	Now() time.Time
}

// Real is the production Clock, backed by the real wall clock.
type Real struct{}

func (Real) Now() time.Time {
	return time.Now().UTC()
}

// Fixed is a Clock that always reports the same instant. Useful in tests.
type Fixed time.Time

func (f Fixed) Now() time.Time {
	return time.Time(f).UTC()
}
