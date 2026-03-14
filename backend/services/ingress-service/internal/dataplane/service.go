package dataplane

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-gateway/backend/packages/platform/trace"
	"llm-gateway/backend/services/ingress-service/internal/controlplane"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrDependency   = errors.New("dependency error")
)

type PromptRenderError struct {
	Issues any
}

func (e *PromptRenderError) Error() string {
	return "prompt render failed"
}

type RoutingDecision struct {
	Matched          bool
	Reason           string
	TargetProviderID string
	TargetModel      string
}

type ChatCompletionResult struct {
	Decision      controlplane.Decision
	Response      map[string]any
	ProviderID    string
	UpstreamModel string
}

type Service struct {
	control          *controlplane.Service
	promptBaseURL    string
	routingBaseURL   string
	executionBaseURL string
	httpClient       *http.Client
}

func NewService(control *controlplane.Service, promptBaseURL string, routingBaseURL string, executionBaseURL string, client *http.Client) *Service {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &Service{
		control:          control,
		promptBaseURL:    strings.TrimRight(strings.TrimSpace(promptBaseURL), "/"),
		routingBaseURL:   strings.TrimRight(strings.TrimSpace(routingBaseURL), "/"),
		executionBaseURL: strings.TrimRight(strings.TrimSpace(executionBaseURL), "/"),
		httpClient:       client,
	}
}

func (s *Service) ChatCompletions(ctx context.Context, bearerToken string, apiKey string, req map[string]any) (ChatCompletionResult, error) {
	if s == nil || s.control == nil || req == nil {
		return ChatCompletionResult{}, ErrInvalidInput
	}
	payload, err := cloneMap(req)
	if err != nil {
		return ChatCompletionResult{}, ErrInvalidInput
	}
	model, err := readString(payload, "model")
	if err != nil {
		return ChatCompletionResult{}, ErrInvalidInput
	}

	decision, err := s.control.CheckRequest(ctx, bearerToken, apiKey, model)
	if err != nil {
		return ChatCompletionResult{}, err
	}
	result := ChatCompletionResult{Decision: decision}
	if !decision.Allowed {
		return result, nil
	}

	delete(payload, "api_key")
	delete(payload, "authorization")

	if scene := readOptionalString(payload, "scene"); scene != "" {
		variables, err := readVariables(payload["variables"])
		if err != nil {
			return ChatCompletionResult{}, ErrInvalidInput
		}
		prompt, err := s.renderPrompt(ctx, decision.Identity.ID, scene, variables)
		if err != nil {
			return ChatCompletionResult{}, err
		}
		payload["messages"], err = prependSystemPrompt(payload["messages"], prompt)
		if err != nil {
			return ChatCompletionResult{}, ErrInvalidInput
		}
	}

	delete(payload, "scene")
	delete(payload, "variables")

	difficultyScore := computeDifficultyScore(payload["messages"])
	routing, err := s.resolveRouting(ctx, decision.Identity.ID, model, difficultyScore)
	if err != nil {
		return ChatCompletionResult{}, err
	}
	if routing.Matched {
		if routing.TargetModel == "" {
			return ChatCompletionResult{}, fmt.Errorf("%w: routing response missing target model", ErrDependency)
		}
		payload["model"] = routing.TargetModel
	} else {
		payload["model"] = model
	}

	execResult, err := s.execute(ctx, decision.Identity.ID, routing.TargetProviderID, payload)
	if err != nil {
		return ChatCompletionResult{}, err
	}

	result.ProviderID = execResult.ProviderID
	result.UpstreamModel = execResult.UpstreamModel
	result.Response = execResult.Response
	return result, nil
}

func (s *Service) renderPrompt(ctx context.Context, ownerID string, scene string, variables map[string]any) (string, error) {
	body := map[string]any{
		"owner_id":  ownerID,
		"scene":     scene,
		"variables": variables,
	}
	var okResp struct {
		Prompt string `json:"prompt"`
	}
	var failResp struct {
		Issues any `json:"issues"`
	}

	status, err := s.postJSON(ctx, s.promptBaseURL+"/v1/render", body, &okResp, &failResp)
	if err != nil {
		return "", err
	}
	if status == http.StatusUnprocessableEntity {
		return "", &PromptRenderError{Issues: failResp.Issues}
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("%w: prompt render status=%d", ErrDependency, status)
	}
	if strings.TrimSpace(okResp.Prompt) == "" {
		return "", fmt.Errorf("%w: prompt render response missing prompt", ErrDependency)
	}
	return okResp.Prompt, nil
}

