package main

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/packages/platform/configx"
	"llm-gateway/backend/packages/platform/ginx"
	"llm-gateway/backend/packages/platform/logx"
)

func main() {
	cfg, err := configx.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("failed to load config: " + err.Error() + "\n")
		os.Exit(1)
	}

	logger := logx.New("routing-service", cfg.LogLevel)
	engine := ginx.NewEngine(logger, cfg.Environment)
	engine.GET("/healthz", ginx.HealthHandler("routing-service"))
	engine.GET("/readyz", func(c *gin.Context) {
		logx.WithTrace(logger, c.Request.Context()).Info("readiness check")
		c.JSON(http.StatusOK, gin.H{"ready": true})
	})

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: engine,
	}

	logger.Info("service starting", "addr", cfg.HTTPAddr, "environment", cfg.Environment)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
