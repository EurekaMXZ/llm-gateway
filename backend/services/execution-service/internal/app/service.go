package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"llm-gateway/backend/services/execution-service/internal/domain"
)

const openAICompatibleProtocol = "openai-compatible"

type Repository interface {
	CreateProvider(ctx context.Context, provider domain.Provider) error
	GetProviderByID(ctx context.Context, id string) (domain.Provider, error)
	ListProviders(ctx context.Context, ownerID string) ([]domain.Provider, error)
	UpdateProviderStatus(ctx context.Context, id string, status domain.ProviderStatus, updatedAt time.Time) (domain.Provider, error)

	CreateModel(ctx context.Context, model domain.Model) error
	GetModelByID(ctx context.Context, id string) (domain.Model, error)
	ListModels(ctx context.Context, providerID string, ownerID string) ([]domain.Model, error)
	UpdateModelStatus(ctx context.Context, id string, status domain.ModelStatus, updatedAt time.Time) (domain.Model, error)
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

type CreateProviderInput struct {
	ActorID          string
	ActorIsSuperuser bool
	ActorCanWrite    bool
	OwnerID          string
	Name             string
	Protocol         string
	BaseURL          string
	APIKey           string
}

func (s *Service) CreateProvider(ctx context.Context, in CreateProviderInput) (domain.Provider, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.OwnerID = strings.TrimSpace(in.OwnerID)
	in.Name = strings.TrimSpace(in.Name)
	in.Protocol = strings.TrimSpace(in.Protocol)
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.APIKey = strings.TrimSpace(in.APIKey)

	if in.ActorID == "" || in.OwnerID == "" || in.Name == "" || in.Protocol == "" || in.BaseURL == "" || in.APIKey == "" {
		return domain.Provider{}, domain.ErrInvalidInput
	}
	if in.Protocol != openAICompatibleProtocol {
		return domain.Provider{}, domain.ErrInvalidInput
	}
	if err := authorizeWrite(in.ActorID, in.OwnerID, in.ActorIsSuperuser, in.ActorCanWrite); err != nil {
		return domain.Provider{}, err
	}

	now := time.Now().UTC()
	provider := domain.Provider{
		ID:        randomID(),
		OwnerID:   in.OwnerID,
		Name:      in.Name,
		Protocol:  in.Protocol,
		BaseURL:   in.BaseURL,
		APIKey:    in.APIKey,
		Status:    domain.ProviderStatusEnabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateProvider(ctx, provider); err != nil {
		return domain.Provider{}, err
	}
	return sanitizeProvider(provider), nil
}

func (s *Service) GetProvider(ctx context.Context, id string) (domain.Provider, error) {
	ctx = ensureContext(ctx)
	provider, err := s.repo.GetProviderByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return domain.Provider{}, err
	}
	return sanitizeProvider(provider), nil
}

func (s *Service) ListProviders(ctx context.Context, ownerID string) ([]domain.Provider, error) {
	ctx = ensureContext(ctx)
	providers, err := s.repo.ListProviders(ctx, strings.TrimSpace(ownerID))
	if err != nil {
		return nil, err
	}
	for i := range providers {
		providers[i] = sanitizeProvider(providers[i])
	}
	return providers, nil
}

type SetProviderStatusInput struct {
	ActorID          string
	ActorIsSuperuser bool
	ActorCanWrite    bool
	ProviderID       string
	Status           domain.ProviderStatus
}

func (s *Service) SetProviderStatus(ctx context.Context, in SetProviderStatusInput) (domain.Provider, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.ProviderID = strings.TrimSpace(in.ProviderID)
	if in.ActorID == "" || in.ProviderID == "" {
		return domain.Provider{}, domain.ErrInvalidInput
	}
	if in.Status != domain.ProviderStatusEnabled && in.Status != domain.ProviderStatusDisabled {
		return domain.Provider{}, domain.ErrInvalidInput
	}

	provider, err := s.repo.GetProviderByID(ctx, in.ProviderID)
	if err != nil {
		return domain.Provider{}, err
	}
	if err := authorizeWrite(in.ActorID, provider.OwnerID, in.ActorIsSuperuser, in.ActorCanWrite); err != nil {
		return domain.Provider{}, err
	}

	updated, err := s.repo.UpdateProviderStatus(ctx, in.ProviderID, in.Status, time.Now().UTC())
	if err != nil {
		return domain.Provider{}, err
	}
	return sanitizeProvider(updated), nil
}

type CreateModelInput struct {
	ActorID          string
	ActorIsSuperuser bool
	ActorCanWrite    bool
	OwnerID          string
	ProviderID       string
	Name             string
	UpstreamModel    string
}

func (s *Service) CreateModel(ctx context.Context, in CreateModelInput) (domain.Model, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.OwnerID = strings.TrimSpace(in.OwnerID)
	in.ProviderID = strings.TrimSpace(in.ProviderID)
	in.Name = strings.TrimSpace(in.Name)
	in.UpstreamModel = strings.TrimSpace(in.UpstreamModel)

	if in.ActorID == "" || in.OwnerID == "" || in.ProviderID == "" || in.Name == "" || in.UpstreamModel == "" {
		return domain.Model{}, domain.ErrInvalidInput
	}
	if err := authorizeWrite(in.ActorID, in.OwnerID, in.ActorIsSuperuser, in.ActorCanWrite); err != nil {
		return domain.Model{}, err
	}

	provider, err := s.repo.GetProviderByID(ctx, in.ProviderID)
	if err != nil {
		return domain.Model{}, err
	}
	if provider.OwnerID != in.OwnerID {
		return domain.Model{}, domain.ErrForbidden
	}
	if provider.Status != domain.ProviderStatusEnabled {
		return domain.Model{}, domain.ErrProviderDisabled
	}

	now := time.Now().UTC()
	model := domain.Model{
		ID:            randomID(),
		ProviderID:    in.ProviderID,
		OwnerID:       in.OwnerID,
		Name:          in.Name,
		UpstreamModel: in.UpstreamModel,
		Status:        domain.ModelStatusEnabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreateModel(ctx, model); err != nil {
		return domain.Model{}, err
	}
	return model, nil
}

func (s *Service) GetModel(ctx context.Context, id string) (domain.Model, error) {
	ctx = ensureContext(ctx)
	return s.repo.GetModelByID(ctx, strings.TrimSpace(id))
}

func (s *Service) ListModels(ctx context.Context, providerID string, ownerID string) ([]domain.Model, error) {
	ctx = ensureContext(ctx)
	return s.repo.ListModels(ctx, strings.TrimSpace(providerID), strings.TrimSpace(ownerID))
}

type SetModelStatusInput struct {
	ActorID          string
	ActorIsSuperuser bool
	ActorCanWrite    bool
	ModelID          string
	Status           domain.ModelStatus
}

func (s *Service) SetModelStatus(ctx context.Context, in SetModelStatusInput) (domain.Model, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.ModelID = strings.TrimSpace(in.ModelID)
	if in.ActorID == "" || in.ModelID == "" {
		return domain.Model{}, domain.ErrInvalidInput
	}
	if in.Status != domain.ModelStatusEnabled && in.Status != domain.ModelStatusDisabled {
		return domain.Model{}, domain.ErrInvalidInput
	}

	model, err := s.repo.GetModelByID(ctx, in.ModelID)
	if err != nil {
		return domain.Model{}, err
	}
	if err := authorizeWrite(in.ActorID, model.OwnerID, in.ActorIsSuperuser, in.ActorCanWrite); err != nil {
		return domain.Model{}, err
	}
	return s.repo.UpdateModelStatus(ctx, in.ModelID, in.Status, time.Now().UTC())
}

func sanitizeProvider(in domain.Provider) domain.Provider {
	in.APIKey = ""
	return in
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
