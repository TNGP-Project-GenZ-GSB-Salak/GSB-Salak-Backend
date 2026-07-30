package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// thaiConsonantStart/Count span ก (U+0E01) through ฮ (U+0E2E) inclusive -
// exactly the 46 Thai consonant letters, nothing else in that range.
const (
	thaiConsonantStart = 0x0E01
	thaiConsonantCount = 46
)

// randomTicketLetter picks one of the 46 Thai consonants uniformly at
// random (crypto/rand.Int avoids the modulo bias a single random byte % 46
// would introduce). Only the letter is randomized; the numeric part of a
// ticket ID is sequential (see ReserveTicketRange) and already guarantees
// global uniqueness, so the letter needs no uniqueness logic of its own.
func randomTicketLetter() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(thaiConsonantCount))
	if err != nil {
		return "", err
	}
	return string(rune(thaiConsonantStart + n.Int64())), nil
}

type SalakService struct {
	products salak.ProductRepository
	holdings salak.HoldingRepository
	accounts account.Service
}

func NewSalakService(products salak.ProductRepository, holdings salak.HoldingRepository, accounts account.Service) *SalakService {
	return &SalakService{products: products, holdings: holdings, accounts: accounts}
}

var _ salak.Service = (*SalakService)(nil)

func (s *SalakService) ListProducts(ctx context.Context) ([]domain.Product, error) {
	products, err := s.products.ListActive(ctx)
	if err != nil {
		return nil, apperror.Internal("failed to list salak products", err)
	}
	return products, nil
}

func (s *SalakService) GetProduct(ctx context.Context, productID uuid.UUID) (domain.Product, error) {
	p, err := s.products.FindByID(ctx, productID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Product{}, apperror.NotFound("salak product not found")
	} else if err != nil {
		return domain.Product{}, apperror.Internal("failed to look up salak product", err)
	}
	if !p.IsActive {
		return domain.Product{}, apperror.Validation("salak product is not available for purchase")
	}
	return p, nil
}

func (s *SalakService) ValidatePurchase(product domain.Product, amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return apperror.Validation("amount must be greater than zero")
	}
	if amount.LessThan(product.MinPurchase) {
		return apperror.Validation("amount is below the minimum purchase amount")
	}
	if amount.GreaterThan(product.MaxPurchase) {
		return apperror.Validation("amount exceeds the maximum purchase amount")
	}
	if !amount.Mod(product.StepAmount).IsZero() {
		return apperror.Validation("amount must be a multiple of the step amount")
	}
	return nil
}

func (s *SalakService) MintHolding(ctx context.Context, tx *gorm.DB, accountID, productID uuid.UUID, amount decimal.Decimal) (domain.Holding, error) {
	product, err := s.products.FindByID(ctx, productID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Holding{}, apperror.NotFound("salak product not found")
	} else if err != nil {
		return domain.Holding{}, apperror.Internal("failed to look up salak product", err)
	}

	units := amount.Div(product.UnitPrice).IntPart()
	if units <= 0 {
		return domain.Holding{}, apperror.Validation("amount does not correspond to any whole units")
	}

	start, end, err := s.holdings.ReserveTicketRange(ctx, tx, units)
	if err != nil {
		return domain.Holding{}, apperror.Internal("failed to reserve ticket range", err)
	}

	letter, err := randomTicketLetter()
	if err != nil {
		return domain.Holding{}, apperror.Internal("failed to generate ticket letter", err)
	}

	now := time.Now().UTC()
	purchaseDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	holding := &domain.Holding{
		ID:             uuid.New(),
		AccountID:      accountID,
		ProductID:      productID,
		Units:          units,
		TicketLetter:   letter,
		TicketStart:    start,
		TicketEnd:      end,
		PurchaseAmount: amount,
		PurchaseDate:   purchaseDate,
		MaturityDate:   purchaseDate.AddDate(0, product.TermMonths, 0),
		CreatedAt:      now,
	}

	if err := s.holdings.Create(ctx, tx, holding); err != nil {
		return domain.Holding{}, apperror.Internal("failed to create salak holding", err)
	}

	return *holding, nil
}

func (s *SalakService) ListHoldingsByAccount(ctx context.Context, userID, accountID uuid.UUID) ([]domain.Holding, error) {
	if _, err := s.accounts.GetByID(ctx, userID, accountID); err != nil {
		return nil, err
	}

	holdings, err := s.holdings.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, apperror.Internal("failed to list salak holdings", err)
	}
	return holdings, nil
}
