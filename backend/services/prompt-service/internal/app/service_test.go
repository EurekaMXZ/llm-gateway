package app

import (
	"context"
	"testing"

	"llm-gateway/backend/services/prompt-service/internal/domain"
)

func TestCreateTemplateOwnershipConstraint(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	_, err := svc.CreateTemplate(ctx, CreateTemplateInput{
		ActorID:          "u-a",
		ActorIsSuperuser: false,
		OwnerID:          "u-b",
		Scene:            "chat_assistant",
		Content:          "Hello {{name}}",
		Variables: []domain.VariableDefinition{
			{Name: "name", Type: domain.VariableTypeString, Required: true},
		},
	})
	if !IsDomainError(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestRenderValidationAndSuccess(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	_, err := svc.CreateTemplate(ctx, CreateTemplateInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		Scene:            "chat_assistant",
		Content:          "Name={{name}} Score={{score}}",
		Variables: []domain.VariableDefinition{
			{Name: "name", Type: domain.VariableTypeString, Required: true},
			{Name: "score", Type: domain.VariableTypeNumber, Required: true},
		},
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	res, err := svc.Render(ctx, RenderInput{
		OwnerID: "u-1",
		Scene:   "chat_assistant",
		Variables: map[string]any{
			"name": "alice",
		},
	})
	if !IsDomainError(err, domain.ErrRenderValidation) {
		t.Fatalf("expected render validation error, got %v", err)
	}
	if len(res.Issues) == 0 {
		t.Fatalf("expected render issues")
	}

	res, err = svc.Render(ctx, RenderInput{
		OwnerID: "u-1",
		Scene:   "chat_assistant",
		Variables: map[string]any{
			"name":  "alice",
			"score": 95,
		},
	})
	if err != nil {
		t.Fatalf("render success: %v", err)
	}
	if res.Prompt == "" {
		t.Fatalf("expected prompt output")
	}
}

func TestActorCanWriteAllowsSubtreeTemplateMutation(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	tpl, err := svc.CreateTemplate(ctx, CreateTemplateInput{
		ActorID:          "u-1",
		ActorIsSuperuser: false,
		OwnerID:          "u-1",
		Scene:            "subtree_scene",
		Content:          "Hi {{name}}",
		Variables: []domain.VariableDefinition{
			{Name: "name", Type: domain.VariableTypeString, Required: true},
		},
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	if _, err := svc.SetTemplateStatus(ctx, SetTemplateStatusInput{
		ActorID:          "admin-a",
		ActorIsSuperuser: false,
		ActorCanWrite:    true,
		TemplateID:       tpl.ID,
		Status:           domain.TemplateStatusDisabled,
	}); err != nil {
		t.Fatalf("expected actor_can_write status update, got %v", err)
	}
}
