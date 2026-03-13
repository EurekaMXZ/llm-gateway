package main

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/packages/platform/configx"
	"llm-gateway/backend/packages/platform/ginx"
	"llm-gateway/backend/packages/platform/logx"
	"llm-gateway/backend/services/ingress-service/internal/controlplane"
	ingresshttp "llm-gateway/backend/services/ingress-service/internal/interfaces/http"
)

func main() {
	cfg, err := configx.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("failed to load config: " + err.Error() + "\n")
		os.Exit(1)
	}

	identityURL := getenv("IDENTITY_SERVICE_BASE_URL", "http://identity-service:8080")
	apikeyURL := getenv("APIKEY_SERVICE_BASE_URL", "http://apikey-service:8080")

	controlService := controlplane.NewService(identityURL, apikeyURL, nil)
	logger := logx.New("ingress-service", cfg.LogLevel)
	engine := ginx.NewEngine(logger, cfg.Environment)
	engine.GET("/healthz", ginx.HealthHandler("ingress-service"))
	engine.GET("/readyz", func(c *gin.Context) {
		logx.WithTrace(logger, c.Request.Context()).Info("readiness check")
		c.JSON(http.StatusOK, gin.H{"ready": true})
	})
	ingresshttp.NewHandler(controlService).RegisterRoutes(engine)

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: engine,
	}

	logger.Info("service starting", "addr", cfg.HTTPAddr, "environment", cfg.Environment, "identity_url", identityURL, "apikey_url", apikeyURL)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func getenv(key string, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
