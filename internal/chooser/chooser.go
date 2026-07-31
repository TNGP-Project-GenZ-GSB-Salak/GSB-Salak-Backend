package chooser

import (
	"errors"
	"math/rand/v2"
	"sort"
)

type Chooser struct {
	cum []float64 // cumulative weights
}

// NewChooser builds a weighted-random-index picker from weights. It validates
// once at construction time so Pick can never index out of range: an empty
// weights slice, any negative weight, or a total weight of zero (every entry
// would be equally impossible to reach) are all rejected here instead of
// surfacing as a panic deep inside Pick.
func NewChooser(weights []float64) (*Chooser, error) {
	if len(weights) == 0 {
		return nil, errors.New("chooser: weights must not be empty")
	}

	cum := make([]float64, len(weights))
	total := 0.0
	for i, w := range weights {
		if w < 0 {
			return nil, errors.New("chooser: weights must not be negative")
		}
		total += w
		cum[i] = total
	}
	if total == 0 {
		return nil, errors.New("chooser: at least one weight must be greater than zero")
	}

	return &Chooser{cum: cum}, nil
}

func (c *Chooser) Pick(r *rand.Rand) int {
	x := r.Float64() * c.cum[len(c.cum)-1]
	return sort.Search(len(c.cum), func(i int) bool { return c.cum[i] > x })
}
