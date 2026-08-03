package service

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/badge"
	"github.com/google/uuid"
)

type BadgeService struct {
	repo badge.Repository
}

func NewBadgeService(repo badge.Repository) *BadgeService {
	return &BadgeService{repo: repo}
}

var _ badge.Service = (*BadgeService)(nil)

func (s *BadgeService) UserOwnsBadge(ctx context.Context, userID, badgeID uuid.UUID) (bool, error) {
	return s.repo.UserOwnsBadge(ctx, userID, badgeID)
}
