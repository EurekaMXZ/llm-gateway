package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/packages/platform/ginx"
	"llm-gateway/backend/services/routing-service/internal/app"
	"llm-gateway/backend/services/routing-service/internal/domain"
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
		v1.POST("/policies", h.createPolicy)
		v1.GET("/policies", h.listPolicies)
		v1.GET("/policies/:id", h.getPolicy)
		v1.POST("/policies/:id/enable", h.enablePolicy)
		v1.POST("/policies/:id/disable", h.disablePolicy)
		v1.POST("/policies/resolve", h.resolve)
	}
}

type createPolicyRequest struct {
	ActorID          string `json:"actor_id"`
	ActorIsSuperuser bool   `json:"actor_is_superuser"`
	ActorCanWrite    bool   `json:"actor_can_write"`
	OwnerID          string `json:"owner_id"`
	CustomModel      string `json:"custom_model"`
	TargetProviderID string `json:"target_provider_id"`
	TargetModel      string `json:"target_model"`
	Priority         int    `json:"priority"`
	ConditionJSON    string `json:"condition_json"`
}

func (h *Handler) createPolicy(c *gin.Context) {
	var req createPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "routing.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	policy, err := h.service.CreatePolicy(c.Request.Context(), app.CreatePolicyInput{
		ActorID:          req.ActorID,
		ActorIsSuperuser: req.ActorIsSuperuser,
		ActorCanWrite:    req.ActorCanWrite,
		OwnerID:          req.OwnerID,
		CustomModel:      req.CustomModel,
		TargetProviderID: req.TargetProviderID,
		TargetModel:      req.TargetModel,
		Priority:         req.Priority,
		ConditionJSON:    req.ConditionJSON,
	})
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"policy": policyResponse(policy)})
}

func (h *Handler) getPolicy(c *gin.Context) {
	policy, err := h.service.GetPolicy(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"policy": policyResponse(policy)})
}

func (h *Handler) listPolicies(c *gin.Context) {
	policies, err := h.service.ListPolicies(c.Request.Context(), c.Query("owner_id"), c.Query("custom_model"))
	if err != nil {
		ginx.JSONError(c, http.StatusInternalServerError, "routing.internal.list_policies_failed", "failed to list policies", "internal")
		return
	}
	items := make([]gin.H, 0, len(policies))
	for _, policy := range policies {
		items = append(items, policyResponse(policy))
	}
	c.JSON(http.StatusOK, gin.H{"policies": items})
}

type setPolicyStatusRequest struct {
	ActorID          string `json:"actor_id"`
	ActorIsSuperuser bool   `json:"actor_is_superuser"`
	ActorCanWrite    bool   `json:"actor_can_write"`
}

func (h *Handler) enablePolicy(c *gin.Context) {
	h.setPolicyStatus(c, domain.PolicyStatusEnabled)
}

func (h *Handler) disablePolicy(c *gin.Context) {
	h.setPolicyStatus(c, domain.PolicyStatusDisabled)
}

func (h *Handler) setPolicyStatus(c *gin.Context, status domain.PolicyStatus) {
	var req setPolicyStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "routing.validation.invalid_payload", "invalid request payload", "validation")
		return
	}
	policy, err := h.service.SetPolicyStatus(c.Request.Context(), app.SetPolicyStatusInput{
		ActorID:          req.ActorID,
		ActorIsSuperuser: req.ActorIsSuperuser,
		ActorCanWrite:    req.ActorCanWrite,
		PolicyID:         c.Param("id"),
		Status:           status,
	})
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"policy": policyResponse(policy)})
}

type resolveRequest struct {
	OwnerID         string `json:"owner_id"`
	Model           string `json:"model"`
	CustomModel     string `json:"custom_model"`
	DifficultyScore int    `json:"difficulty_score"`
}

func (h *Handler) resolve(c *gin.Context) {
	var req resolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "routing.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	result, err := h.service.Resolve(c.Request.Context(), app.ResolveInput{
		OwnerID:         req.OwnerID,
		Model:           req.Model,
		CustomModel:     req.CustomModel,
		DifficultyScore: req.DifficultyScore,
	})
	if err != nil {
		h.writeDomainError(c, err)
		return
	}
	if result.Matched {
		c.JSON(http.StatusOK, gin.H{"matched": true, "reason": result.Reason, "policy": policyResponse(result.Policy)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"matched": false, "reason": result.Reason})
}

func (h *Handler) writeDomainError(c *gin.Context, err error) {
	switch {
	case app.IsDomainError(err, domain.ErrInvalidInput):
		ginx.JSONError(c, http.StatusBadRequest, "routing.validation.invalid_input", err.Error(), "validation")
	case app.IsDomainError(err, domain.ErrForbidden):
		ginx.JSONError(c, http.StatusForbidden, "routing.auth.forbidden", err.Error(), "auth")
	case app.IsDomainError(err, domain.ErrPolicyNotFound):
		ginx.JSONError(c, http.StatusNotFound, "routing.not_found.policy", err.Error(), "not_found")
	case app.IsDomainError(err, domain.ErrPolicyNameTaken):
		ginx.JSONError(c, http.StatusConflict, "routing.conflict.policy", err.Error(), "conflict")
	default:
		ginx.JSONError(c, http.StatusInternalServerError, "routing.internal.unexpected_error", "unexpected routing error", "internal")
	}
}

func policyResponse(policy domain.Policy) gin.H {
	return gin.H{
		"id":                 policy.ID,
		"owner_id":           policy.OwnerID,
		"custom_model":       policy.CustomModel,
		"target_provider_id": policy.TargetProviderID,
		"target_model":       policy.TargetModel,
		"priority":           policy.Priority,
		"condition_json":     policy.ConditionJSON,
		"status":             policy.Status,
		"created_at":         policy.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":         policy.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
