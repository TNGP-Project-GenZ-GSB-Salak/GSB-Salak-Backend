package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type fakeDrawDateRepo struct {
	furthest    map[uuid.UUID]time.Time
	furthestErr error
}

func (f *fakeDrawDateRepo) IsDrawDay(ctx context.Context, productID uuid.UUID, date time.Time) (bool, error) {
	return false, nil
}

func (f *fakeDrawDateRepo) Create(ctx context.Context, d *domain.DrawDate) error {
	return nil
}

func (f *fakeDrawDateRepo) FurthestDrawDate(ctx context.Context, productID uuid.UUID) (time.Time, bool, error) {
	if f.furthestErr != nil {
		return time.Time{}, false, f.furthestErr
	}
	d, ok := f.furthest[productID]
	return d, ok, nil
}

func TestDrawDateCoverageWarnings(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	within := 60 * 24 * time.Hour
	product := domain.Product{ID: uuid.New(), Code: "SALAK_1Y"}

	t.Run("plenty of coverage produces no warning", func(t *testing.T) {
		repo := &fakeDrawDateRepo{furthest: map[uuid.UUID]time.Time{
			product.ID: now.Add(365 * 24 * time.Hour),
		}}
		got := drawDateCoverageWarnings(context.Background(), []domain.Product{product}, repo, now, within)
		assert.Empty(t, got)
	})

	t.Run("furthest date within the window warns", func(t *testing.T) {
		repo := &fakeDrawDateRepo{furthest: map[uuid.UUID]time.Time{
			product.ID: now.Add(10 * 24 * time.Hour),
		}}
		got := drawDateCoverageWarnings(context.Background(), []domain.Product{product}, repo, now, within)
		assert.Len(t, got, 1)
		assert.Contains(t, got[0], "SALAK_1Y")
	})

	t.Run("no rows at all warns", func(t *testing.T) {
		repo := &fakeDrawDateRepo{furthest: map[uuid.UUID]time.Time{}}
		got := drawDateCoverageWarnings(context.Background(), []domain.Product{product}, repo, now, within)
		assert.Len(t, got, 1)
		assert.Contains(t, got[0], "no draw dates seeded")
	})

	t.Run("repo error is surfaced as a warning, not silently dropped", func(t *testing.T) {
		repo := &fakeDrawDateRepo{furthestErr: errors.New("db down")}
		got := drawDateCoverageWarnings(context.Background(), []domain.Product{product}, repo, now, within)
		assert.Len(t, got, 1)
		assert.Contains(t, got[0], "db down")
	})

	t.Run("no active products produces no warnings", func(t *testing.T) {
		repo := &fakeDrawDateRepo{furthest: map[uuid.UUID]time.Time{}}
		got := drawDateCoverageWarnings(context.Background(), nil, repo, now, within)
		assert.Empty(t, got)
	})
}
