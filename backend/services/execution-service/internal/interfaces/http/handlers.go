package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/packages/platform/ginx"
	"llm-gateway/backend/services/execution-service/internal/app"
	"llm-gateway/backend/services/execution-service/internal/domain"
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
		v1.POST("/providers", h.createProvider)
		v1.GET("/providers", h.listProviders)
		v1.GET("/providers/:id", h.getProvider)
		v1.POST("/providers/:id/enable", h.enableProvider)
		v1.POST("/providers/:id/disable", h.disableProvider)

		v1.POST("/models", h.createModel)
		v1.GET("/models", h.listModels)
		v1.GET("/models/:id", h.getModel)
		v1.POST("/models/:id/enable", h.enableModel)
		v1.POST("/models/:id/disable", h.disableModel)
	}
}

type createProviderRequest struct {
	ActorID          string `json:"actor_id"`
	ActorIsSuperuser bool   `json:"actor_is_superuser"`
	ActorCanWrite    bool   `json:"actor_can_write"`
	OwnerID          string `json:"owner_id"`
	Name             string `json:"name"`
	Protocol         string `json:"protocol"`
	BaseURL          string `json:"base_url"`
	APIKey           string `json:"api_key"`
}

func (h *Handler) createProvider(c *gin.Context) {
	var req createProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "execution.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	provider, err := h.service.CreateProvider(c.Request.Context(), app.CreateProviderInput{
		ActorID:          req.ActorID,
		ActorIsSuperuser: req.ActorIsSuperuser,
		ActorCanWrite:    req.ActorCanWrite,
		OwnerID:          req.OwnerID,
		Name:             req.Name,
		Protocol:         req.Protocol,
		BaseURL:          req.BaseURL,
		APIKey:           req.APIKey,
	})
	if err != nil {
		h.writeDomainError(c, err, "provider")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"provider": providerResponse(provider)})
}

func (h *Handler) getProvider(c *gin.Context) {
	provider, err := h.service.GetProvider(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeDomainError(c, err, "provider")
		return
	}
	c.JSON(http.StatusOK, gin.H{"provider": providerResponse(provider)})
}

func (h *Handler) listProviders(c *gin.Context) {
	providers, err := h.service.ListProviders(c.Request.Context(), c.Query("owner_id"))
	if err != nil {
		ginx.JSONError(c, http.StatusInternalServerError, "execution.internal.list_providers_failed", "failed to list providers", "internal")
		return
	}
	items := make([]gin.H, 0, len(providers))
	for _, provider := range providers {
		items = append(items, providerResponse(provider))
	}
	c.JSON(http.StatusOK, gin.H{"providers": items})
}

type setProviderStatusRequest struct {
	ActorID          string `json:"actor_id"`
	ActorIsSuperuser bool   `json:"actor_is_superuser"`
	ActorCanWrite    bool   `json:"actor_can_write"`
}

func (h *Handler) enableProvider(c *gin.Context) {
	h.setProviderStatus(c, domain.ProviderStatusEnabled)
}

func (h *Handler) disableProvider(c *gin.Context) {
	h.setProviderStatus(c, domain.ProviderStatusDisabled)
}

func (h *Handler) setProviderStatus(c *gin.Context, status domain.ProviderStatus) {
	var req setProviderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "execution.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	provider, err := h.service.SetProviderStatus(c.Request.Context(), app.SetProviderStatusInput{
		ActorID:          req.ActorID,
		ActorIsSuperuser: req.ActorIsSuperuser,
		ActorCanWrite:    req.ActorCanWrite,
		ProviderID:       c.Param("id"),
		Status:           status,
	})
	if err != nil {
		h.writeDomainError(c, err, "provider")
		return
	}
	c.JSON(http.StatusOK, gin.H{"provider": providerResponse(provider)})
}

type createModelRequest struct {
	ActorID          string `json:"actor_id"`
	ActorIsSuperuser bool   `json:"actor_is_superuser"`
	ActorCanWrite    bool   `json:"actor_can_write"`
	OwnerID          string `json:"owner_id"`
	ProviderID       string `json:"provider_id"`
	Name             string `json:"name"`
	UpstreamModel    string `json:"upstream_model"`
}

