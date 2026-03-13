# Local Compose Runbook

## Scope

This runbook defines one-command local bootstrap for M1 baseline services.

## Prerequisites

1. Docker with `docker compose` support.
2. Go 1.26+ for local service testing.
3. Configure root `.env` for Go module proxy used in Docker build.
4. Keep `.env.example` as template with empty `GOPROXY` default.
5. Infrastructure host ports are configurable via `.env` (`POSTGRES_HOST_PORT`, `REDIS_HOST_PORT`, `ZOOKEEPER_HOST_PORT`, `KAFKA_HOST_PORT`).

## Commands

1. Start infrastructure + all services:
- `make compose-up`

2. Check running containers:
- `make compose-ps`

3. Stream logs:
- `make compose-logs`

4. Stop all containers:
- `make compose-down`

## Health Endpoints

- Ingress: `http://localhost:18080/healthz`
- Identity: `http://localhost:18081/healthz`
- API Key: `http://localhost:18082/healthz`
- Execution: `http://localhost:18083/healthz`
- Routing: `http://localhost:18084/healthz`
- Prompt: `http://localhost:18085/healthz`
- Billing: `http://localhost:18086/healthz`
- Audit: `http://localhost:18087/healthz`

## Notes

1. If command execution fails due sandbox cache permissions, request escalation and rerun.
2. Do not route tool caches into project-local directories to bypass permissions.
3. Current default local value is `GOPROXY=https://goproxy.cn,direct` in `.env`.
4. Default mapped infra ports are `15432/16379/12181/19092` to reduce collision with local host services.
