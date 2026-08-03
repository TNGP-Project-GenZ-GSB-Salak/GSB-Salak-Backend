package service

import (
	"fmt"
	"math/rand/v2"
	"sync"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/badge/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/chooser"
)

type RandomBadgeService interface {
	GetRandomBadge() (domain.Badge, error)
}

// WeightedRandomBadgeService picks a badge with probability proportional to
// its Weight. rand.Rand (v2) is not safe for concurrent use, so access to it
// is serialized with mu.
type WeightedRandomBadgeService struct {
	mu      sync.Mutex
	rand    *rand.Rand
	chooser *chooser.Chooser
	badges  []domain.Badge
}

var _ RandomBadgeService = (*WeightedRandomBadgeService)(nil)

func NewWeightedRandomBadgeService(rand *rand.Rand, badges []domain.Badge) (*WeightedRandomBadgeService, error) {
	weights := make([]float64, len(badges))
	for i, badge := range badges {
		weights[i] = badge.Weight
	}

	c, err := chooser.NewChooser(weights)
	if err != nil {
		return nil, fmt.Errorf("invalid badge weights: %w", err)
	}

	return &WeightedRandomBadgeService{
		rand:    rand,
		chooser: c,
		badges:  badges,
	}, nil
}

func (s *WeightedRandomBadgeService) GetRandomBadge() (domain.Badge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	randomIndex := s.chooser.Pick(s.rand)
	return s.badges[randomIndex], nil
}
