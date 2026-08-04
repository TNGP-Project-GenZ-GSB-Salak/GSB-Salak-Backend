package service

import (
	"context"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/google/uuid"
)

type TermsService struct {
	terms kapook.TermsRepository
}

func NewTermsService(terms kapook.TermsRepository) *TermsService {
	return &TermsService{terms: terms}
}

var _ kapook.Service = (*TermsService)(nil)

func (s *TermsService) Accept(ctx context.Context, userID uuid.UUID) error {
	if err := s.terms.Accept(ctx, userID); err != nil {
		return apperror.Internal("failed to record terms acceptance", err)
	}
	return nil
}

func (s *TermsService) HasAccepted(ctx context.Context, userID uuid.UUID) (bool, error) {
	accepted, err := s.terms.HasAccepted(ctx, userID)
	if err != nil {
		return false, apperror.Internal("failed to check terms acceptance", err)
	}
	return accepted, nil
}
