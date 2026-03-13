package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const HeaderTraceID = "X-Trace-Id"

type contextKey string

const traceIDKey contextKey = "trace_id"

func NewTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return "trace-id-unavailable"
}

func FromHeader(raw string) string {
	id := strings.TrimSpace(raw)
	if id == "" {
		return NewTraceID()
	}
	return id
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

func FromContext(ctx context.Context) string {
	v, _ := ctx.Value(traceIDKey).(string)
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return v
}
