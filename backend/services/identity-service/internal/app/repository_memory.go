package app

import (
	"context"
	"sync"

	"llm-gateway/backend/services/identity-service/internal/domain"
)

type InMemoryUserRepository struct {
	mu         sync.RWMutex
	users      map[string]domain.User
	userByName map[string]string
}

func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		users:      map[string]domain.User{},
		userByName: map[string]string{},
	}
}

func (r *InMemoryUserRepository) CreateUser(_ context.Context, user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.userByName[user.Username]; exists {
		return domain.ErrUsernameTaken
	}
	if user.Role == domain.RoleSuperuser {
		for _, existing := range r.users {
			if existing.Role == domain.RoleSuperuser {
				return domain.ErrSuperuserAlreadyExists
			}
		}
	}
	r.users[user.ID] = user
	r.userByName[user.Username] = user.ID
	return nil
}

func (r *InMemoryUserRepository) GetUserByID(_ context.Context, id string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *InMemoryUserRepository) GetUserByUsername(_ context.Context, username string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.userByName[username]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *InMemoryUserRepository) SuperuserExists(_ context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if u.Role == domain.RoleSuperuser {
			return true, nil
		}
	}
	return false, nil
}
