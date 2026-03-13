package app

import (
	"context"
	"testing"

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
