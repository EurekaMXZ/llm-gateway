package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/packages/platform/ginx"
	"llm-gateway/backend/services/ingress-service/internal/controlplane"
)

type Handler struct {
	service *controlplane.Service
}

func NewHandler(service *controlplane.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	v1 := engine.Group("/v1")
	{
		v1.POST("/control/validate", h.validateControlPlane)
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

	decision, err := h.service.CheckRequest(c.Request.Context(), bearer, apiKey, req.Model)
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
