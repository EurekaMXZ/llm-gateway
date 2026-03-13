package app

import (
	"context"
	"sort"
	"sync"
	"time"

	"llm-gateway/backend/services/execution-service/internal/domain"
)

type InMemoryRepository struct {
	mu             sync.RWMutex
	providers      map[string]domain.Provider
	providerByName map[string]string
	models         map[string]domain.Model
	modelByName    map[string]string
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		providers:      map[string]domain.Provider{},
		providerByName: map[string]string{},
		models:         map[string]domain.Model{},
		modelByName:    map[string]string{},
	}
}

func (r *InMemoryRepository) CreateProvider(_ context.Context, provider domain.Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	nameKey := providerNameKey(provider.OwnerID, provider.Name)
	if _, exists := r.providerByName[nameKey]; exists {
		return domain.ErrProviderNameTaken
	}
	r.providers[provider.ID] = provider
	r.providerByName[nameKey] = provider.ID
	return nil
}

func (r *InMemoryRepository) GetProviderByID(_ context.Context, id string) (domain.Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[id]
	if !ok {
		return domain.Provider{}, domain.ErrProviderNotFound
	}
	return provider, nil
}

func (r *InMemoryRepository) ListProviders(_ context.Context, ownerID string) ([]domain.Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]domain.Provider, 0)
	for _, provider := range r.providers {
		if ownerID != "" && provider.OwnerID != ownerID {
			continue
		}
		list = append(list, provider)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
	return list, nil
}

func (r *InMemoryRepository) UpdateProviderStatus(_ context.Context, id string, status domain.ProviderStatus, updatedAt time.Time) (domain.Provider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	provider, ok := r.providers[id]
	if !ok {
		return domain.Provider{}, domain.ErrProviderNotFound
	}
	provider.Status = status
	provider.UpdatedAt = updatedAt.UTC()
	r.providers[id] = provider
	return provider, nil
}

func (r *InMemoryRepository) CreateModel(_ context.Context, model domain.Model) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	nameKey := modelNameKey(model.ProviderID, model.Name)
	if _, exists := r.modelByName[nameKey]; exists {
		return domain.ErrModelNameTaken
	}
	r.models[model.ID] = model
	r.modelByName[nameKey] = model.ID
	return nil
}

func (r *InMemoryRepository) GetModelByID(_ context.Context, id string) (domain.Model, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	model, ok := r.models[id]
	if !ok {
		return domain.Model{}, domain.ErrModelNotFound
	}
	return model, nil
}

func (r *InMemoryRepository) ListModels(_ context.Context, providerID string, ownerID string) ([]domain.Model, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]domain.Model, 0)
	for _, model := range r.models {
		if providerID != "" && model.ProviderID != providerID {
			continue
		}
		if ownerID != "" && model.OwnerID != ownerID {
			continue
		}
		list = append(list, model)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
	return list, nil
}

func (r *InMemoryRepository) UpdateModelStatus(_ context.Context, id string, status domain.ModelStatus, updatedAt time.Time) (domain.Model, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	model, ok := r.models[id]
	if !ok {
		return domain.Model{}, domain.ErrModelNotFound
	}
	model.Status = status
	model.UpdatedAt = updatedAt.UTC()
	r.models[id] = model
	return model, nil
}

func providerNameKey(ownerID string, name string) string {
	return ownerID + "::" + name
}

func modelNameKey(providerID string, name string) string {
	return providerID + "::" + name
}
