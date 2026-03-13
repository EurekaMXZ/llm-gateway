package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/packages/platform/ginx"
	"llm-gateway/backend/services/identity-service/internal/app"
	"llm-gateway/backend/services/identity-service/internal/domain"
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
		v1.POST("/auth/bootstrap-superuser", h.bootstrapSuperuser)
		v1.POST("/auth/token", h.issueToken)
		v1.GET("/auth/validate", h.validateToken)

		v1.POST("/users", h.createUser)
		v1.POST("/permissions/check", h.checkPermission)
	}
}

type bootstrapRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

func (h *Handler) bootstrapSuperuser(c *gin.Context) {
	var req bootstrapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "identity.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	u, err := h.service.BootstrapSuperuser(c.Request.Context(), req.Username, req.Password, req.DisplayName)
	if err != nil {
		if app.IsDomainError(err, domain.ErrSuperuserAlreadyExists) {
			ginx.JSONError(c, http.StatusConflict, "identity.conflict.superuser_exists", err.Error(), "conflict")
			return
		}
		ginx.JSONError(c, http.StatusBadRequest, "identity.validation.bootstrap_failed", err.Error(), "validation")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": userResponse(u)})
}

type tokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) issueToken(c *gin.Context) {
	var req tokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "identity.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	token, user, ttl, err := h.service.Authenticate(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if app.IsDomainError(err, domain.ErrInvalidCredentials) {
			ginx.JSONError(c, http.StatusUnauthorized, "identity.auth.invalid_credentials", err.Error(), "auth")
			return
		}
		ginx.JSONError(c, http.StatusInternalServerError, "identity.internal.token_issue_failed", "failed to issue token", "internal")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   int(ttl.Seconds()),
		"user":         userResponse(user),
	})
}

func (h *Handler) validateToken(c *gin.Context) {
	tokenText := bearerToken(c.GetHeader("Authorization"))
	if tokenText == "" {
		ginx.JSONError(c, http.StatusUnauthorized, "identity.auth.missing_token", "missing bearer token", "auth")
		return
	}

	u, err := h.service.ValidateToken(c.Request.Context(), tokenText)
	if err != nil {
		ginx.JSONError(c, http.StatusUnauthorized, "identity.auth.invalid_token", err.Error(), "auth")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userResponse(u)})
}

type createUserRequest struct {
	Username    string      `json:"username"`
	Password    string      `json:"password"`
	DisplayName string      `json:"display_name"`
	Role        domain.Role `json:"role"`
	ParentID    string      `json:"parent_id"`
}

func (h *Handler) createUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "identity.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	tokenText := bearerToken(c.GetHeader("Authorization"))
	if tokenText == "" {
		ginx.JSONError(c, http.StatusUnauthorized, "identity.auth.missing_token", "missing bearer token", "auth")
		return
	}
	actor, err := h.service.ValidateToken(c.Request.Context(), tokenText)
	if err != nil {
		ginx.JSONError(c, http.StatusUnauthorized, "identity.auth.invalid_token", err.Error(), "auth")
		return
	}

	u, err := h.service.CreateUser(c.Request.Context(), app.CreateUserInput{
		ActorID:     actor.ID,
		ActorRole:   actor.Role,
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Role:        req.Role,
		ParentID:    req.ParentID,
	})
	if err != nil {
		switch {
		case app.IsDomainError(err, domain.ErrUsernameTaken):
			ginx.JSONError(c, http.StatusConflict, "identity.conflict.username_taken", err.Error(), "conflict")
		case app.IsDomainError(err, domain.ErrForbidden):
			ginx.JSONError(c, http.StatusForbidden, "identity.permission.create_user_forbidden", err.Error(), "permission")
		case app.IsDomainError(err, domain.ErrInvalidRole), app.IsDomainError(err, domain.ErrInvalidCredentials), app.IsDomainError(err, domain.ErrUserNotFound):
			ginx.JSONError(c, http.StatusBadRequest, "identity.validation.create_user_failed", err.Error(), "validation")
		default:
			ginx.JSONError(c, http.StatusInternalServerError, "identity.internal.create_user_failed", "failed to create user", "internal")
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": userResponse(u)})
}

type checkPermissionRequest struct {
	ActorID         string `json:"actor_id"`
	ResourceOwnerID string `json:"resource_owner_id"`
}

func (h *Handler) checkPermission(c *gin.Context) {
	var req checkPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.JSONError(c, http.StatusBadRequest, "identity.validation.invalid_payload", "invalid request payload", "validation")
		return
	}

	allowed, reason, err := h.service.CheckAccess(c.Request.Context(), req.ActorID, req.ResourceOwnerID)
	if err != nil && !app.IsDomainError(err, domain.ErrUserNotFound) {
		ginx.JSONError(c, http.StatusInternalServerError, "identity.internal.permission_check_failed", "permission check failed", "internal")
		return
	}
	status := http.StatusOK
	if app.IsDomainError(err, domain.ErrUserNotFound) {
		status = http.StatusNotFound
	}
	c.JSON(status, gin.H{
		"allowed": allowed,
		"reason":  reason,
	})
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func userResponse(u domain.User) gin.H {
	return gin.H{
		"id":           u.ID,
		"username":     u.Username,
		"display_name": u.DisplayName,
		"role":         u.Role,
		"parent_id":    u.ParentID,
	}
}
