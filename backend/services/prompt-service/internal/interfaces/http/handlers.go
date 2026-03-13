package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/packages/platform/ginx"
	promptapp "llm-gateway/backend/services/prompt-service/internal/app"
	"llm-gateway/backend/services/prompt-service/internal/domain"
)

type Handler struct {
	service *promptapp.Service
}

func NewHandler(service *promptapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	v1 := engine.Group("/v1")
	{
		v1.POST("/templates", h.createTemplate)
		v1.GET("/templates", h.listTemplates)
		v1.GET("/templates/:id", h.getTemplate)
		v1.POST("/templates/:id/enable", h.enableTemplate)
		v1.POST("/templates/:id/disable", h.disableTemplate)
		v1.POST("/render", h.render)
	}
}

type createTemplateRequest struct {
	ActorID          string                      `json:"actor_id"`
	ActorIsSuperuser bool                        `json:"actor_is_superuser"`
	ActorCanWrite    bool                        `json:"actor_can_write"`
	OwnerID          string                      `json:"owner_id"`
	Scene            string                      `json:"scene"`
	Content          string                      `json:"content"`
	Variables        []domain.VariableDefinition `json:"variables"`
}

func (h *Handler) createTemplate(c *gin.Context) {
	var req createTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "prompt.validation.invalid_payload", "invalid request payload", "validation")
		return
	}
	tpl, err := h.service.CreateTemplate(c.Request.Context(), promptapp.CreateTemplateInput{
		ActorID:          req.ActorID,
		ActorIsSuperuser: req.ActorIsSuperuser,
		ActorCanWrite:    req.ActorCanWrite,
		OwnerID:          req.OwnerID,
		Scene:            req.Scene,
		Content:          req.Content,
		Variables:        req.Variables,
	})
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"template": templateResponse(tpl)})
}

func (h *Handler) getTemplate(c *gin.Context) {
	tpl, err := h.service.GetTemplate(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"template": templateResponse(tpl)})
}

func (h *Handler) listTemplates(c *gin.Context) {
	items, err := h.service.ListTemplates(c.Request.Context(), c.Query("owner_id"))
	if err != nil {
		ginx.JSONError(c, http.StatusInternalServerError, "prompt.internal.list_templates_failed", "failed to list templates", "internal")
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, templateResponse(item))
	}
	c.JSON(http.StatusOK, gin.H{"templates": out})
}

type setTemplateStatusRequest struct {
	ActorID          string `json:"actor_id"`
	ActorIsSuperuser bool   `json:"actor_is_superuser"`
	ActorCanWrite    bool   `json:"actor_can_write"`
}

func (h *Handler) enableTemplate(c *gin.Context) {
	h.setTemplateStatus(c, domain.TemplateStatusEnabled)
}

func (h *Handler) disableTemplate(c *gin.Context) {
	h.setTemplateStatus(c, domain.TemplateStatusDisabled)
}

func (h *Handler) setTemplateStatus(c *gin.Context, status domain.TemplateStatus) {
	var req setTemplateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "prompt.validation.invalid_payload", "invalid request payload", "validation")
		return
	}
	tpl, err := h.service.SetTemplateStatus(c.Request.Context(), promptapp.SetTemplateStatusInput{
		ActorID:          req.ActorID,
		ActorIsSuperuser: req.ActorIsSuperuser,
		ActorCanWrite:    req.ActorCanWrite,
		TemplateID:       c.Param("id"),
		Status:           status,
	})
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"template": templateResponse(tpl)})
}

type renderRequest struct {
	OwnerID   string         `json:"owner_id"`
	Scene     string         `json:"scene"`
	Variables map[string]any `json:"variables"`
}

func (h *Handler) render(c *gin.Context) {
	var req renderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "prompt.validation.invalid_payload", "invalid request payload", "validation")
		return
	}
	result, err := h.service.Render(c.Request.Context(), promptapp.RenderInput{
		OwnerID:   req.OwnerID,
		Scene:     req.Scene,
		Variables: req.Variables,
	})
	if err != nil {
		if promptapp.IsDomainError(err, domain.ErrRenderValidation) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"issues": result.Issues})
			return
		}
		h.writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"prompt": result.Prompt})
}

func (h *Handler) writeDomainError(c *gin.Context, err error) {
	switch {
	case promptapp.IsDomainError(err, domain.ErrInvalidInput):
		ginx.JSONError(c, http.StatusBadRequest, "prompt.validation.invalid_input", err.Error(), "validation")
	case promptapp.IsDomainError(err, domain.ErrForbidden):
		ginx.JSONError(c, http.StatusForbidden, "prompt.auth.forbidden", err.Error(), "auth")
	case promptapp.IsDomainError(err, domain.ErrTemplateNotFound):
		ginx.JSONError(c, http.StatusNotFound, "prompt.not_found.template", err.Error(), "not_found")
	case promptapp.IsDomainError(err, domain.ErrTemplateNameTaken):
		ginx.JSONError(c, http.StatusConflict, "prompt.conflict.template", err.Error(), "conflict")
	case promptapp.IsDomainError(err, domain.ErrTemplateDisabled):
		ginx.JSONError(c, http.StatusBadRequest, "prompt.validation.template_disabled", err.Error(), "validation")
	default:
		ginx.JSONError(c, http.StatusInternalServerError, "prompt.internal.unexpected_error", "unexpected prompt error", "internal")
	}
}

func templateResponse(tpl domain.SceneTemplate) gin.H {
	return gin.H{
		"id":         tpl.ID,
		"owner_id":   tpl.OwnerID,
		"scene":      tpl.Scene,
		"content":    tpl.Content,
		"variables":  tpl.Variables,
		"status":     tpl.Status,
		"created_at": tpl.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": tpl.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
