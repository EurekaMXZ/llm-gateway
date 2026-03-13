package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	apikeyapp "llm-gateway/backend/services/apikey-service/internal/app"
)

func TestAPIKeyHTTPFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := apikeyapp.NewService()
	engine := gin.New()
	NewHandler(svc).RegisterRoutes(engine)

	createResp := doJSON(engine, http.MethodPost, "/v1/keys", map[string]any{
		"owner_id":       "u-1",
		"name":           "dev",
		"allowed_models": []string{"gpt-4o-mini"},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create key status=%d body=%s", createResp.Code, createResp.Body.String())
	}

	var createPayload map[string]any
	if err := json.Unmarshal(createResp.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("unmarshal create payload: %v", err)
	}
	apiKey, _ := createPayload["api_key"].(string)
	keyID := extractKeyID(createResp.Body.Bytes())
	if apiKey == "" || keyID == "" {
		t.Fatalf("missing api key or key id")
	}

	validateResp := doJSON(engine, http.MethodPost, "/v1/keys/validate", map[string]any{
		"api_key": apiKey,
		"model":   "gpt-4o-mini",
	}, "")
	if validateResp.Code != http.StatusOK {
		t.Fatalf("validate status=%d body=%s", validateResp.Code, validateResp.Body.String())
	}

	missingModelResp := doJSON(engine, http.MethodPost, "/v1/keys/validate", map[string]any{
		"api_key": apiKey,
	}, "")
	if missingModelResp.Code != http.StatusOK {
		t.Fatalf("validate missing model status=%d body=%s", missingModelResp.Code, missingModelResp.Body.String())
	}
	var missingModelPayload map[string]any
	if err := json.Unmarshal(missingModelResp.Body.Bytes(), &missingModelPayload); err != nil {
		t.Fatalf("unmarshal missing model payload: %v", err)
	}
	validMissingModel, _ := missingModelPayload["valid"].(bool)
	if validMissingModel {
		t.Fatalf("expected missing model to be invalid for restricted key")
	}
	if reason, _ := missingModelPayload["reason"].(string); reason != "model_required" {
		t.Fatalf("expected reason=model_required, got %q", reason)
	}

	disableResp := doJSON(engine, http.MethodPost, "/v1/keys/"+keyID+"/disable", map[string]any{}, "")
	if disableResp.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disableResp.Code, disableResp.Body.String())
	}

	invalidResp := doJSON(engine, http.MethodPost, "/v1/keys/validate", map[string]any{
		"api_key": apiKey,
		"model":   "gpt-4o-mini",
	}, "")
	if invalidResp.Code != http.StatusOK {
		t.Fatalf("validate disabled status=%d body=%s", invalidResp.Code, invalidResp.Body.String())
	}
	var invalidPayload map[string]any
	if err := json.Unmarshal(invalidResp.Body.Bytes(), &invalidPayload); err != nil {
		t.Fatalf("unmarshal invalid payload: %v", err)
	}
	valid, _ := invalidPayload["valid"].(bool)
	if valid {
		t.Fatalf("expected disabled key to be invalid")
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

func extractKeyID(body []byte) string {
	var payload map[string]map[string]any
	_ = json.Unmarshal(body, &payload)
	if key, ok := payload["key"]; ok {
		if id, ok := key["id"].(string); ok {
			return id
		}
	}
	return ""
}
