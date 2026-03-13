package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"llm-gateway/backend/services/apikey-service/internal/domain"
)

type KeyRepository interface {
	CreateKey(ctx context.Context, key domain.APIKey) error
	UpdateKeyStatus(ctx context.Context, id string, status domain.KeyStatus, updatedAt time.Time) (domain.APIKey, error)
	GetKeyByID(ctx context.Context, id string) (domain.APIKey, error)
	GetKeyBySecretHash(ctx context.Context, secretHash string) (domain.APIKey, error)
}

type Service struct {
	repo KeyRepository
}

type CreateKeyInput struct {
	OwnerID       string
	Name          string
	AllowedModels []string
	ExpiresAt     *time.Time
}

type ValidateResult struct {
	Valid   bool   `json:"valid"`
	KeyID   string `json:"key_id,omitempty"`
	OwnerID string `json:"owner_id,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Status  string `json:"status,omitempty"`
}

func NewService() *Service {
	return NewServiceWithRepository(NewInMemoryKeyRepository())
}

func NewServiceWithRepository(repo KeyRepository) *Service {
	if repo == nil {
		repo = NewInMemoryKeyRepository()
	}
	return &Service{repo: repo}
}

func (s *Service) CreateKey(ctx context.Context, in CreateKeyInput) (domain.APIKey, string, error) {
	ctx = ensureContext(ctx)
	ownerID := strings.TrimSpace(in.OwnerID)
	name := strings.TrimSpace(in.Name)
	if ownerID == "" || name == "" {
		return domain.APIKey{}, "", domain.ErrInvalidInput
	}

	models := normalizeModels(in.AllowedModels)
	now := time.Now().UTC()
	if in.ExpiresAt != nil && in.ExpiresAt.Before(now) {
		return domain.APIKey{}, "", domain.ErrInvalidInput
	}

	plainKey := generatePlainKey()
	hash := hashSecret(plainKey)

	record := domain.APIKey{
		ID:            randomID(),
		OwnerID:       ownerID,
		Name:          name,
		SecretHash:    hash,
		AllowedModels: models,
		Status:        domain.KeyStatusEnabled,
		ExpiresAt:     in.ExpiresAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repo.CreateKey(ctx, record); err != nil {
		return domain.APIKey{}, "", err
	}
	return sanitize(record), plainKey, nil
}

func (s *Service) DisableKey(ctx context.Context, id string) (domain.APIKey, error) {
	return s.changeStatus(ctx, id, domain.KeyStatusDisabled)
}

func (s *Service) EnableKey(ctx context.Context, id string) (domain.APIKey, error) {
	return s.changeStatus(ctx, id, domain.KeyStatusEnabled)
}

func (s *Service) changeStatus(ctx context.Context, id string, status domain.KeyStatus) (domain.APIKey, error) {
	ctx = ensureContext(ctx)
	record, err := s.repo.UpdateKeyStatus(ctx, strings.TrimSpace(id), status, time.Now().UTC())
	if err != nil {
		return domain.APIKey{}, err
	}
	return sanitize(record), nil
}

func (s *Service) GetKey(ctx context.Context, id string) (domain.APIKey, error) {
	ctx = ensureContext(ctx)
	record, err := s.repo.GetKeyByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return domain.APIKey{}, err
	}
	return sanitize(record), nil
}

func (s *Service) Validate(ctx context.Context, secret string, model string, now time.Time) (ValidateResult, error) {
	ctx = ensureContext(ctx)
	hash := hashSecret(secret)
	model = strings.TrimSpace(model)

	record, err := s.repo.GetKeyBySecretHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrKeyNotFound) {
			return ValidateResult{Valid: false, Reason: "key_not_found"}, nil
		}
		return ValidateResult{}, err
	}

	if record.Status == domain.KeyStatusDisabled {
		return ValidateResult{Valid: false, KeyID: record.ID, OwnerID: record.OwnerID, Status: string(record.Status), Reason: "key_disabled"}, nil
	}
	if record.ExpiresAt != nil && now.UTC().After(record.ExpiresAt.UTC()) {
		return ValidateResult{Valid: false, KeyID: record.ID, OwnerID: record.OwnerID, Status: string(record.Status), Reason: "key_expired"}, nil
	}
	if len(record.AllowedModels) > 0 {
		if model == "" {
			return ValidateResult{Valid: false, KeyID: record.ID, OwnerID: record.OwnerID, Status: string(record.Status), Reason: "model_required"}, nil
		}
		if !contains(record.AllowedModels, model) {
			return ValidateResult{Valid: false, KeyID: record.ID, OwnerID: record.OwnerID, Status: string(record.Status), Reason: "model_forbidden"}, nil
		}
	}

	return ValidateResult{Valid: true, KeyID: record.ID, OwnerID: record.OwnerID, Status: string(record.Status), Reason: "ok"}, nil
}

func sanitize(in domain.APIKey) domain.APIKey {
	in.SecretHash = ""
	models := append([]string(nil), in.AllowedModels...)
	in.AllowedModels = models
	return in
}

func normalizeModels(models []string) []string {
	set := map[string]struct{}{}
	for _, m := range models {
		n := strings.TrimSpace(m)
		if n == "" {
			continue
		}
		set[n] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}

func generatePlainKey() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "kgw_fallback"
	}
	return "kgw_" + hex.EncodeToString(b)
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func IsDomainError(err error, target error) bool {
	return errors.Is(err, target)
}
