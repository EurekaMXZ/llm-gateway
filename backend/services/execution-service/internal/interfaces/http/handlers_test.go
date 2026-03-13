package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	execapp "llm-gateway/backend/services/execution-service/internal/app"
)

func TestExecutionProviderModelFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := execapp.NewService()
	engine := gin.New()
	NewHandler(svc).RegisterRoutes(engine)

	createProvider := doJSON(engine, http.MethodPost, "/v1/providers", map[string]any{
		"actor_id":           "u-1",
		"actor_is_superuser": false,
		"owner_id":           "u-1",
		"name":               "provider-a",
		"protocol":           "openai-compatible",
		"base_url":           "https://example.com/v1",
		"api_key":            "sk-123",
	})
	if createProvider.Code != http.StatusCreated {
		t.Fatalf("create provider status=%d body=%s", createProvider.Code, createProvider.Body.String())
	}
	providerID := extractID(createProvider.Body.Bytes(), "provider")
	if providerID == "" {
		t.Fatalf("missing provider id")
	}

	createModel := doJSON(engine, http.MethodPost, "/v1/models", map[string]any{
		"actor_id":           "u-1",
		"actor_is_superuser": false,
		"owner_id":           "u-1",
		"provider_id":        providerID,
		"name":               "tenant-model",
		"upstream_model":     "gpt-4o-mini",
	})
	if createModel.Code != http.StatusCreated {
		t.Fatalf("create model status=%d body=%s", createModel.Code, createModel.Body.String())
	}

	listModels := doJSON(engine, http.MethodGet, "/v1/models?provider_id="+providerID, nil)
	if listModels.Code != http.StatusOK {
		t.Fatalf("list models status=%d body=%s", listModels.Code, listModels.Body.String())
	}
}

func TestExecutionForbiddenCreateProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := execapp.NewService()
	engine := gin.New()
	NewHandler(svc).RegisterRoutes(engine)

	resp := doJSON(engine, http.MethodPost, "/v1/providers", map[string]any{
		"actor_id":           "u-actor",
		"actor_is_superuser": false,
		"owner_id":           "u-owner",
		"name":               "provider-a",
		"protocol":           "openai-compatible",
		"base_url":           "https://example.com/v1",
		"api_key":            "sk-123",
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

func extractID(payload []byte, key string) string {
	var body map[string]map[string]any
	_ = json.Unmarshal(payload, &body)
	item, ok := body[key]
	if !ok {
		return ""
	}
	id, _ := item["id"].(string)
	return id
}
