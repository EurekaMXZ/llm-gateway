package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"llm-gateway/backend/services/routing-service/internal/domain"
)

type Repository interface {
	CreatePolicy(ctx context.Context, policy domain.Policy) error
	GetPolicyByID(ctx context.Context, id string) (domain.Policy, error)
	ListPolicies(ctx context.Context, ownerID string, customModel string) ([]domain.Policy, error)
	UpdatePolicyStatus(ctx context.Context, id string, status domain.PolicyStatus, updatedAt time.Time) (domain.Policy, error)
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

type CreatePolicyInput struct {
	ActorID          string
	ActorIsSuperuser bool
	ActorCanWrite    bool
	OwnerID          string
	CustomModel      string
	TargetProviderID string
	TargetModel      string
	Priority         int
	ConditionJSON    string
}

func (s *Service) CreatePolicy(ctx context.Context, in CreatePolicyInput) (domain.Policy, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.OwnerID = strings.TrimSpace(in.OwnerID)
	in.CustomModel = strings.TrimSpace(in.CustomModel)
	in.TargetProviderID = strings.TrimSpace(in.TargetProviderID)
	in.TargetModel = strings.TrimSpace(in.TargetModel)
	in.ConditionJSON = strings.TrimSpace(in.ConditionJSON)

	if in.ActorID == "" || in.OwnerID == "" || in.CustomModel == "" || in.TargetProviderID == "" || in.TargetModel == "" {
		return domain.Policy{}, domain.ErrInvalidInput
	}
	if in.Priority < 0 {
		return domain.Policy{}, domain.ErrInvalidInput
	}
	if err := authorizeWrite(in.ActorID, in.OwnerID, in.ActorIsSuperuser, in.ActorCanWrite); err != nil {
		return domain.Policy{}, err
	}

	now := time.Now().UTC()
	policy := domain.Policy{
		ID:               randomID(),
		OwnerID:          in.OwnerID,
		CustomModel:      in.CustomModel,
		TargetProviderID: in.TargetProviderID,
		TargetModel:      in.TargetModel,
		Priority:         in.Priority,
		ConditionJSON:    in.ConditionJSON,
		Status:           domain.PolicyStatusEnabled,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.CreatePolicy(ctx, policy); err != nil {
		return domain.Policy{}, err
	}
	return policy, nil
}

func (s *Service) GetPolicy(ctx context.Context, id string) (domain.Policy, error) {
	ctx = ensureContext(ctx)
	return s.repo.GetPolicyByID(ctx, strings.TrimSpace(id))
}

func (s *Service) ListPolicies(ctx context.Context, ownerID string, customModel string) ([]domain.Policy, error) {
	ctx = ensureContext(ctx)
	return s.repo.ListPolicies(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(customModel))
}

type SetPolicyStatusInput struct {
	ActorID          string
	ActorIsSuperuser bool
	ActorCanWrite    bool
	PolicyID         string
	Status           domain.PolicyStatus
}

func (s *Service) SetPolicyStatus(ctx context.Context, in SetPolicyStatusInput) (domain.Policy, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.PolicyID = strings.TrimSpace(in.PolicyID)
	if in.ActorID == "" || in.PolicyID == "" {
		return domain.Policy{}, domain.ErrInvalidInput
	}
	if in.Status != domain.PolicyStatusEnabled && in.Status != domain.PolicyStatusDisabled {
		return domain.Policy{}, domain.ErrInvalidInput
	}

	policy, err := s.repo.GetPolicyByID(ctx, in.PolicyID)
	if err != nil {
		return domain.Policy{}, err
	}
	if err := authorizeWrite(in.ActorID, policy.OwnerID, in.ActorIsSuperuser, in.ActorCanWrite); err != nil {
		return domain.Policy{}, err
	}

	return s.repo.UpdatePolicyStatus(ctx, in.PolicyID, in.Status, time.Now().UTC())
}

type ResolveInput struct {
	OwnerID     string
	CustomModel string
}

type ResolveResult struct {
	Matched bool          `json:"matched"`
	Policy  domain.Policy `json:"policy"`
	Reason  string        `json:"reason"`
}

func (s *Service) Resolve(ctx context.Context, in ResolveInput) (ResolveResult, error) {
	ctx = ensureContext(ctx)
	in.OwnerID = strings.TrimSpace(in.OwnerID)
	in.CustomModel = strings.TrimSpace(in.CustomModel)
	if in.OwnerID == "" || in.CustomModel == "" {
		return ResolveResult{}, domain.ErrInvalidInput
	}

	policies, err := s.repo.ListPolicies(ctx, in.OwnerID, in.CustomModel)
	if err != nil {
		return ResolveResult{}, err
	}
	for _, policy := range policies {
		if policy.Status != domain.PolicyStatusEnabled {
			continue
		}
		return ResolveResult{Matched: true, Policy: policy, Reason: "ok"}, nil
	}
	return ResolveResult{Matched: false, Reason: "no_policy_matched"}, nil
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
