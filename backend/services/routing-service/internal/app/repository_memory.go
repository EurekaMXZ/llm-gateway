package app

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"llm-gateway/backend/services/routing-service/internal/domain"
)

type InMemoryRepository struct {
	mu          sync.RWMutex
	policies    map[string]domain.Policy
	policyByKey map[string]string
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		policies:    map[string]domain.Policy{},
		policyByKey: map[string]string{},
	}
}

func (r *InMemoryRepository) CreatePolicy(_ context.Context, policy domain.Policy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := policyKey(policy.OwnerID, policy.CustomModel, policy.Priority)
	if _, exists := r.policyByKey[key]; exists {
		return domain.ErrPolicyNameTaken
	}
	r.policies[policy.ID] = policy
	r.policyByKey[key] = policy.ID
	return nil
}

func (r *InMemoryRepository) GetPolicyByID(_ context.Context, id string) (domain.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	policy, ok := r.policies[id]
	if !ok {
		return domain.Policy{}, domain.ErrPolicyNotFound
	}
	return policy, nil
}

func (r *InMemoryRepository) ListPolicies(_ context.Context, ownerID string, customModel string) ([]domain.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]domain.Policy, 0)
	for _, policy := range r.policies {
		if ownerID != "" && policy.OwnerID != ownerID {
			continue
		}
		if customModel != "" && policy.CustomModel != customModel {
			continue
		}
		list = append(list, policy)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Priority == list[j].Priority {
			return list[i].CreatedAt.Before(list[j].CreatedAt)
		}
		return list[i].Priority < list[j].Priority
	})
	return list, nil
}

func (r *InMemoryRepository) UpdatePolicyStatus(_ context.Context, id string, status domain.PolicyStatus, updatedAt time.Time) (domain.Policy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	policy, ok := r.policies[id]
	if !ok {
		return domain.Policy{}, domain.ErrPolicyNotFound
	}
	policy.Status = status
	policy.UpdatedAt = updatedAt.UTC()
	r.policies[id] = policy
	return policy, nil
}

func policyKey(ownerID string, customModel string, priority int) string {
	return ownerID + "::" + customModel + "::" + fmt.Sprintf("%d", priority)
}
