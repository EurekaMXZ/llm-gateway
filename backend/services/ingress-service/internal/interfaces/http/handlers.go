package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/packages/platform/ginx"
	"llm-gateway/backend/packages/platform/trace"
	"llm-gateway/backend/services/ingress-service/internal/controlplane"
	"llm-gateway/backend/services/ingress-service/internal/dataplane"
)

type Handler struct {
	control  *controlplane.Service
	dataPath *dataplane.Service
}

func NewHandler(control *controlplane.Service, dataPath *dataplane.Service) *Handler {
	return &Handler{
		control:  control,
		dataPath: dataPath,
	}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	v1 := engine.Group("/v1")
	{
		v1.POST("/control/validate", h.validateControlPlane)
		v1.POST("/chat/completions", h.chatCompletions)
		v1.POST("/responses", h.responses)
	}
}

type validateRequest struct {
	Model         string `json:"model"`
	APIKey        string `json:"api_key"`
	Authorization string `json:"authorization"`
}

func (h *Handler) validateControlPlane(c *gin.Context) {
	var req validateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "ingress.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	bearer := extractBearerToken(req.Authorization)
	if bearer == "" {
		bearer = extractBearerToken(c.GetHeader("Authorization"))
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(c.GetHeader("X-API-Key"))
	}

	decision, err := h.control.CheckRequest(c.Request.Context(), bearer, apiKey, req.Model)
	if err != nil {
		switch {
		case errors.Is(err, controlplane.ErrMissingBearer), errors.Is(err, controlplane.ErrMissingAPIKey):
			ginx.JSONError(c, http.StatusBadRequest, "ingress.validation.missing_credentials", err.Error(), "validation")
		case errors.Is(err, controlplane.ErrUnauthorized):
			ginx.JSONError(c, http.StatusUnauthorized, "ingress.auth.identity_unauthorized", "identity token unauthorized", "auth")
		case errors.Is(err, controlplane.ErrUpstream):
			ginx.JSONError(c, http.StatusBadGateway, "ingress.upstream.control_plane_error", err.Error(), "upstream")
		default:
			ginx.JSONError(c, http.StatusInternalServerError, "ingress.internal.control_plane_error", "control plane validation failed", "internal")
		}
		return
	}

	status := http.StatusOK
	if !decision.Allowed {
		status = http.StatusForbidden
	}
	c.JSON(status, decision)
}

func (h *Handler) chatCompletions(c *gin.Context) {
	req, ok := parseJSONMap(c)
	if !ok {
		return
	}
	result, ok := h.executeChatPipeline(c, req)
	if !ok {
		return
	}
	if !result.Decision.Allowed {
		writeDeniedError(c, result.Decision.Reason)
		return
	}
	c.JSON(http.StatusOK, result.Response)
}

func (h *Handler) responses(c *gin.Context) {
	req, ok := parseJSONMap(c)
	if !ok {
		return
	}
	chatReq, requestedModel, err := responsesToChatRequest(req)
	if err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "ingress.validation.invalid_request", err.Error(), "validation")
		return
	}

	result, ok := h.executeChatPipeline(c, chatReq)
	if !ok {
		return
	}
	if !result.Decision.Allowed {
		writeDeniedError(c, result.Decision.Reason)
		return
	}

	c.JSON(http.StatusOK, chatToResponsesPayload(result.Response, requestedModel))
}

func (h *Handler) executeChatPipeline(c *gin.Context, req map[string]any) (dataplane.ChatCompletionResult, bool) {
	if h.dataPath == nil {
		ginx.JSONError(c, http.StatusNotImplemented, "ingress.internal.dataplane_not_ready", "dataplane service is not configured", "internal")
		return dataplane.ChatCompletionResult{}, false
	}

	bearer := extractBearerToken(c.GetHeader("Authorization"))
	if raw, ok := req["authorization"].(string); ok {
		if t := extractBearerToken(raw); t != "" {
			bearer = t
		}
	}
	apiKey := strings.TrimSpace(c.GetHeader("X-API-Key"))
	if raw, ok := req["api_key"].(string); ok && strings.TrimSpace(raw) != "" {
		apiKey = strings.TrimSpace(raw)
	}

	result, err := h.dataPath.ChatCompletions(c.Request.Context(), bearer, apiKey, req)
	if err != nil {
		writePipelineError(c, err)
		return dataplane.ChatCompletionResult{}, false
	}
	return result, true
}

