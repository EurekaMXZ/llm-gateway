package app

import (
	"context"
	"testing"
	"time"

	"llm-gateway/backend/services/routing-service/internal/domain"
)

func TestPolicyOwnershipConstraint(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	_, err := svc.CreatePolicy(ctx, CreatePolicyInput{
		ActorID:          "u-a",
		ActorIsSuperuser: false,
		OwnerID:          "u-b",
		CustomModel:      "common_chat",
		TargetProviderID: "provider-1",
		TargetModel:      "gpt-4o-mini",
		Priority:         10,
	})
	if !IsDomainError(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	_, err = svc.CreatePolicy(ctx, CreatePolicyInput{
		ActorID:          "u-b",
		ActorIsSuperuser: false,
		OwnerID:          "u-b",
		CustomModel:      "common_chat",
		TargetProviderID: "provider-1",
		TargetModel:      "gpt-4o-mini",
		Priority:         10,
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
}

func TestResolveByPriorityAndStatus(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	pLow, err := svc.CreatePolicy(ctx, CreatePolicyInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		CustomModel:      "common_chat",
		TargetProviderID: "provider-a",
		TargetModel:      "gpt-4.1-mini",
		Priority:         20,
	})
	if err != nil {
		t.Fatalf("create policy pLow: %v", err)
	}

	pHigh, err := svc.CreatePolicy(ctx, CreatePolicyInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		CustomModel:      "common_chat",
		TargetProviderID: "provider-b",
		TargetModel:      "gpt-4o-mini",
		Priority:         10,
	})
	if err != nil {
		t.Fatalf("create policy pHigh: %v", err)
	}

	result, err := svc.Resolve(ctx, ResolveInput{OwnerID: "u-1", CustomModel: "common_chat"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !result.Matched || result.Policy.Priority != 10 {
		t.Fatalf("expected priority 10 policy, got %+v", result)
	}

	if _, err := svc.SetPolicyStatus(ctx, SetPolicyStatusInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		PolicyID:         pHigh.ID,
		Status:           domain.PolicyStatusDisabled,
	}); err != nil {
		t.Fatalf("disable pHigh: %v", err)
	}

	result, err = svc.Resolve(ctx, ResolveInput{OwnerID: "u-1", CustomModel: "common_chat"})
	if err != nil {
		t.Fatalf("resolve after disable: %v", err)
	}
	if !result.Matched || result.Policy.ID != pLow.ID {
		t.Fatalf("expected fallback to pLow, got %+v", result)
	}
}

func TestActorCanWriteAllowsSubtreePolicyMutation(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	policy, err := svc.CreatePolicy(ctx, CreatePolicyInput{
		ActorID:          "owner-1",
		ActorIsSuperuser: false,
		OwnerID:          "owner-1",
		CustomModel:      "common_chat",
		TargetProviderID: "provider-1",
		TargetModel:      "gpt-4o-mini",
		Priority:         1,
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	if _, err := svc.SetPolicyStatus(ctx, SetPolicyStatusInput{
		ActorID:          "admin-a",
		ActorIsSuperuser: false,
		ActorCanWrite:    true,
		PolicyID:         policy.ID,
		Status:           domain.PolicyStatusDisabled,
	}); err != nil {
		t.Fatalf("expected actor_can_write status update, got %v", err)
	}
}

func TestResolveWithDifficultyCondition(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	if _, err := svc.CreatePolicy(ctx, CreatePolicyInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		CustomModel:      "gpt-5.3-codex",
		TargetProviderID: "provider-hard",
		TargetModel:      "gpt-5.4",
		Priority:         10,
		ConditionJSON:    `{"type":"difficulty","min":70,"max":100}`,
	}); err != nil {
		t.Fatalf("create hard policy: %v", err)
	}
	if _, err := svc.CreatePolicy(ctx, CreatePolicyInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		CustomModel:      "gpt-5.3-codex",
		TargetProviderID: "provider-easy",
		TargetModel:      "qwen3.5-plus",
		Priority:         20,
		ConditionJSON:    `{"type":"difficulty","min":0,"max":69}`,
	}); err != nil {
		t.Fatalf("create easy policy: %v", err)
	}

	high, err := svc.Resolve(ctx, ResolveInput{
		OwnerID:         "u-1",
		Model:           "gpt-5.3-codex",
		DifficultyScore: 80,
	})
	if err != nil {
		t.Fatalf("resolve high score: %v", err)
	}
	if !high.Matched || high.Policy.TargetProviderID != "provider-hard" {
		t.Fatalf("expected hard route, got %+v", high)
	}

	low, err := svc.Resolve(ctx, ResolveInput{
		OwnerID:         "u-1",
		CustomModel:     "gpt-5.3-codex",
		DifficultyScore: 20,
	})
	if err != nil {
		t.Fatalf("resolve low score: %v", err)
	}
	if !low.Matched || low.Policy.TargetProviderID != "provider-easy" {
		t.Fatalf("expected easy route, got %+v", low)
	}

	miss, err := svc.Resolve(ctx, ResolveInput{
		OwnerID:         "u-1",
		Model:           "gpt-4.4-pro",
		DifficultyScore: 50,
	})
	if err != nil {
		t.Fatalf("resolve miss: %v", err)
	}
	if miss.Matched || miss.Reason != "no_policy_matched" {
		t.Fatalf("expected no match, got %+v", miss)
	}
}

func TestCreatePolicyRejectsInvalidCondition(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	_, err := svc.CreatePolicy(ctx, CreatePolicyInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		CustomModel:      "common_chat",
		TargetProviderID: "provider-1",
		TargetModel:      "gpt-4o-mini",
		Priority:         10,
		ConditionJSON:    `{"type":"difficulty","min":95,"max":10}`,
	})
	if !IsDomainError(err, domain.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestResolveSkipsLegacyInvalidCondition(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	now := time.Now().UTC()
	legacy := domain.Policy{
		ID:               "legacy-invalid",
		OwnerID:          "u-1",
		CustomModel:      "common_chat",
		TargetProviderID: "provider-legacy",
		TargetModel:      "gpt-legacy",
		Priority:         1,
		ConditionJSON:    "legacy-condition-format",
		Status:           domain.PolicyStatusEnabled,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := svc.repo.CreatePolicy(ctx, legacy); err != nil {
		t.Fatalf("create legacy policy: %v", err)
	}

	valid, err := svc.CreatePolicy(ctx, CreatePolicyInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		CustomModel:      "common_chat",
		TargetProviderID: "provider-new",
		TargetModel:      "gpt-4o-mini",
		Priority:         10,
		ConditionJSON:    `{"type":"difficulty","min":0,"max":100}`,
	})
	if err != nil {
		t.Fatalf("create valid policy: %v", err)
	}

	result, err := svc.Resolve(ctx, ResolveInput{
		OwnerID:         "u-1",
		Model:           "common_chat",
		DifficultyScore: 50,
	})
	if err != nil {
		t.Fatalf("resolve should not fail on legacy invalid condition: %v", err)
	}
	if !result.Matched || result.Policy.ID != valid.ID {
		t.Fatalf("expected valid policy match, got %+v", result)
	}
}

func TestResolveOnlyLegacyInvalidConditionReturnsNoMatch(t *testing.T) {
	svc := NewService()
	ctx := context.Background()
	now := time.Now().UTC()

	legacy := domain.Policy{
		ID:               "legacy-invalid-only",
		OwnerID:          "u-1",
		CustomModel:      "common_chat",
		TargetProviderID: "provider-legacy",
		TargetModel:      "gpt-legacy",
		Priority:         1,
		ConditionJSON:    "{not-json",
		Status:           domain.PolicyStatusEnabled,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := svc.repo.CreatePolicy(ctx, legacy); err != nil {
		t.Fatalf("create legacy policy: %v", err)
	}

	result, err := svc.Resolve(ctx, ResolveInput{
		OwnerID:         "u-1",
		Model:           "common_chat",
		DifficultyScore: 50,
	})
	if err != nil {
		t.Fatalf("resolve should not fail: %v", err)
	}
	if result.Matched || result.Reason != "no_policy_matched" {
		t.Fatalf("expected no match, got %+v", result)
	}
}
