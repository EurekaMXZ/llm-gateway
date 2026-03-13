package logx

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"llm-gateway/backend/packages/platform/trace"
)

func New(service string, level string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(level)})
	return slog.New(handler).With("service", service)
}

func WithTrace(logger *slog.Logger, ctx context.Context) *slog.Logger {
	traceID := trace.FromContext(ctx)
	if traceID == "" {
		return logger
	}
	return logger.With("trace_id", traceID)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
