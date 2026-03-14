package dataplane

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/backend/services/ingress-service/internal/controlplane"
)

func TestChatCompletionsMatchedRouting(t *testing.T) {
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user": map[string]any{"id": "u-1", "username": "alice", "role": "administrator"},
		})
	}))
	defer identityServer.Close()

	apikeyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid":    true,
			"reason":   "ok",
			"status":   "enabled",
			"key_id":   "k-1",
			"owner_id": "u-1",
		})
	}))
	defer apikeyServer.Close()

	promptServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"prompt": "Use concise style"})
	}))
	defer promptServer.Close()

	routingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"matched": true,
			"reason":  "policy_matched",
			"policy": map[string]any{
				"target_provider_id": "p-1",
				"target_model":       "upstream-target-model",
			},
		})
	}))
	defer routingServer.Close()

	executionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OwnerID    string         `json:"owner_id"`
			ProviderID string         `json:"provider_id"`
			Payload    map[string]any `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode execute req: %v", err)
		}
		if req.OwnerID != "u-1" {
			t.Fatalf("unexpected owner id: %s", req.OwnerID)
		}
		if req.ProviderID != "p-1" {
			t.Fatalf("expected routed provider p-1, got %s", req.ProviderID)
		}
		if req.Payload["model"] != "upstream-target-model" {
			t.Fatalf("expected routed model upstream-target-model, got %v", req.Payload["model"])
		}
		msgs, ok := req.Payload["messages"].([]any)
		if !ok || len(msgs) == 0 {
			t.Fatalf("messages missing after template injection")
		}
		first, _ := msgs[0].(map[string]any)
		if first["role"] != "system" {
			t.Fatalf("expected system prompt prepended, got %v", first["role"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"provider_id":    "p-1",
			"upstream_model": "upstream-target-model",
			"response": map[string]any{
				"id":    "chatcmpl-1",
				"model": "upstream-target-model",
			},
		})
	}))
	defer executionServer.Close()

	controlSvc := controlplane.NewService(identityServer.URL, apikeyServer.URL, nil)
	svc := NewService(controlSvc, promptServer.URL, routingServer.URL, executionServer.URL, nil)

	out, err := svc.ChatCompletions(context.Background(), "token-ok", "kgw_x", map[string]any{
		"model": "common_chat",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
		"scene": "default_scene",
		"variables": map[string]any{
			"name": "bob",
		},
	})
	if err != nil {
		t.Fatalf("chat completions error: %v", err)
	}
	if !out.Decision.Allowed {
		t.Fatalf("expected allowed decision, got %+v", out.Decision)
	}
	if out.Response["id"] != "chatcmpl-1" {
		t.Fatalf("unexpected response: %+v", out.Response)
	}
}

func TestChatCompletionsNoRoutingMatchFallsBack(t *testing.T) {
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"id": "u-1", "username": "alice", "role": "administrator"}})
	}))
	defer identityServer.Close()

	apikeyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid":    true,
			"reason":   "ok",
			"status":   "enabled",
			"key_id":   "k-1",
			"owner_id": "u-1",
		})
	}))
	defer apikeyServer.Close()

	promptServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("prompt should not be called without scene")
	}))
	defer promptServer.Close()

	routingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"matched": false,
			"reason":  "no_policy_matched",
		})
	}))
	defer routingServer.Close()

	executionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ProviderID string         `json:"provider_id"`
			Payload    map[string]any `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode execute req: %v", err)
		}
		if req.ProviderID != "" {
			t.Fatalf("expected empty provider_id on no match, got %s", req.ProviderID)
		}
		if req.Payload["model"] != "gpt-4o-mini" {
			t.Fatalf("expected original model kept on no match, got %v", req.Payload["model"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"provider_id":    "p-direct",
			"upstream_model": "gpt-4o-mini",
			"response":       map[string]any{"id": "chatcmpl-direct"},
		})
	}))
	defer executionServer.Close()

	controlSvc := controlplane.NewService(identityServer.URL, apikeyServer.URL, nil)
	svc := NewService(controlSvc, promptServer.URL, routingServer.URL, executionServer.URL, nil)

	out, err := svc.ChatCompletions(context.Background(), "token-ok", "kgw_x", map[string]any{
		"model": "gpt-4o-mini",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	})
	if err != nil {
		t.Fatalf("chat completions error: %v", err)
	}
	if out.Response["id"] != "chatcmpl-direct" {
		t.Fatalf("unexpected response: %+v", out.Response)
	}
}

func TestChatCompletionsPromptValidationError(t *testing.T) {
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"id": "u-1", "username": "alice", "role": "administrator"}})
	}))
	defer identityServer.Close()

	apikeyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid":    true,
			"reason":   "ok",
			"status":   "enabled",
			"key_id":   "k-1",
			"owner_id": "u-1",
		})
	}))
	defer apikeyServer.Close()

	promptServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issues": []map[string]any{{"field": "name", "reason": "missing"}},
		})
	}))
	defer promptServer.Close()

	routingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("routing should not be called when prompt render fails")
	}))
	defer routingServer.Close()

	executionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("execution should not be called when prompt render fails")
	}))
	defer executionServer.Close()

	controlSvc := controlplane.NewService(identityServer.URL, apikeyServer.URL, nil)
	svc := NewService(controlSvc, promptServer.URL, routingServer.URL, executionServer.URL, nil)

	_, err := svc.ChatCompletions(context.Background(), "token-ok", "kgw_x", map[string]any{
		"model": "gpt-4o-mini",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
		"scene": "missing-vars-scene",
	})
	if err == nil {
		t.Fatalf("expected prompt render error")
	}
	var renderErr *PromptRenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("expected PromptRenderError, got %v", err)
	}
}
