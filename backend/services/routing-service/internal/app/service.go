package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	if _, _, err := parseDifficultyCondition(in.ConditionJSON); err != nil {
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
	OwnerID         string
	Model           string
	CustomModel     string
	DifficultyScore int
}

type ResolveResult struct {
	Matched bool          `json:"matched"`
	Policy  domain.Policy `json:"policy"`
	Reason  string        `json:"reason"`
}

func (s *Service) Resolve(ctx context.Context, in ResolveInput) (ResolveResult, error) {
	ctx = ensureContext(ctx)
	in.OwnerID = strings.TrimSpace(in.OwnerID)
	in.Model = strings.TrimSpace(in.Model)
	in.CustomModel = strings.TrimSpace(in.CustomModel)
	if in.Model == "" {
		in.Model = in.CustomModel
	}
	if in.OwnerID == "" || in.Model == "" {
		return ResolveResult{}, domain.ErrInvalidInput
	}

	policies, err := s.repo.ListPolicies(ctx, in.OwnerID, in.Model)
	if err != nil {
		return ResolveResult{}, err
	}
	for _, policy := range policies {
		if policy.Status != domain.PolicyStatusEnabled {
			continue
		}
		matched, err := matchesCondition(policy.ConditionJSON, in.DifficultyScore)
		if err != nil {
			// Backward compatibility:
			// legacy rows may contain condition strings that are not parseable by
			// current evaluator. Treat them as non-matching so resolve can continue.
			continue
		}
		if !matched {
			continue
		}
		return ResolveResult{Matched: true, Policy: policy, Reason: "ok"}, nil
	}
	return ResolveResult{Matched: false, Reason: "no_policy_matched"}, nil
}

type difficultyCondition struct {
	Type string `json:"type"`
	Min  *int   `json:"min"`
	Max  *int   `json:"max"`
}

func matchesCondition(raw string, difficultyScore int) (bool, error) {
	cond, hasCondition, err := parseDifficultyCondition(raw)
	if err != nil {
		return false, err
	}
	if !hasCondition {
		return true, nil
	}
	min := 0
	max := 100
	if cond.Min != nil {
		min = *cond.Min
	}
	if cond.Max != nil {
		max = *cond.Max
	}
	return difficultyScore >= min && difficultyScore <= max, nil
}

func parseDifficultyCondition(raw string) (difficultyCondition, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return difficultyCondition{}, false, nil
	}

	var cond difficultyCondition
	if err := json.Unmarshal([]byte(raw), &cond); err != nil {
		return difficultyCondition{}, false, fmt.Errorf("invalid condition json: %w", err)
	}

	if cond.Type == "" && cond.Min == nil && cond.Max == nil {
		return difficultyCondition{}, false, nil
	}
	if cond.Type == "" {
		cond.Type = "difficulty"
	}
	if cond.Type != "difficulty" {
		return difficultyCondition{}, false, errors.New("unsupported condition type")
	}

	min := 0
	max := 100
	if cond.Min != nil {
		min = *cond.Min
	}
	if cond.Max != nil {
		max = *cond.Max
	}
	if min < 0 || min > 100 || max < 0 || max > 100 || min > max {
		return difficultyCondition{}, false, errors.New("invalid difficulty range")
	}
	return cond, true, nil
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
