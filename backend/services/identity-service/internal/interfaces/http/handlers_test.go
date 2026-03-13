package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	identityapp "llm-gateway/backend/services/identity-service/internal/app"
)

func TestIdentityHTTPFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := identityapp.NewService("test-secret")
	engine := gin.New()
	NewHandler(svc).RegisterRoutes(engine)

	bootstrapBody := map[string]any{
		"username":     "root",
		"password":     "pass",
		"display_name": "Root",
	}
	resp := doJSON(engine, http.MethodPost, "/v1/auth/bootstrap-superuser", bootstrapBody, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("bootstrap status=%d body=%s", resp.Code, resp.Body.String())
	}

	tokenResp := doJSON(engine, http.MethodPost, "/v1/auth/token", map[string]any{"username": "root", "password": "pass"}, "")
	if tokenResp.Code != http.StatusOK {
		t.Fatalf("token status=%d body=%s", tokenResp.Code, tokenResp.Body.String())
	}
	var tokenPayload map[string]any
	if err := json.Unmarshal(tokenResp.Body.Bytes(), &tokenPayload); err != nil {
		t.Fatalf("unmarshal token payload: %v", err)
	}
	token, _ := tokenPayload["access_token"].(string)
	if token == "" {
		t.Fatalf("empty access token")
	}

	validateResp := doJSON(engine, http.MethodGet, "/v1/auth/validate", nil, "Bearer "+token)
	if validateResp.Code != http.StatusOK {
		t.Fatalf("validate status=%d body=%s", validateResp.Code, validateResp.Body.String())
	}
}

func TestIdentityPermissionCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := identityapp.NewService("test-secret")
	engine := gin.New()
	NewHandler(svc).RegisterRoutes(engine)

	super := doJSON(engine, http.MethodPost, "/v1/auth/bootstrap-superuser", map[string]any{"username": "root", "password": "pass"}, "")
	if super.Code != http.StatusCreated {
		t.Fatalf("bootstrap status=%d", super.Code)
	}

	rootTokenResp := doJSON(engine, http.MethodPost, "/v1/auth/token", map[string]any{"username": "root", "password": "pass"}, "")
	if rootTokenResp.Code != http.StatusOK {
		t.Fatalf("root token status=%d body=%s", rootTokenResp.Code, rootTokenResp.Body.String())
	}
	rootToken := extractAccessToken(rootTokenResp.Body.Bytes())
	if rootToken == "" {
		t.Fatalf("missing root token")
	}

	unauthorized := doJSON(engine, http.MethodPost, "/v1/users", map[string]any{
		"username":  "unauthorized",
		"password":  "pass",
		"role":      "regular_user",
		"parent_id": extractUserID(super.Body.Bytes()),
	}, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized create, status=%d body=%s", unauthorized.Code, unauthorized.Body.String())
	}

	adminResp := doJSON(engine, http.MethodPost, "/v1/users", map[string]any{
		"username":  "admin-a",
		"password":  "pass",
		"role":      "administrator",
		"parent_id": extractUserID(super.Body.Bytes()),
	}, "Bearer "+rootToken)
	if adminResp.Code != http.StatusCreated {
		t.Fatalf("create admin status=%d body=%s", adminResp.Code, adminResp.Body.String())
	}
	adminID := extractUserID(adminResp.Body.Bytes())

	adminTokenResp := doJSON(engine, http.MethodPost, "/v1/auth/token", map[string]any{"username": "admin-a", "password": "pass"}, "")
	if adminTokenResp.Code != http.StatusOK {
		t.Fatalf("admin token status=%d body=%s", adminTokenResp.Code, adminTokenResp.Body.String())
	}
	adminToken := extractAccessToken(adminTokenResp.Body.Bytes())
	if adminToken == "" {
		t.Fatalf("missing admin token")
	}
	userResp := doJSON(engine, http.MethodPost, "/v1/users", map[string]any{
		"username":  "user-a1",
		"password":  "pass",
		"role":      "regular_user",
		"parent_id": adminID,
	}, "Bearer "+adminToken)
	if userResp.Code != http.StatusCreated {
		t.Fatalf("create user status=%d body=%s", userResp.Code, userResp.Body.String())
	}

	checkResp := doJSON(engine, http.MethodPost, "/v1/permissions/check", map[string]any{
		"actor_id":          adminID,
		"resource_owner_id": extractUserID(userResp.Body.Bytes()),
	}, "")
	if checkResp.Code != http.StatusOK {
		t.Fatalf("check status=%d body=%s", checkResp.Code, checkResp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(checkResp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal check payload: %v", err)
	}
	allowed, _ := payload["allowed"].(bool)
	if !allowed {
		t.Fatalf("expected allowed permission check, payload=%v", payload)
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

func extractUserID(body []byte) string {
	var payload map[string]map[string]any
	_ = json.Unmarshal(body, &payload)
	if user, ok := payload["user"]; ok {
		if id, ok := user["id"].(string); ok {
			return id
		}
	}
	return ""
}

func extractAccessToken(body []byte) string {
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)
	token, _ := payload["access_token"].(string)
	return token
}
