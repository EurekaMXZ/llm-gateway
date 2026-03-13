package app

import (
	"context"
	"testing"

	"llm-gateway/backend/services/execution-service/internal/domain"
)

func TestProviderOwnershipConstraint(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	_, err := svc.CreateProvider(ctx, CreateProviderInput{
		ActorID:          "u-actor",
		ActorIsSuperuser: false,
		OwnerID:          "u-owner",
		Name:             "openai-main",
		Protocol:         "openai-compatible",
		BaseURL:          "https://api.openai.com/v1",
		APIKey:           "sk-xxx",
	})
	if !IsDomainError(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	provider, err := svc.CreateProvider(ctx, CreateProviderInput{
		ActorID:          "u-owner",
		ActorIsSuperuser: false,
		OwnerID:          "u-owner",
		Name:             "openai-main",
		Protocol:         "openai-compatible",
		BaseURL:          "https://api.openai.com/v1",
		APIKey:           "sk-xxx",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if provider.APIKey != "" {
		t.Fatalf("api key should be sanitized")
	}
}

func TestModelCreationRequiresProviderOwnershipAndStatus(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	provider, err := svc.CreateProvider(ctx, CreateProviderInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		Name:             "provider-a",
		Protocol:         "openai-compatible",
		BaseURL:          "https://example.com/v1",
		APIKey:           "sk-xxx",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	_, err = svc.CreateModel(ctx, CreateModelInput{
		ActorID:          "u-2",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		ProviderID:       provider.ID,
		Name:             "gpt-4o-mini",
		UpstreamModel:    "gpt-4o-mini",
	})
	if !IsDomainError(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden model creation, got %v", err)
	}

	if _, err := svc.SetProviderStatus(ctx, SetProviderStatusInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		ProviderID:       provider.ID,
		Status:           domain.ProviderStatusDisabled,
	}); err != nil {
		t.Fatalf("disable provider: %v", err)
	}

	_, err = svc.CreateModel(ctx, CreateModelInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		ProviderID:       provider.ID,
		Name:             "gpt-4o-mini",
		UpstreamModel:    "gpt-4o-mini",
	})
	if !IsDomainError(err, domain.ErrProviderDisabled) {
		t.Fatalf("expected provider disabled, got %v", err)
	}
}

func TestSuperuserCanOperateCrossOwner(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	provider, err := svc.CreateProvider(ctx, CreateProviderInput{
		ActorID:          "root",
		ActorIsSuperuser: true,
		OwnerID:          "u-tenant",
		Name:             "provider-super",
		Protocol:         "openai-compatible",
		BaseURL:          "https://example.com/v1",
		APIKey:           "sk-xxx",
	})
	if err != nil {
		t.Fatalf("create provider by superuser: %v", err)
	}

	model, err := svc.CreateModel(ctx, CreateModelInput{
		ActorID:          "root",
		ActorIsSuperuser: true,
		OwnerID:          "u-tenant",
		ProviderID:       provider.ID,
		Name:             "tenant-model",
		UpstreamModel:    "gpt-4.1-mini",
	})
	if err != nil {
		t.Fatalf("create model by superuser: %v", err)
	}

	if _, err := svc.SetModelStatus(ctx, SetModelStatusInput{
		ActorID:          "root",
		ActorIsSuperuser: true,
		ModelID:          model.ID,
		Status:           domain.ModelStatusDisabled,
	}); err != nil {
		t.Fatalf("disable model by superuser: %v", err)
	}
}

func TestActorCanWriteAllowsSubtreeWrite(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	provider, err := svc.CreateProvider(ctx, CreateProviderInput{
		ActorID:          "u-owner",
		ActorIsSuperuser: false,
		OwnerID:          "u-owner",
		Name:             "provider-subtree",
		Protocol:         "openai-compatible",
		BaseURL:          "https://example.com/v1",
		APIKey:           "sk-xxx",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	if _, err := svc.SetProviderStatus(ctx, SetProviderStatusInput{
		ActorID:          "admin-a",
		ActorIsSuperuser: false,
		ActorCanWrite:    true,
		ProviderID:       provider.ID,
		Status:           domain.ProviderStatusDisabled,
	}); err != nil {
		t.Fatalf("expected actor_can_write status update, got %v", err)
	}
}