func (s *Service) resolveRouting(ctx context.Context, ownerID string, model string, difficultyScore int) (RoutingDecision, error) {
	body := map[string]any{
		"owner_id":         ownerID,
		"model":            model,
		"difficulty_score": difficultyScore,
	}
	var resp struct {
		Matched bool   `json:"matched"`
		Reason  string `json:"reason"`
		Policy  struct {
			TargetProviderID string `json:"target_provider_id"`
			TargetModel      string `json:"target_model"`
		} `json:"policy"`
	}
	status, err := s.postJSON(ctx, s.routingBaseURL+"/v1/policies/resolve", body, &resp, nil)
	if err != nil {
		return RoutingDecision{}, err
	}
	if status != http.StatusOK {
		return RoutingDecision{}, fmt.Errorf("%w: routing resolve status=%d", ErrDependency, status)
	}
	return RoutingDecision{
		Matched:          resp.Matched,
		Reason:           resp.Reason,
		TargetProviderID: strings.TrimSpace(resp.Policy.TargetProviderID),
		TargetModel:      strings.TrimSpace(resp.Policy.TargetModel),
	}, nil
}

type executeResult struct {
	ProviderID    string         `json:"provider_id"`
	UpstreamModel string         `json:"upstream_model"`
	Response      map[string]any `json:"response"`
}

func (s *Service) execute(ctx context.Context, ownerID string, providerID string, payload map[string]any) (executeResult, error) {
	body := map[string]any{
		"owner_id":    ownerID,
		"provider_id": strings.TrimSpace(providerID),
		"payload":     payload,
	}
	var resp executeResult
	status, err := s.postJSON(ctx, s.executionBaseURL+"/v1/execute/chat/completions", body, &resp, nil)
	if err != nil {
		return executeResult{}, err
	}
	if status != http.StatusOK {
		return executeResult{}, fmt.Errorf("%w: execution status=%d", ErrDependency, status)
	}
	if resp.Response == nil {
		return executeResult{}, fmt.Errorf("%w: execution response missing payload", ErrDependency)
	}
	return resp, nil
}

func (s *Service) postJSON(ctx context.Context, url string, reqBody any, okOut any, failOut any) (int, error) {
	if strings.TrimSpace(url) == "" {
		return 0, ErrInvalidInput
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return 0, ErrInvalidInput
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, ErrInvalidInput
	}
	req.Header.Set("Content-Type", "application/json")
	if traceID := strings.TrimSpace(trace.FromContext(ctx)); traceID != "" {
		req.Header.Set(trace.HeaderTraceID, traceID)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: request failed: %v", ErrDependency, err)
	}
	defer resp.Body.Close()

	if okOut != nil && resp.StatusCode < 400 {
		if err := decodeJSON(resp.Body, okOut); err != nil {
			return 0, fmt.Errorf("%w: decode success payload failed: %v", ErrDependency, err)
		}
	}
	if failOut != nil && resp.StatusCode >= 400 {
		if err := decodeJSON(resp.Body, failOut); err != nil {
			return 0, fmt.Errorf("%w: decode failure payload failed: %v", ErrDependency, err)
		}
	}
	return resp.StatusCode, nil
}

func decodeJSON(r io.Reader, out any) error {
	decoder := json.NewDecoder(io.LimitReader(r, 2<<20))
	if err := decoder.Decode(out); err != nil {
		return err
	}
	return nil
}

func cloneMap(in map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func readString(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok {
		return "", ErrInvalidInput
	}
	s, ok := v.(string)
	if !ok {
		return "", ErrInvalidInput
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrInvalidInput
	}
	return s, nil
}

func readOptionalString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func readVariables(v any) (map[string]any, error) {
	if v == nil {
		return map[string]any{}, nil
	}
	out, ok := v.(map[string]any)
	if !ok {
		return nil, ErrInvalidInput
	}
	return out, nil
}

func prependSystemPrompt(messagesRaw any, prompt string) ([]any, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, ErrInvalidInput
	}
	system := map[string]any{
		"role":    "system",
		"content": prompt,
	}
	if messagesRaw == nil {
		return []any{system}, nil
	}
	messages, ok := messagesRaw.([]any)
	if !ok {
		return nil, ErrInvalidInput
	}
	out := make([]any, 0, len(messages)+1)
	out = append(out, system)
	out = append(out, messages...)
	return out, nil
}

func computeDifficultyScore(messagesRaw any) int {
	messages, ok := messagesRaw.([]any)
	if !ok {
		return 0
	}
	totalChars := 0
	userTurns := 0
	for _, msgRaw := range messages {
		msg, ok := msgRaw.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if !strings.EqualFold(strings.TrimSpace(role), "user") {
			continue
		}
		userTurns++
		totalChars += contentChars(msg["content"])
	}
	score := userTurns*8 + totalChars/24
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}

func contentChars(v any) int {
	switch x := v.(type) {
	case string:
		return len(strings.TrimSpace(x))
	case []any:
		total := 0
		for _, item := range x {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, _ := part["text"].(string)
			total += len(strings.TrimSpace(text))
		}
		return total
	default:
		return 0
	}
}