func (h *Handler) createModel(c *gin.Context) {
	var req createModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "execution.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	model, err := h.service.CreateModel(c.Request.Context(), app.CreateModelInput{
		ActorID:          req.ActorID,
		ActorIsSuperuser: req.ActorIsSuperuser,
		ActorCanWrite:    req.ActorCanWrite,
		OwnerID:          req.OwnerID,
		ProviderID:       req.ProviderID,
		Name:             req.Name,
		UpstreamModel:    req.UpstreamModel,
	})
	if err != nil {
		h.writeDomainError(c, err, "model")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"model": modelResponse(model)})
}

func (h *Handler) getModel(c *gin.Context) {
	model, err := h.service.GetModel(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeDomainError(c, err, "model")
		return
	}
	c.JSON(http.StatusOK, gin.H{"model": modelResponse(model)})
}

func (h *Handler) listModels(c *gin.Context) {
	models, err := h.service.ListModels(c.Request.Context(), c.Query("provider_id"), c.Query("owner_id"))
	if err != nil {
		ginx.JSONError(c, http.StatusInternalServerError, "execution.internal.list_models_failed", "failed to list models", "internal")
		return
	}
	items := make([]gin.H, 0, len(models))
	for _, model := range models {
		items = append(items, modelResponse(model))
	}
	c.JSON(http.StatusOK, gin.H{"models": items})
}

type setModelStatusRequest struct {
	ActorID          string `json:"actor_id"`
	ActorIsSuperuser bool   `json:"actor_is_superuser"`
	ActorCanWrite    bool   `json:"actor_can_write"`
}

func (h *Handler) enableModel(c *gin.Context) {
	h.setModelStatus(c, domain.ModelStatusEnabled)
}

func (h *Handler) disableModel(c *gin.Context) {
	h.setModelStatus(c, domain.ModelStatusDisabled)
}

func (h *Handler) setModelStatus(c *gin.Context, status domain.ModelStatus) {
	var req setModelStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "execution.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	model, err := h.service.SetModelStatus(c.Request.Context(), app.SetModelStatusInput{
		ActorID:          req.ActorID,
		ActorIsSuperuser: req.ActorIsSuperuser,
		ActorCanWrite:    req.ActorCanWrite,
		ModelID:          c.Param("id"),
		Status:           status,
	})
	if err != nil {
		h.writeDomainError(c, err, "model")
		return
	}
	c.JSON(http.StatusOK, gin.H{"model": modelResponse(model)})
}

func (h *Handler) writeDomainError(c *gin.Context, err error, entity string) {
	switch {
	case app.IsDomainError(err, domain.ErrInvalidInput):
		ginx.JSONError(c, http.StatusBadRequest, "execution.validation.invalid_input", err.Error(), "validation")
	case app.IsDomainError(err, domain.ErrForbidden):
		ginx.JSONError(c, http.StatusForbidden, "execution.auth.forbidden", err.Error(), "auth")
	case app.IsDomainError(err, domain.ErrProviderNotFound), app.IsDomainError(err, domain.ErrModelNotFound):
		ginx.JSONError(c, http.StatusNotFound, "execution.not_found."+entity, err.Error(), "not_found")
	case app.IsDomainError(err, domain.ErrProviderNameTaken), app.IsDomainError(err, domain.ErrModelNameTaken):
		ginx.JSONError(c, http.StatusConflict, "execution.conflict."+entity, err.Error(), "conflict")
	case app.IsDomainError(err, domain.ErrProviderDisabled):
		ginx.JSONError(c, http.StatusBadRequest, "execution.validation.provider_disabled", err.Error(), "validation")
	default:
		ginx.JSONError(c, http.StatusInternalServerError, "execution.internal.unexpected_error", "unexpected execution error", "internal")
	}
}

func providerResponse(provider domain.Provider) gin.H {
	return gin.H{
		"id":         provider.ID,
		"owner_id":   provider.OwnerID,
		"name":       provider.Name,
		"protocol":   provider.Protocol,
		"base_url":   provider.BaseURL,
		"status":     provider.Status,
		"created_at": provider.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at": provider.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func modelResponse(model domain.Model) gin.H {
	return gin.H{
		"id":             model.ID,
		"provider_id":    model.ProviderID,
		"owner_id":       model.OwnerID,
		"name":           model.Name,
		"upstream_model": model.UpstreamModel,
		"status":         model.Status,
		"created_at":     model.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":     model.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
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
