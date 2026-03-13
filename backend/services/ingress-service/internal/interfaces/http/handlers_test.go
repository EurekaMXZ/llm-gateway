package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/services/ingress-service/internal/controlplane"
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
	NewHandler(svc).RegisterRoutes(engine)

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
	NewHandler(svc).RegisterRoutes(engine)

	resp := doJSON(engine, http.MethodPost, "/v1/control/validate", map[string]any{"model": "gpt-4o-mini"}, "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
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
