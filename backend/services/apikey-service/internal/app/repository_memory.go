package app

import (
	"context"
	"sync"
	"time"

	"llm-gateway/backend/services/apikey-service/internal/domain"
)

type InMemoryKeyRepository struct {
	mu        sync.RWMutex
	keys      map[string]domain.APIKey
	keyByHash map[string]string
}

func NewInMemoryKeyRepository() *InMemoryKeyRepository {
	return &InMemoryKeyRepository{
		keys:      map[string]domain.APIKey{},
		keyByHash: map[string]string{},
	}
}

func (r *InMemoryKeyRepository) CreateKey(_ context.Context, key domain.APIKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys[key.ID] = copyKey(key)
	r.keyByHash[key.SecretHash] = key.ID
	return nil
}

func (r *InMemoryKeyRepository) UpdateKeyStatus(_ context.Context, id string, status domain.KeyStatus, updatedAt time.Time) (domain.APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.keys[id]
	if !ok {
		return domain.APIKey{}, domain.ErrKeyNotFound
	}
	record.Status = status
	record.UpdatedAt = updatedAt.UTC()
	r.keys[id] = record
	return copyKey(record), nil
}

func (r *InMemoryKeyRepository) GetKeyByID(_ context.Context, id string) (domain.APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.keys[id]
	if !ok {
		return domain.APIKey{}, domain.ErrKeyNotFound
	}
	return copyKey(record), nil
}

func (r *InMemoryKeyRepository) GetKeyBySecretHash(_ context.Context, secretHash string) (domain.APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.keyByHash[secretHash]
	if !ok {
		return domain.APIKey{}, domain.ErrKeyNotFound
	}
	record, ok := r.keys[id]
	if !ok {
		return domain.APIKey{}, domain.ErrKeyNotFound
	}
	return copyKey(record), nil
}

func copyKey(in domain.APIKey) domain.APIKey {
	out := in
	out.AllowedModels = append([]string(nil), in.AllowedModels...)
	if in.ExpiresAt != nil {
		exp := in.ExpiresAt.UTC()
		out.ExpiresAt = &exp
	}
	return out
}
