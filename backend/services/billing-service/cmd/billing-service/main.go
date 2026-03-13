package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"llm-gateway/backend/packages/platform/configx"
	"llm-gateway/backend/packages/platform/ginx"
	"llm-gateway/backend/packages/platform/logx"
	billingapp "llm-gateway/backend/services/billing-service/internal/app"
	billingpg "llm-gateway/backend/services/billing-service/internal/infra/postgres"
	billinghttp "llm-gateway/backend/services/billing-service/internal/interfaces/http"
)

func main() {
	cfg, err := configx.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("failed to load config: " + err.Error() + "\n")
		os.Exit(1)
	}

	logger := logx.New("billing-service", cfg.LogLevel)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		logger.Error("failed to create postgres pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("postgres ping failed", "error", err)
		os.Exit(1)
	}

	repo := billingpg.NewRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		logger.Error("failed to ensure billing schema", "error", err)
		os.Exit(1)
	}

	svc := billingapp.NewServiceWithRepository(repo)
	engine := ginx.NewEngine(logger, cfg.Environment)
	engine.GET("/healthz", ginx.HealthHandler("billing-service"))
	engine.GET("/readyz", func(c *gin.Context) {
		rCtx, rCancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer rCancel()
		if err := pool.Ping(rCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"ready": false, "reason": "postgres_unavailable"})
			return
		}
		logx.WithTrace(logger, c.Request.Context()).Info("readiness check")
		c.JSON(http.StatusOK, gin.H{"ready": true})
	})
	billinghttp.NewHandler(svc).RegisterRoutes(engine)

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
