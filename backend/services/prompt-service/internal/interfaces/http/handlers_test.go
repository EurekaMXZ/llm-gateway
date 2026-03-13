package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	promptapp "llm-gateway/backend/services/prompt-service/internal/app"
)

func TestPromptTemplateAndRenderFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := promptapp.NewService()
	engine := gin.New()
	NewHandler(svc).RegisterRoutes(engine)

	create := doJSON(engine, http.MethodPost, "/v1/templates", map[string]any{
		"actor_id":           "u-1",
		"actor_is_superuser": false,
		"owner_id":           "u-1",
		"scene":              "chat_assistant",
		"content":            "Hi {{name}}",
		"variables": []map[string]any{
			{"name": "name", "type": "string", "required": true},
		},
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create template status=%d body=%s", create.Code, create.Body.String())
	}

	render := doJSON(engine, http.MethodPost, "/v1/render", map[string]any{
		"owner_id": "u-1",
		"scene":    "chat_assistant",
		"variables": map[string]any{
			"name": "alice",
		},
	})
	if render.Code != http.StatusOK {
		t.Fatalf("render status=%d body=%s", render.Code, render.Body.String())
	}
}

func TestPromptRenderValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := promptapp.NewService()
	engine := gin.New()
	NewHandler(svc).RegisterRoutes(engine)

	_ = doJSON(engine, http.MethodPost, "/v1/templates", map[string]any{
		"actor_id":           "u-1",
		"actor_is_superuser": false,
		"owner_id":           "u-1",
		"scene":              "chat_assistant",
		"content":            "Hi {{name}}",
		"variables": []map[string]any{
			{"name": "name", "type": "string", "required": true},
		},
	})

	render := doJSON(engine, http.MethodPost, "/v1/render", map[string]any{
		"owner_id": "u-1",
		"scene":    "chat_assistant",
		"variables": map[string]any{
			"name": 123,
		},
	})
	if render.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 got=%d body=%s", render.Code, render.Body.String())
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
