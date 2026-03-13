FROM golang:1.26-alpine AS builder
ARG GOPROXY
WORKDIR /workspace
COPY backend /workspace/backend
WORKDIR /workspace/backend/services/identity-service
RUN if [ -n "$GOPROXY" ]; then go env -w GOPROXY="$GOPROXY"; fi && go build -mod=mod -o /out/service ./cmd/identity-service

FROM alpine:3.22
WORKDIR /app
COPY --from=builder /out/service /app/service
EXPOSE 8080
ENTRYPOINT ["/app/service"]