func parseJSONMap(c *gin.Context) (map[string]any, bool) {
	var req map[string]any
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "ingress.validation.invalid_payload", "invalid request payload", "validation")
		return nil, false
	}
	return req, true
}

func writePipelineError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, dataplane.ErrInvalidInput), errors.Is(err, controlplane.ErrMissingBearer), errors.Is(err, controlplane.ErrMissingAPIKey):
		ginx.JSONError(c, http.StatusBadRequest, "ingress.validation.invalid_request", err.Error(), "validation")
	case errors.Is(err, controlplane.ErrUnauthorized):
		ginx.JSONError(c, http.StatusUnauthorized, "ingress.auth.identity_unauthorized", "identity token unauthorized", "auth")
	case errors.Is(err, controlplane.ErrUpstream), errors.Is(err, dataplane.ErrDependency):
		ginx.JSONError(c, http.StatusBadGateway, "ingress.dependency.pipeline_error", err.Error(), "dependency")
	default:
		var renderErr *dataplane.PromptRenderError
		if errors.As(err, &renderErr) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": gin.H{
					"code":    "ingress.validation.prompt_render_failed",
					"message": "prompt rendering failed",
					"type":    "validation",
				},
				"issues":   renderErr.Issues,
				"trace_id": trace.FromContext(c.Request.Context()),
			})
			return
		}
		ginx.JSONError(c, http.StatusInternalServerError, "ingress.internal.pipeline_error", "dataplane pipeline failed", "internal")
	}
}

func writeDeniedError(c *gin.Context, reason string) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "forbidden"
	}
	ginx.JSONError(c, http.StatusForbidden, "ingress.permission.request_denied", fmt.Sprintf("request denied: %s", reason), "permission")
}

func responsesToChatRequest(req map[string]any) (map[string]any, string, error) {
	model, ok := req["model"].(string)
	model = strings.TrimSpace(model)
	if !ok || model == "" {
		return nil, "", dataplane.ErrInvalidInput
	}
	if stream, _ := req["stream"].(bool); stream {
		return nil, "", dataplane.ErrInvalidInput
	}

	rawInput, exists := req["input"]
	if !exists {
		rawInput = req["messages"]
	}
	messages, err := responsesInputToMessages(rawInput)
	if err != nil {
		return nil, "", err
	}
	if len(messages) == 0 {
		return nil, "", dataplane.ErrInvalidInput
	}
	if instructions, _ := req["instructions"].(string); strings.TrimSpace(instructions) != "" {
		messages = append([]any{map[string]any{
			"role":    "system",
			"content": strings.TrimSpace(instructions),
		}}, messages...)
	}

	chatReq := map[string]any{
		"model":    model,
		"messages": messages,
	}
	for _, k := range []string{
		"temperature", "top_p", "max_tokens", "presence_penalty", "frequency_penalty",
		"n", "stop", "user", "tools", "tool_choice", "response_format", "seed", "scene", "variables",
		"api_key", "authorization",
	} {
		if v, exists := req[k]; exists {
			chatReq[k] = v
		}
	}
	if _, exists := chatReq["max_tokens"]; !exists {
		if v, exists := req["max_output_tokens"]; exists {
			chatReq["max_tokens"] = v
		}
	}
	return chatReq, model, nil
}

