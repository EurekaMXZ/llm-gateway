package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"llm-gateway/backend/services/prompt-service/internal/domain"
)

type Repository interface {
	CreateTemplate(ctx context.Context, tpl domain.SceneTemplate) error
	GetTemplateByID(ctx context.Context, id string) (domain.SceneTemplate, error)
	GetTemplateByOwnerScene(ctx context.Context, ownerID string, scene string) (domain.SceneTemplate, error)
	ListTemplates(ctx context.Context, ownerID string) ([]domain.SceneTemplate, error)
	UpdateTemplateStatus(ctx context.Context, id string, status domain.TemplateStatus, updatedAt time.Time) (domain.SceneTemplate, error)
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

type CreateTemplateInput struct {
	ActorID          string
	ActorIsSuperuser bool
	ActorCanWrite    bool
	OwnerID          string
	Scene            string
	Content          string
	Variables        []domain.VariableDefinition
}

func (s *Service) CreateTemplate(ctx context.Context, in CreateTemplateInput) (domain.SceneTemplate, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.OwnerID = strings.TrimSpace(in.OwnerID)
	in.Scene = strings.TrimSpace(in.Scene)
	in.Content = strings.TrimSpace(in.Content)

	if in.ActorID == "" || in.OwnerID == "" || in.Scene == "" || in.Content == "" {
		return domain.SceneTemplate{}, domain.ErrInvalidInput
	}
	if err := authorizeWrite(in.ActorID, in.OwnerID, in.ActorIsSuperuser, in.ActorCanWrite); err != nil {
		return domain.SceneTemplate{}, err
	}
	if err := validateVariables(in.Variables); err != nil {
		return domain.SceneTemplate{}, err
	}

	now := time.Now().UTC()
	tpl := domain.SceneTemplate{
		ID:        randomID(),
		OwnerID:   in.OwnerID,
		Scene:     in.Scene,
		Content:   in.Content,
		Variables: copyVariables(in.Variables),
		Status:    domain.TemplateStatusEnabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateTemplate(ctx, tpl); err != nil {
		return domain.SceneTemplate{}, err
	}
	return tpl, nil
}

func (s *Service) GetTemplate(ctx context.Context, id string) (domain.SceneTemplate, error) {
	ctx = ensureContext(ctx)
	return s.repo.GetTemplateByID(ctx, strings.TrimSpace(id))
}

func (s *Service) ListTemplates(ctx context.Context, ownerID string) ([]domain.SceneTemplate, error) {
	ctx = ensureContext(ctx)
	return s.repo.ListTemplates(ctx, strings.TrimSpace(ownerID))
}

type SetTemplateStatusInput struct {
	ActorID          string
	ActorIsSuperuser bool
	ActorCanWrite    bool
	TemplateID       string
	Status           domain.TemplateStatus
}

func (s *Service) SetTemplateStatus(ctx context.Context, in SetTemplateStatusInput) (domain.SceneTemplate, error) {
	ctx = ensureContext(ctx)
	in.ActorID = strings.TrimSpace(in.ActorID)
	in.TemplateID = strings.TrimSpace(in.TemplateID)
	if in.ActorID == "" || in.TemplateID == "" {
		return domain.SceneTemplate{}, domain.ErrInvalidInput
	}
	if in.Status != domain.TemplateStatusEnabled && in.Status != domain.TemplateStatusDisabled {
		return domain.SceneTemplate{}, domain.ErrInvalidInput
	}
	tpl, err := s.repo.GetTemplateByID(ctx, in.TemplateID)
	if err != nil {
		return domain.SceneTemplate{}, err
	}
	if err := authorizeWrite(in.ActorID, tpl.OwnerID, in.ActorIsSuperuser, in.ActorCanWrite); err != nil {
		return domain.SceneTemplate{}, err
	}
	return s.repo.UpdateTemplateStatus(ctx, in.TemplateID, in.Status, time.Now().UTC())
}

type RenderInput struct {
	OwnerID   string
	Scene     string
	Variables map[string]any
}

type RenderResult struct {
	Prompt string               `json:"prompt"`
	Issues []domain.RenderIssue `json:"issues,omitempty"`
}

func (s *Service) Render(ctx context.Context, in RenderInput) (RenderResult, error) {
	ctx = ensureContext(ctx)
	in.OwnerID = strings.TrimSpace(in.OwnerID)
	in.Scene = strings.TrimSpace(in.Scene)
	if in.OwnerID == "" || in.Scene == "" {
		return RenderResult{}, domain.ErrInvalidInput
	}
	if in.Variables == nil {
		in.Variables = map[string]any{}
	}

	tpl, err := s.repo.GetTemplateByOwnerScene(ctx, in.OwnerID, in.Scene)
	if err != nil {
		return RenderResult{}, err
	}
	if tpl.Status != domain.TemplateStatusEnabled {
		return RenderResult{}, domain.ErrTemplateDisabled
	}

	values := map[string]string{}
	issues := make([]domain.RenderIssue, 0)
	for _, def := range tpl.Variables {
		raw, exists := in.Variables[def.Name]
		if (!exists || raw == nil) && def.DefaultValue != nil {
			raw = *def.DefaultValue
			exists = true
		}
		if !exists || raw == nil {
			if def.Required {
				issues = append(issues, domain.RenderIssue{Code: "missing_variable", Field: def.Name, Message: "required variable is missing"})
			}
			continue
		}

		normalized, ok := normalizeByType(raw, def.Type)
		if !ok {
			issues = append(issues, domain.RenderIssue{Code: "type_mismatch", Field: def.Name, Message: fmt.Sprintf("expected %s", def.Type)})
			continue
		}
		values[def.Name] = normalized
	}

	if len(issues) > 0 {
		return RenderResult{Issues: issues}, domain.ErrRenderValidation
	}

	prompt := tpl.Content
	for key, value := range values {
		prompt = strings.ReplaceAll(prompt, "{{"+key+"}}", value)
	}
	return RenderResult{Prompt: prompt}, nil
}

func validateVariables(defs []domain.VariableDefinition) error {
	seen := map[string]struct{}{}
	for _, def := range defs {
		name := strings.TrimSpace(def.Name)
		if name == "" {
			return domain.ErrInvalidInput
		}
		if _, exists := seen[name]; exists {
			return domain.ErrInvalidInput
		}
		seen[name] = struct{}{}
		switch def.Type {
		case domain.VariableTypeString, domain.VariableTypeNumber, domain.VariableTypeBoolean:
		default:
			return domain.ErrInvalidInput
		}
	}
	return nil
}

func normalizeByType(raw any, t domain.VariableType) (string, bool) {
	switch t {
	case domain.VariableTypeString:
		v, ok := raw.(string)
		if !ok {
			return "", false
		}
		return v, true
	case domain.VariableTypeNumber:
		switch v := raw.(type) {
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64), true
		case float32:
			return strconv.FormatFloat(float64(v), 'f', -1, 64), true
		case int:
			return strconv.Itoa(v), true
		case int64:
			return strconv.FormatInt(v, 10), true
		case int32:
			return strconv.FormatInt(int64(v), 10), true
		case string:
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return "", false
			}
			return v, true
		default:
			return "", false
		}
	case domain.VariableTypeBoolean:
		switch v := raw.(type) {
		case bool:
			return strconv.FormatBool(v), true
		case string:
			if _, err := strconv.ParseBool(v); err != nil {
				return "", false
			}
			return v, true
		default:
			return "", false
		}
	default:
		return "", false
	}
}

func copyVariables(in []domain.VariableDefinition) []domain.VariableDefinition {
	out := make([]domain.VariableDefinition, len(in))
	copy(out, in)
	return out
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
