package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-gateway/backend/services/execution-service/internal/domain"
)

const (
	openAICompatibleProtocol = "openai-compatible"
	defaultProviderPriority  = 100
	defaultUpstreamTimeout   = 30 * time.Second
)

type ExecutionTarget struct {
	Provider domain.Provider
	Model    domain.Model
}

type Repository interface {
	CreateProvider(ctx context.Context, provider domain.Provider) error
	GetProviderByID(ctx context.Context, id string) (domain.Provider, error)
	ListProviders(ctx context.Context, ownerID string) ([]domain.Provider, error)
	UpdateProviderStatus(ctx context.Context, id string, status domain.ProviderStatus, updatedAt time.Time) (domain.Provider, error)
	UpdateProviderPriority(ctx context.Context, id string, priority int, updatedAt time.Time) (domain.Provider, error)

	CreateModel(ctx context.Context, model domain.Model) error
	GetModelByID(ctx context.Context, id string) (domain.Model, error)
	ListModels(ctx context.Context, providerID string, ownerID string) ([]domain.Model, error)
	UpdateModelStatus(ctx context.Context, id string, status domain.ModelStatus, updatedAt time.Time) (domain.Model, error)

	ResolveExecutionTarget(ctx context.Context, ownerID string, modelName string, providerID string) (ExecutionTarget, error)
}

type Service struct {
	repo       Repository
	httpClient *http.Client
}

func NewService() *Service {
	return NewServiceWithRepositoryAndClient(NewInMemoryRepository(), nil)
}

func NewServiceWithRepository(repo Repository) *Service {
	return NewServiceWithRepositoryAndClient(repo, nil)
}

func NewServiceWithRepositoryAndClient(repo Repository, client *http.Client) *Service {
	if repo == nil {
		repo = NewInMemoryRepository()
	}
	if client == nil {
		client = &http.Client{Timeout: defaultUpstreamTimeout}
	}
	return &Service{repo: repo, httpClient: client}
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
	Priority         *int
}

func (s *Service) CreateProvider(ctx context.Context, in CreateProviderInput) (domain.Provider, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.OwnerID = strings.TrimSpace(in.OwnerID)
	in.Name = strings.TrimSpace(in.Name)
	in.Protocol = strings.TrimSpace(in.Protocol)
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.APIKey = strings.TrimSpace(in.APIKey)

	priority := defaultProviderPriority
	if in.Priority != nil {
		priority = *in.Priority
	}

	if in.ActorID == "" || in.OwnerID == "" || in.Name == "" || in.Protocol == "" || in.BaseURL == "" || in.APIKey == "" {
		return domain.Provider{}, domain.ErrInvalidInput
	}
	if priority < 0 {
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
		Priority:  priority,
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

type SetProviderPriorityInput struct {
	ActorID          string
	ActorIsSuperuser bool
	ActorCanWrite    bool
	ProviderID       string
	Priority         int
}

func (s *Service) SetProviderPriority(ctx context.Context, in SetProviderPriorityInput) (domain.Provider, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.ProviderID = strings.TrimSpace(in.ProviderID)
	if in.ActorID == "" || in.ProviderID == "" || in.Priority < 0 {
		return domain.Provider{}, domain.ErrInvalidInput
	}

	provider, err := s.repo.GetProviderByID(ctx, in.ProviderID)
	if err != nil {
		return domain.Provider{}, err
	}
	if err := authorizeWrite(in.ActorID, provider.OwnerID, in.ActorIsSuperuser, in.ActorCanWrite); err != nil {
		return domain.Provider{}, err
	}

	updated, err := s.repo.UpdateProviderPriority(ctx, in.ProviderID, in.Priority, time.Now().UTC())
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

type ExecuteChatInput struct {
	OwnerID    string
	ProviderID string
	Payload    map[string]any
}

type ExecuteChatResult struct {
	ProviderID    string         `json:"provider_id"`
	UpstreamModel string         `json:"upstream_model"`
	Response      map[string]any `json:"response"`
}

func (s *Service) ExecuteChat(ctx context.Context, in ExecuteChatInput) (ExecuteChatResult, error) {
	ctx = ensureContext(ctx)
	in.OwnerID = strings.TrimSpace(in.OwnerID)
	in.ProviderID = strings.TrimSpace(in.ProviderID)
	if in.OwnerID == "" || in.Payload == nil {
		return ExecuteChatResult{}, domain.ErrInvalidInput
	}

	modelName, err := payloadModelName(in.Payload)
	if err != nil {
		return ExecuteChatResult{}, err
	}
	if stream, _ := in.Payload["stream"].(bool); stream {
		return ExecuteChatResult{}, domain.ErrInvalidInput
	}

	target, err := s.repo.ResolveExecutionTarget(ctx, in.OwnerID, modelName, in.ProviderID)
	if err != nil {
		return ExecuteChatResult{}, err
	}
	if target.Provider.Protocol != openAICompatibleProtocol {
		return ExecuteChatResult{}, domain.ErrInvalidInput
	}

	upstreamPayload, err := clonePayload(in.Payload)
	if err != nil {
		return ExecuteChatResult{}, domain.ErrInvalidInput
	}
	upstreamPayload["model"] = target.Model.UpstreamModel

	respPayload, err := s.invokeUpstreamChat(ctx, target.Provider, upstreamPayload)
	if err != nil {
		return ExecuteChatResult{}, err
	}
	return ExecuteChatResult{
		ProviderID:    target.Provider.ID,
		UpstreamModel: target.Model.UpstreamModel,
		Response:      respPayload,
	}, nil
}

func (s *Service) invokeUpstreamChat(ctx context.Context, provider domain.Provider, payload map[string]any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, domain.ErrInvalidInput
	}

	url := strings.TrimRight(provider.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: create request failed", domain.ErrUpstreamFailed)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed: %v", domain.ErrUpstreamFailed, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read response failed", domain.ErrUpstreamFailed)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status=%d body=%s", domain.ErrUpstreamFailed, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("%w: decode response failed", domain.ErrUpstreamFailed)
	}
	return parsed, nil
}

func payloadModelName(payload map[string]any) (string, error) {
	raw, ok := payload["model"]
	if !ok {
		return "", domain.ErrInvalidInput
	}
	model, ok := raw.(string)
	if !ok {
		return "", domain.ErrInvalidInput
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return "", domain.ErrInvalidInput
	}
	return model, nil
}

func clonePayload(payload map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
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
