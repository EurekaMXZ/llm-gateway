package httpx

import (
	"net/http"

	"llm-gateway/backend/packages/platform/trace"
)

func TraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := trace.FromHeader(r.Header.Get(trace.HeaderTraceID))
		ctx := trace.WithTraceID(r.Context(), traceID)
		w.Header().Set(trace.HeaderTraceID, traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
