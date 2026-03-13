package ginx

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"llm-gateway/backend/packages/platform/logx"
	"llm-gateway/backend/packages/platform/trace"
)

func NewEngine(logger *slog.Logger, environment string) *gin.Engine {
	if environment == "local" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(TraceMiddleware())
	engine.Use(RequestLogMiddleware(logger))
	return engine
}

func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := trace.FromHeader(c.GetHeader(trace.HeaderTraceID))
		ctx := trace.WithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)
		c.Writer.Header().Set(trace.HeaderTraceID, traceID)
		c.Next()
	}
}

func RequestLogMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		logx.WithTrace(logger, c.Request.Context()).Info("request completed",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
		)
	}
}

func JSONError(c *gin.Context, status int, code string, message string, errorType string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
			"type":    errorType,
		},
		"trace_id": trace.FromContext(c.Request.Context()),
	})
}

func HealthHandler(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": service,
		})
	}
}