func responsesInputToMessages(input any) ([]any, error) {
	switch v := input.(type) {
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return nil, dataplane.ErrInvalidInput
		}
		return []any{map[string]any{"role": "user", "content": text}}, nil
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			msg, ok := item.(map[string]any)
			if !ok {
				return nil, dataplane.ErrInvalidInput
			}
			role, _ := msg["role"].(string)
			role = strings.TrimSpace(role)
			content := msg["content"]
			if role != "" && (isString(content) || isArray(content)) {
				normalized, err := normalizeChatContent(content)
				if err != nil {
					return nil, err
				}
				out = append(out, map[string]any{"role": role, "content": normalized})
				continue
			}
			if msgType, _ := msg["type"].(string); strings.EqualFold(strings.TrimSpace(msgType), "input_text") {
				text, _ := msg["text"].(string)
				text = strings.TrimSpace(text)
				if text == "" {
					return nil, dataplane.ErrInvalidInput
				}
				out = append(out, map[string]any{"role": "user", "content": text})
				continue
			}
			return nil, dataplane.ErrInvalidInput
		}
		return out, nil
	default:
		return nil, dataplane.ErrInvalidInput
	}
}

func normalizeChatContent(content any) (any, error) {
	if text, ok := content.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, dataplane.ErrInvalidInput
		}
		return text, nil
	}
	parts, ok := content.([]any)
	if !ok {
		return nil, dataplane.ErrInvalidInput
	}
	normalized := make([]any, 0, len(parts))
	for _, partRaw := range parts {
		part, ok := partRaw.(map[string]any)
		if !ok {
			return nil, dataplane.ErrInvalidInput
		}
		partType := strings.TrimSpace(asString(part["type"]))
		switch partType {
		case "input_text", "text":
			text := strings.TrimSpace(asString(part["text"]))
			if text == "" {
				return nil, dataplane.ErrInvalidInput
			}
			normalized = append(normalized, map[string]any{
				"type": "text",
				"text": text,
			})
		case "input_image", "image_url":
			imageURL := part["image_url"]
			if imageURL == nil {
				if url := strings.TrimSpace(asString(part["url"])); url != "" {
					imageURL = map[string]any{"url": url}
				}
			}
			if imageURL == nil {
				return nil, dataplane.ErrInvalidInput
			}
			normalized = append(normalized, map[string]any{
				"type":      "image_url",
				"image_url": imageURL,
			})
		default:
			return nil, dataplane.ErrInvalidInput
		}
	}
	return normalized, nil
}

func chatToResponsesPayload(chat map[string]any, requestedModel string) map[string]any {
	model := strings.TrimSpace(asString(chat["model"]))
	if model == "" {
		model = strings.TrimSpace(requestedModel)
	}
	responseID := strings.TrimSpace(asString(chat["id"]))
	if responseID == "" {
		responseID = "resp_" + trace.NewTraceID()
	}
	text := firstAssistantText(chat)
	output := []map[string]any{
		{
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{
					"type": "output_text",
					"text": text,
				},
			},
		},
	}

	out := map[string]any{
		"id":          responseID,
		"object":      "response",
		"created_at":  time.Now().UTC().Unix(),
		"status":      "completed",
		"model":       model,
		"output":      output,
		"output_text": text,
	}
	if usage, ok := chat["usage"]; ok {
		out["usage"] = usage
	}
	return out
}

func firstAssistantText(chat map[string]any) string {
	choices, ok := chat["choices"].([]any)
	if !ok || len(choices) == 0 {
		return ""
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		return ""
	}
	message, ok := choice["message"].(map[string]any)
	if !ok {
		return ""
	}
	if content, ok := message["content"].(string); ok {
		return strings.TrimSpace(content)
	}
	parts, ok := message["content"].([]any)
	if !ok {
		return ""
	}
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		p, ok := part.(map[string]any)
		if !ok {
			continue
		}
		if text := strings.TrimSpace(asString(p["text"])); text != "" {
			segments = append(segments, text)
			continue
		}
		if text := strings.TrimSpace(asString(p["content"])); text != "" {
			segments = append(segments, text)
		}
	}
	return strings.TrimSpace(strings.Join(segments, "\n"))
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func isString(v any) bool {
	_, ok := v.(string)
	return ok
}

func isArray(v any) bool {
	_, ok := v.([]any)
	return ok
}

func extractBearerToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
