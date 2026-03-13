package controlplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckRequestSuccess(t *testing.T) {
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/validate" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user": map[string]any{"id": "u-1", "username": "alice", "role": "administrator"},
		})
	}))
	defer identityServer.Close()

	apiKeyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/keys/validate" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid":    true,
			"reason":   "ok",
			"status":   "enabled",
			"key_id":   "k-1",
			"owner_id": "u-1",
		})
	}))
	defer apiKeyServer.Close()

	svc := NewService(identityServer.URL, apiKeyServer.URL, nil)
	decision, err := svc.CheckRequest(context.Background(), "good-token", "key-plaintext", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("check request: %v", err)
	}
	if !decision.Allowed || decision.Reason != "ok" {
		t.Fatalf("expected allowed decision, got %+v", decision)
	}
}

func TestCheckRequestOwnerMismatch(t *testing.T) {
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
			"owner_id": "u-2",
		})
	}))
	defer apiKeyServer.Close()

	svc := NewService(identityServer.URL, apiKeyServer.URL, nil)
	decision, err := svc.CheckRequest(context.Background(), "good-token", "key-plaintext", "")
	if err != nil {
		t.Fatalf("check request: %v", err)
	}
	if decision.Allowed || decision.Reason != "key_owner_mismatch" {
		t.Fatalf("expected owner mismatch decision, got %+v", decision)
	}
}

func TestCheckRequestUnauthorized(t *testing.T) {
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer identityServer.Close()

	apiKeyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"valid": true, "reason": "ok"})
	}))
	defer apiKeyServer.Close()

	svc := NewService(identityServer.URL, apiKeyServer.URL, nil)
	_, err := svc.CheckRequest(context.Background(), "bad-token", "key-plaintext", "")
	if !Is(err, ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}
