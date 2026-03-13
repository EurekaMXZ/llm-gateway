package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"llm-gateway/backend/services/identity-service/internal/domain"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user domain.User) error
	GetUserByID(ctx context.Context, id string) (domain.User, error)
	GetUserByUsername(ctx context.Context, username string) (domain.User, error)
}

type Service struct {
	repo      UserRepository
	jwtSecret []byte
	tokenTTL  time.Duration
}

type Claims struct {
	UserID   string      `json:"uid"`
	Username string      `json:"username"`
	Role     domain.Role `json:"role"`
	jwt.RegisteredClaims
}

func NewService(jwtSecret string) *Service {
	return NewServiceWithRepository(jwtSecret, NewInMemoryUserRepository())
}

func NewServiceWithRepository(jwtSecret string, repo UserRepository) *Service {
	if strings.TrimSpace(jwtSecret) == "" {
		jwtSecret = "dev-only-secret"
	}
	if repo == nil {
		repo = NewInMemoryUserRepository()
	}
	return &Service{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
		tokenTTL:  time.Hour,
	}
}

func (s *Service) BootstrapSuperuser(ctx context.Context, username string, password string, displayName string) (domain.User, error) {
	ctx = ensureContext(ctx)
	username = strings.TrimSpace(username)
	if username == "" || strings.TrimSpace(password) == "" {
		return domain.User{}, domain.ErrInvalidCredentials
	}

	if _, err := s.repo.GetUserByUsername(ctx, username); err == nil {
		return domain.User{}, domain.ErrUsernameTaken
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return domain.User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}

	now := time.Now().UTC()
	u := domain.User{
		ID:           randomID(),
		Username:     username,
		DisplayName:  strings.TrimSpace(displayName),
		Role:         domain.RoleSuperuser,
		PasswordHash: string(hash),
		CreatedAt:    now,
	}
	if u.DisplayName == "" {
		u.DisplayName = "Superuser"
	}

	if err := s.repo.CreateUser(ctx, u); err != nil {
		return domain.User{}, err
	}
	return sanitizeUser(u), nil
}

type CreateUserInput struct {
	ActorID     string
	ActorRole   domain.Role
	Username    string
	Password    string
	DisplayName string
	Role        domain.Role
	ParentID    string
}

func (s *Service) CreateUser(ctx context.Context, in CreateUserInput) (domain.User, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.Username = strings.TrimSpace(in.Username)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.ParentID = strings.TrimSpace(in.ParentID)

	if in.ActorID == "" {
		return domain.User{}, domain.ErrForbidden
	}
	if in.ActorRole != domain.RoleSuperuser && in.ActorRole != domain.RoleAdmin {
		return domain.User{}, domain.ErrForbidden
	}
	if in.Username == "" || strings.TrimSpace(in.Password) == "" {
		return domain.User{}, domain.ErrInvalidCredentials
	}
	if in.Role != domain.RoleAdmin && in.Role != domain.RoleUser {
		return domain.User{}, domain.ErrInvalidRole
	}
	if in.ActorRole == domain.RoleAdmin && in.ParentID == "" {
		in.ParentID = in.ActorID
	}

	if _, err := s.repo.GetUserByUsername(ctx, in.Username); err == nil {
		return domain.User{}, domain.ErrUsernameTaken
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return domain.User{}, err
	}

	if in.ParentID != "" {
		parent, err := s.repo.GetUserByID(ctx, in.ParentID)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				return domain.User{}, domain.ErrUserNotFound
			}
			return domain.User{}, err
		}
		if in.ActorRole == domain.RoleAdmin && in.ParentID != in.ActorID {
			allowed, err := isDescendant(ctx, parent, in.ActorID, s.repo)
			if err != nil {
				return domain.User{}, err
			}
			if !allowed {
				return domain.User{}, domain.ErrForbidden
			}
		}
	} else if in.ActorRole == domain.RoleAdmin {
		return domain.User{}, domain.ErrForbidden
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}

	now := time.Now().UTC()
	u := domain.User{
		ID:           randomID(),
		Username:     in.Username,
		DisplayName:  in.DisplayName,
		Role:         in.Role,
		ParentID:     in.ParentID,
		PasswordHash: string(hash),
		CreatedAt:    now,
	}
	if u.DisplayName == "" {
		u.DisplayName = in.Username
	}

	if err := s.repo.CreateUser(ctx, u); err != nil {
		return domain.User{}, err
	}
	return sanitizeUser(u), nil
}

func (s *Service) Authenticate(ctx context.Context, username string, password string) (string, domain.User, time.Duration, error) {
	ctx = ensureContext(ctx)
	u, err := s.repo.GetUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return "", domain.User{}, 0, domain.ErrInvalidCredentials
		}
		return "", domain.User{}, 0, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", domain.User{}, 0, domain.ErrInvalidCredentials
	}

	now := time.Now().UTC()
	ttl := s.tokenTTL
	claims := Claims{
		UserID:   u.ID,
		Username: u.Username,
		Role:     u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   u.ID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", domain.User{}, 0, err
	}
	return signed, sanitizeUser(u), ttl, nil
}

func (s *Service) ValidateToken(ctx context.Context, tokenText string) (domain.User, error) {
	ctx = ensureContext(ctx)
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenText, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrInvalidToken
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return domain.User{}, domain.ErrInvalidToken
	}

	u, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, err
	}
	return sanitizeUser(u), nil
}

func (s *Service) CheckAccess(ctx context.Context, actorID string, resourceOwnerID string) (bool, string, error) {
	ctx = ensureContext(ctx)
	actor, err := s.repo.GetUserByID(ctx, strings.TrimSpace(actorID))
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return false, "actor_not_found", domain.ErrUserNotFound
		}
		return false, "actor_lookup_failed", err
	}

	owner, err := s.repo.GetUserByID(ctx, strings.TrimSpace(resourceOwnerID))
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return false, "resource_owner_not_found", domain.ErrUserNotFound
		}
		return false, "resource_owner_lookup_failed", err
	}

	switch actor.Role {
	case domain.RoleSuperuser:
		return true, "superuser_global_access", nil
	case domain.RoleAdmin:
		if actor.ID == owner.ID {
			return true, "self_access", nil
		}
		descendant, err := isDescendant(ctx, owner, actor.ID, s.repo)
		if err != nil {
			return false, "ancestor_check_failed", err
		}
		if descendant {
			return true, "subtree_access", nil
		}
		return false, "outside_subtree", nil
	case domain.RoleUser:
		if actor.ID == owner.ID {
			return true, "self_access", nil
		}
		return false, "peer_or_superior_forbidden", nil
	default:
		return false, "invalid_role", domain.ErrInvalidRole
	}
}

func (s *Service) GetUser(ctx context.Context, id string) (domain.User, error) {
	ctx = ensureContext(ctx)
	u, err := s.repo.GetUserByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return domain.User{}, err
	}
	return sanitizeUser(u), nil
}

func isDescendant(ctx context.Context, user domain.User, ancestorID string, repo UserRepository) (bool, error) {
	visited := map[string]struct{}{}
	current := user
	for strings.TrimSpace(current.ParentID) != "" {
		if current.ParentID == ancestorID {
			return true, nil
		}
		if _, seen := visited[current.ParentID]; seen {
			return false, nil
		}
		visited[current.ParentID] = struct{}{}

		next, err := repo.GetUserByID(ctx, current.ParentID)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				return false, nil
			}
			return false, err
		}
		current = next
	}
	return false, nil
}

func sanitizeUser(u domain.User) domain.User {
	u.PasswordHash = ""
	return u
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
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
