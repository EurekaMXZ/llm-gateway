FROM golang:1.26-alpine AS builder
ARG GOPROXY
WORKDIR /workspace
COPY backend /workspace/backend
WORKDIR /workspace/backend/services/ingress-service
RUN if [ -n "$GOPROXY" ]; then go env -w GOPROXY="$GOPROXY"; fi && go build -mod=mod -o /out/service ./cmd/ingress-service

FROM alpine:3.22
WORKDIR /app
COPY --from=builder /out/service /app/service
EXPOSE 8080
ENTRYPOINT ["/app/service"]
