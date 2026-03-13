package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"llm-gateway/backend/services/billing-service/internal/domain"
)

type Repository interface {
	UpsertPrice(ctx context.Context, price domain.Price) (domain.Price, error)
	GetPriceByID(ctx context.Context, id string) (domain.Price, error)
	ListPrices(ctx context.Context, ownerID string) ([]domain.Price, error)

	GetWallet(ctx context.Context, ownerID string) (domain.Wallet, error)
	ApplyWalletDelta(ctx context.Context, ownerID string, deltaCents int64, txID string, txType domain.TransactionType, reason string, now time.Time) (domain.Wallet, domain.WalletTransaction, error)
	ListTransactions(ctx context.Context, ownerID string, limit int) ([]domain.WalletTransaction, error)
}

type Service struct {
	repo Repository
}

func NewService() *Service {
	return NewServiceWithRepository(NewInMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	if repo == nil {
		repo = NewInMemoryRepository()
	}
	return &Service{repo: repo}
}

type SetPriceInput struct {
	ActorID          string
	ActorIsSuperuser bool
	ActorCanWrite    bool
	OwnerID          string
	ProviderID       string
	Model            string
	InputPricePer1K  float64
	OutputPricePer1K float64
	Currency         string
}

func (s *Service) SetPrice(ctx context.Context, in SetPriceInput) (domain.Price, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.OwnerID = strings.TrimSpace(in.OwnerID)
	in.ProviderID = strings.TrimSpace(in.ProviderID)
	in.Model = strings.TrimSpace(in.Model)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))

	if in.ActorID == "" || in.OwnerID == "" || in.ProviderID == "" || in.Model == "" || in.Currency == "" {
		return domain.Price{}, domain.ErrInvalidInput
	}
	if in.InputPricePer1K < 0 || in.OutputPricePer1K < 0 {
		return domain.Price{}, domain.ErrInvalidInput
	}
	if err := authorizeWrite(in.ActorID, in.OwnerID, in.ActorIsSuperuser, in.ActorCanWrite); err != nil {
		return domain.Price{}, err
	}

	now := time.Now().UTC()
	price := domain.Price{
		ID:               randomID(),
		OwnerID:          in.OwnerID,
		ProviderID:       in.ProviderID,
		Model:            in.Model,
		InputPricePer1K:  in.InputPricePer1K,
		OutputPricePer1K: in.OutputPricePer1K,
		Currency:         in.Currency,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	return s.repo.UpsertPrice(ctx, price)
}

func (s *Service) GetPrice(ctx context.Context, id string) (domain.Price, error) {
	ctx = ensureContext(ctx)
	return s.repo.GetPriceByID(ctx, strings.TrimSpace(id))
}

func (s *Service) ListPrices(ctx context.Context, ownerID string) ([]domain.Price, error) {
	ctx = ensureContext(ctx)
	return s.repo.ListPrices(ctx, strings.TrimSpace(ownerID))
}

type WalletOperationInput struct {
	ActorID          string
	ActorIsSuperuser bool
	ActorCanWrite    bool
	OwnerID          string
	AmountCents      int64
	Reason           string
}

func (s *Service) TopUp(ctx context.Context, in WalletOperationInput) (domain.Wallet, domain.WalletTransaction, error) {
	ctx = ensureContext(ctx)
	if err := validateWalletInput(in); err != nil {
		return domain.Wallet{}, domain.WalletTransaction{}, err
	}
	return s.repo.ApplyWalletDelta(ctx, strings.TrimSpace(in.OwnerID), in.AmountCents, randomID(), domain.TransactionTypeTopUp, strings.TrimSpace(in.Reason), time.Now().UTC())
}

func (s *Service) Deduct(ctx context.Context, in WalletOperationInput) (domain.Wallet, domain.WalletTransaction, error) {
	ctx = ensureContext(ctx)
	if err := validateWalletInput(in); err != nil {
		return domain.Wallet{}, domain.WalletTransaction{}, err
	}
	return s.repo.ApplyWalletDelta(ctx, strings.TrimSpace(in.OwnerID), -in.AmountCents, randomID(), domain.TransactionTypeDeduct, strings.TrimSpace(in.Reason), time.Now().UTC())
}

func (s *Service) GetWallet(ctx context.Context, ownerID string) (domain.Wallet, error) {
	ctx = ensureContext(ctx)
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return domain.Wallet{}, domain.ErrInvalidInput
	}
	wallet, err := s.repo.GetWallet(ctx, ownerID)
	if err != nil {
		if errors.Is(err, domain.ErrWalletNotFound) {
			return domain.Wallet{OwnerID: ownerID, BalanceCents: 0, UpdatedAt: time.Now().UTC()}, nil
		}
		return domain.Wallet{}, err
	}
	return wallet, nil
}

func (s *Service) ListTransactions(ctx context.Context, ownerID string, limit int) ([]domain.WalletTransaction, error) {
	ctx = ensureContext(ctx)
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, domain.ErrInvalidInput
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return s.repo.ListTransactions(ctx, ownerID, limit)
}

func validateWalletInput(in WalletOperationInput) error {
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.OwnerID = strings.TrimSpace(in.OwnerID)
	if in.ActorID == "" || in.OwnerID == "" || in.AmountCents <= 0 {
		return domain.ErrInvalidInput
	}
	if err := authorizeWrite(in.ActorID, in.OwnerID, in.ActorIsSuperuser, in.ActorCanWrite); err != nil {
		return err
	}
	return nil
}

func authorizeWrite(actorID string, ownerID string, actorIsSuperuser bool, actorCanWrite bool) error {
	if actorIsSuperuser || actorCanWrite {
		return nil
	}
	if actorID != ownerID {
		return domain.ErrForbidden
	}
	return nil
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}

func IsDomainError(err error, target error) bool {
	return errors.Is(err, target)
}
