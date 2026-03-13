package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/packages/platform/ginx"
	"llm-gateway/backend/services/apikey-service/internal/app"
	"llm-gateway/backend/services/apikey-service/internal/domain"
)

type Handler struct {
	service *app.Service
}

func NewHandler(service *app.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	v1 := engine.Group("/v1")
	{
		v1.POST("/keys", h.createKey)
		v1.GET("/keys/:id", h.getKey)
		v1.POST("/keys/:id/disable", h.disableKey)
		v1.POST("/keys/:id/enable", h.enableKey)
		v1.POST("/keys/validate", h.validateKey)
	}
}

type createKeyRequest struct {
	OwnerID       string   `json:"owner_id"`
	Name          string   `json:"name"`
	AllowedModels []string `json:"allowed_models"`
	ExpiresAt     string   `json:"expires_at"`
}

func (h *Handler) createKey(c *gin.Context) {
	var req createKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "apikey.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			ginx.JSONError(c, http.StatusBadRequest, "apikey.validation.invalid_expires_at", "expires_at must be RFC3339", "validation")
			return
		}
		expiresAt = &parsed
	}

	record, plainKey, err := h.service.CreateKey(c.Request.Context(), app.CreateKeyInput{
		OwnerID:       req.OwnerID,
		Name:          req.Name,
		AllowedModels: req.AllowedModels,
		ExpiresAt:     expiresAt,
	})
	if err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "apikey.validation.create_key_failed", err.Error(), "validation")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"key":     keyResponse(record),
		"api_key": plainKey,
	})
}

func (h *Handler) getKey(c *gin.Context) {
	record, err := h.service.GetKey(c.Request.Context(), c.Param("id"))
	if err != nil {
		ginx.JSONError(c, http.StatusNotFound, "apikey.not_found.key", err.Error(), "not_found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": keyResponse(record)})
}

func (h *Handler) disableKey(c *gin.Context) {
	record, err := h.service.DisableKey(c.Request.Context(), c.Param("id"))
	if err != nil {
		ginx.JSONError(c, http.StatusNotFound, "apikey.not_found.key", err.Error(), "not_found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": keyResponse(record)})
}

func (h *Handler) enableKey(c *gin.Context) {
	record, err := h.service.EnableKey(c.Request.Context(), c.Param("id"))
	if err != nil {
		ginx.JSONError(c, http.StatusNotFound, "apikey.not_found.key", err.Error(), "not_found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": keyResponse(record)})
}

type validateKeyRequest struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model"`
}

func (h *Handler) validateKey(c *gin.Context) {
	var req validateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "apikey.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	result, err := h.service.Validate(c.Request.Context(), req.APIKey, req.Model, time.Now().UTC())
	if err != nil {
		ginx.JSONError(c, http.StatusInternalServerError, "apikey.internal.validation_failed", "failed to validate api key", "internal")
		return
	}
	if !result.Valid {
		c.JSON(http.StatusOK, gin.H{"valid": false, "reason": result.Reason, "status": result.Status, "key_id": result.KeyID, "owner_id": result.OwnerID})
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": true, "reason": result.Reason, "status": result.Status, "key_id": result.KeyID, "owner_id": result.OwnerID})
}

func keyResponse(k domain.APIKey) gin.H {
	var expiresAt any = nil
	if k.ExpiresAt != nil {
		expiresAt = k.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return gin.H{
		"id":             k.ID,
		"owner_id":       k.OwnerID,
		"name":           k.Name,
		"status":         k.Status,
		"allowed_models": k.AllowedModels,
		"expires_at":     expiresAt,
		"created_at":     k.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":     k.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
