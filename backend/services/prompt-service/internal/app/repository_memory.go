package app

import (
	"context"
	"sort"
	"sync"
	"time"

	"llm-gateway/backend/services/prompt-service/internal/domain"
)

type InMemoryRepository struct {
	mu           sync.RWMutex
	templates    map[string]domain.SceneTemplate
	templateKeys map[string]string
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		templates:    map[string]domain.SceneTemplate{},
		templateKeys: map[string]string{},
	}
}

func (r *InMemoryRepository) CreateTemplate(_ context.Context, tpl domain.SceneTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := templateKey(tpl.OwnerID, tpl.Scene)
	if _, exists := r.templateKeys[k]; exists {
		return domain.ErrTemplateNameTaken
	}
	r.templates[tpl.ID] = copyTemplate(tpl)
	r.templateKeys[k] = tpl.ID
	return nil
}

func (r *InMemoryRepository) GetTemplateByID(_ context.Context, id string) (domain.SceneTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tpl, ok := r.templates[id]
	if !ok {
		return domain.SceneTemplate{}, domain.ErrTemplateNotFound
	}
	return copyTemplate(tpl), nil
}

func (r *InMemoryRepository) GetTemplateByOwnerScene(_ context.Context, ownerID string, scene string) (domain.SceneTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.templateKeys[templateKey(ownerID, scene)]
	if !ok {
		return domain.SceneTemplate{}, domain.ErrTemplateNotFound
	}
	tpl, ok := r.templates[id]
	if !ok {
		return domain.SceneTemplate{}, domain.ErrTemplateNotFound
	}
	return copyTemplate(tpl), nil
}

func (r *InMemoryRepository) ListTemplates(_ context.Context, ownerID string) ([]domain.SceneTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]domain.SceneTemplate, 0)
	for _, tpl := range r.templates {
		if ownerID != "" && tpl.OwnerID != ownerID {
			continue
		}
		list = append(list, copyTemplate(tpl))
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list, nil
}

func (r *InMemoryRepository) UpdateTemplateStatus(_ context.Context, id string, status domain.TemplateStatus, updatedAt time.Time) (domain.SceneTemplate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tpl, ok := r.templates[id]
	if !ok {
		return domain.SceneTemplate{}, domain.ErrTemplateNotFound
	}
	tpl.Status = status
	tpl.UpdatedAt = updatedAt.UTC()
	r.templates[id] = tpl
	return copyTemplate(tpl), nil
}

func templateKey(ownerID string, scene string) string {
	return ownerID + "::" + scene
}

func copyTemplate(tpl domain.SceneTemplate) domain.SceneTemplate {
	out := tpl
	out.Variables = copyVariables(tpl.Variables)
	return out
}
