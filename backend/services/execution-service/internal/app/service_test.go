package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestExecuteChatUsesProviderPriority(t *testing.T) {
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "chatcmpl-test",
			"model": payload["model"],
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
			"choices": []map[string]any{
				{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"},
			},
		})
	}))
	defer upstream.Close()

	svc := NewServiceWithRepositoryAndClient(NewInMemoryRepository(), upstream.Client())

	p1 := 20
	providerA, err := svc.CreateProvider(ctx, CreateProviderInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		Name:             "provider-a",
		Protocol:         "openai-compatible",
		BaseURL:          upstream.URL,
		APIKey:           "sk-a",
		Priority:         &p1,
	})
	if err != nil {
		t.Fatalf("create providerA: %v", err)
	}

	p2 := 5
	providerB, err := svc.CreateProvider(ctx, CreateProviderInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		Name:             "provider-b",
		Protocol:         "openai-compatible",
		BaseURL:          upstream.URL,
		APIKey:           "sk-b",
		Priority:         &p2,
	})
	if err != nil {
		t.Fatalf("create providerB: %v", err)
	}

	if _, err := svc.CreateModel(ctx, CreateModelInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		ProviderID:       providerA.ID,
		Name:             "gpt-4o-mini",
		UpstreamModel:    "provider-a-upstream",
	}); err != nil {
		t.Fatalf("create model A: %v", err)
	}
	if _, err := svc.CreateModel(ctx, CreateModelInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		ProviderID:       providerB.ID,
		Name:             "gpt-4o-mini",
		UpstreamModel:    "provider-b-upstream",
	}); err != nil {
		t.Fatalf("create model B: %v", err)
	}

	result, err := svc.ExecuteChat(ctx, ExecuteChatInput{
		OwnerID: "u-1",
		Payload: map[string]any{
			"model": "gpt-4o-mini",
			"messages": []map[string]any{
				{"role": "user", "content": "hello"},
			},
		},
	})
	if err != nil {
		t.Fatalf("execute chat: %v", err)
	}
	if result.ProviderID != providerB.ID {
		t.Fatalf("expected providerB by priority, got %s", result.ProviderID)
	}
	if model, _ := result.Response["model"].(string); model != "provider-b-upstream" {
		t.Fatalf("expected upstream model provider-b-upstream, got %v", result.Response["model"])
	}
}

func TestExecuteChatRejectsStream(t *testing.T) {
	svc := NewService()
	_, err := svc.ExecuteChat(context.Background(), ExecuteChatInput{
		OwnerID: "u-1",
		Payload: map[string]any{
			"model":  "gpt-4o-mini",
			"stream": true,
		},
	})
	if !IsDomainError(err, domain.ErrInvalidInput) {
		t.Fatalf("expected invalid input for stream=true, got %v", err)
	}
}
