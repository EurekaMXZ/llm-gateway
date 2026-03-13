package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	routingapp "llm-gateway/backend/services/routing-service/internal/app"
)

func TestRoutingPolicyFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := routingapp.NewService()
	engine := gin.New()
	NewHandler(svc).RegisterRoutes(engine)

	createResp := doJSON(engine, http.MethodPost, "/v1/policies", map[string]any{
		"actor_id":           "u-1",
		"actor_is_superuser": false,
		"owner_id":           "u-1",
		"custom_model":       "common_chat",
		"target_provider_id": "provider-a",
		"target_model":       "gpt-4o-mini",
		"priority":           10,
	})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create policy status=%d body=%s", createResp.Code, createResp.Body.String())
	}

	resolveResp := doJSON(engine, http.MethodPost, "/v1/policies/resolve", map[string]any{
		"owner_id":     "u-1",
		"custom_model": "common_chat",
	})
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve status=%d body=%s", resolveResp.Code, resolveResp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resolveResp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal resolve payload: %v", err)
	}
	matched, _ := payload["matched"].(bool)
	if !matched {
		t.Fatalf("expected matched=true payload=%v", payload)
	}
}

func TestRoutingPolicyForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := routingapp.NewService()
	engine := gin.New()
	NewHandler(svc).RegisterRoutes(engine)

	resp := doJSON(engine, http.MethodPost, "/v1/policies", map[string]any{
		"actor_id":           "u-actor",
		"actor_is_superuser": false,
		"owner_id":           "u-owner",
		"custom_model":       "common_chat",
		"target_provider_id": "provider-a",
		"target_model":       "gpt-4o-mini",
		"priority":           10,
	})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got=%d body=%s", resp.Code, resp.Body.String())
	}
}

func doJSON(engine *gin.Engine, method string, path string, body any) *httptest.ResponseRecorder {
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
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}
