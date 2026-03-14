package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/services/ingress-service/internal/controlplane"
	"llm-gateway/backend/services/ingress-service/internal/dataplane"
)

func TestValidateControlPlaneHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-ok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"id": "u-1", "username": "alice", "role": "administrator"}})
	}))
	defer identityServer.Close()

	apiKeyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid":    true,
			"reason":   "ok",
			"status":   "enabled",
			"key_id":   "k-1",
			"owner_id": "u-1",
		})
	}))
	defer apiKeyServer.Close()

	svc := controlplane.NewService(identityServer.URL, apiKeyServer.URL, nil)
	engine := gin.New()
	NewHandler(svc, nil).RegisterRoutes(engine)

	body := map[string]any{"model": "gpt-4o-mini", "api_key": "kgw_x"}
	resp := doJSON(engine, http.MethodPost, "/v1/control/validate", body, "Bearer token-ok")
	if resp.Code != http.StatusOK {
		t.Fatalf("validate status=%d body=%s", resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	allowed, _ := payload["allowed"].(bool)
	if !allowed {
		t.Fatalf("expected allowed=true payload=%v", payload)
	}
}

func TestValidateControlPlaneMissingCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := controlplane.NewService("http://identity.invalid", "http://apikey.invalid", nil)
	engine := gin.New()
	NewHandler(svc, nil).RegisterRoutes(engine)

	resp := doJSON(engine, http.MethodPost, "/v1/control/validate", map[string]any{"model": "gpt-4o-mini"}, "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestChatCompletionsHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"id": "u-1", "username": "alice", "role": "administrator"}})
	}))
	defer identityServer.Close()

	apiKeyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid":    true,
			"reason":   "ok",
			"status":   "enabled",
			"key_id":   "k-1",
			"owner_id": "u-1",
		})
	}))
	defer apiKeyServer.Close()

	promptServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"prompt": "system prompt"})
	}))
	defer promptServer.Close()

	routingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"matched": false, "reason": "no_policy_matched"})
	}))
	defer routingServer.Close()

	executionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode execution req: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"provider_id":    "p-direct",
			"upstream_model": "gpt-4o-mini",
			"response": map[string]any{
				"id":    "chatcmpl-xyz",
				"model": "gpt-4o-mini",
			},
		})
	}))
	defer executionServer.Close()

	controlSvc := controlplane.NewService(identityServer.URL, apiKeyServer.URL, nil)
	dataSvc := dataplane.NewService(controlSvc, promptServer.URL, routingServer.URL, executionServer.URL, nil)
	engine := gin.New()
	NewHandler(controlSvc, dataSvc).RegisterRoutes(engine)

	reqBody := map[string]any{
		"model": "gpt-4o-mini",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
		"scene": "default_scene",
	}
	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token-ok")
	req.Header.Set("X-API-Key", "kgw_x")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["id"] != "chatcmpl-xyz" {
		t.Fatalf("unexpected response body: %v", payload)
	}
}

func TestResponsesHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"id": "u-1", "username": "alice", "role": "administrator"}})
	}))
	defer identityServer.Close()

	apiKeyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid":    true,
			"reason":   "ok",
			"status":   "enabled",
			"key_id":   "k-1",
			"owner_id": "u-1",
		})
	}))
	defer apiKeyServer.Close()

	promptServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("prompt should not be called in this test")
	}))
	defer promptServer.Close()

	routingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"matched": false, "reason": "no_policy_matched"})
	}))
	defer routingServer.Close()

	executionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Payload map[string]any `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode execution req: %v", err)
		}
		if req.Payload["model"] != "gpt-4o-mini" {
			t.Fatalf("unexpected model forwarded: %v", req.Payload["model"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"provider_id":    "p-direct",
			"upstream_model": "gpt-4o-mini",
			"response": map[string]any{
				"id":    "chatcmpl-responses",
				"model": "gpt-4o-mini",
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role":    "assistant",
							"content": "hello from responses",
						},
					},
				},
				"usage": map[string]any{"total_tokens": 12},
			},
		})
	}))
	defer executionServer.Close()

	controlSvc := controlplane.NewService(identityServer.URL, apiKeyServer.URL, nil)
	dataSvc := dataplane.NewService(controlSvc, promptServer.URL, routingServer.URL, executionServer.URL, nil)
	engine := gin.New()
	NewHandler(controlSvc, dataSvc).RegisterRoutes(engine)

	reqBody := map[string]any{
		"model": "gpt-4o-mini",
		"input": "hello",
	}
	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token-ok")
	req.Header.Set("X-API-Key", "kgw_x")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["object"] != "response" {
		t.Fatalf("unexpected object: %v", payload["object"])
	}
	if payload["output_text"] != "hello from responses" {
		t.Fatalf("unexpected output_text: %v", payload["output_text"])
	}
}

func TestResponsesHTTPNormalizesInputTextParts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"id": "u-1", "username": "alice", "role": "administrator"}})
	}))
	defer identityServer.Close()

	apiKeyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid":    true,
			"reason":   "ok",
			"status":   "enabled",
			"key_id":   "k-1",
			"owner_id": "u-1",
		})
	}))
	defer apiKeyServer.Close()

	promptServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("prompt should not be called in this test")
	}))
	defer promptServer.Close()

	routingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"matched": false, "reason": "no_policy_matched"})
	}))
	defer routingServer.Close()

	executionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Payload map[string]any `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode execution req: %v", err)
		}
		msgs, ok := req.Payload["messages"].([]any)
		if !ok || len(msgs) == 0 {
			t.Fatalf("missing messages in forwarded payload: %+v", req.Payload)
		}
		first, _ := msgs[0].(map[string]any)
		content, ok := first["content"].([]any)
		if !ok || len(content) == 0 {
			t.Fatalf("expected array content for first message, got %T", first["content"])
		}
		part, _ := content[0].(map[string]any)
		if part["type"] != "text" {
			t.Fatalf("expected normalized type=text, got %v", part["type"])
		}
		if part["text"] != "hello part" {
			t.Fatalf("expected normalized text, got %v", part["text"])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"provider_id":    "p-direct",
			"upstream_model": "gpt-4o-mini",
			"response": map[string]any{
				"id":    "chatcmpl-responses",
				"model": "gpt-4o-mini",
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role":    "assistant",
							"content": "normalized",
						},
					},
				},
			},
		})
	}))
	defer executionServer.Close()

	controlSvc := controlplane.NewService(identityServer.URL, apiKeyServer.URL, nil)
	dataSvc := dataplane.NewService(controlSvc, promptServer.URL, routingServer.URL, executionServer.URL, nil)
	engine := gin.New()
	NewHandler(controlSvc, dataSvc).RegisterRoutes(engine)

	reqBody := map[string]any{
		"model": "gpt-4o-mini",
		"input": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "input_text",
						"text": "hello part",
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token-ok")
	req.Header.Set("X-API-Key", "kgw_x")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestChatCompletionsDeniedUsesErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"id": "u-1", "username": "alice", "role": "administrator"}})
	}))
	defer identityServer.Close()

	apiKeyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid":    false,
			"reason":   "model_not_allowed",
			"status":   "enabled",
			"key_id":   "k-1",
			"owner_id": "u-1",
		})
	}))
	defer apiKeyServer.Close()

	controlSvc := controlplane.NewService(identityServer.URL, apiKeyServer.URL, nil)
	dataSvc := dataplane.NewService(controlSvc, "http://prompt.invalid", "http://routing.invalid", "http://execution.invalid", nil)
	engine := gin.New()
	NewHandler(controlSvc, dataSvc).RegisterRoutes(engine)

	reqBody := map[string]any{
		"model": "gpt-4o-mini",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}
	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token-ok")
	req.Header.Set("X-API-Key", "kgw_x")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload["error"].(map[string]any); !ok {
		t.Fatalf("expected standard error envelope, got %v", payload)
	}
	if _, hasAllowed := payload["allowed"]; hasAllowed {
		t.Fatalf("unexpected raw decision payload: %v", payload)
	}
}

func doJSON(engine *gin.Engine, method string, path string, body any, auth string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}
