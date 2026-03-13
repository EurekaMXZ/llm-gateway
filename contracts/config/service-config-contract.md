# Service Config Contract (M1)

## Scope

This contract defines runtime environment variable conventions shared by all backend services in M1.

## Precedence

1. Process environment variables provided at runtime.
2. Orchestrator-provided values (for example Docker Compose `environment`/`env_file`, Kubernetes ConfigMap/Secret env injection).
3. Code-level defaults in `backend/packages/platform/configx`.

No local file-based config loader is used in M1.

## Common Variables (All Services)

| Variable | Required in M1 | Default | Description |
| --- | --- | --- | --- |
| `APP_ENV` | No | `local` | Runtime environment identifier (`local`, `test`, `staging`, `prod`) |
| `HTTP_ADDR` | No | `:8080` | HTTP bind address |
| `LOG_LEVEL` | No | `info` | Log level (`debug`, `info`, `warn`, `error`) |

## Infrastructure Variables

| Variable | Required in M1 | Default | Applies To |
| --- | --- | --- | --- |
| `POSTGRES_DSN` | No | `postgres://gateway:gateway@postgres:5432/llm_gateway?sslmode=disable` | All services with DB dependency |
| `REDIS_ADDR` | No | `redis:6379` | `ingress`, `identity`, `apikey`, `execution`, `routing`, `prompt` |
| `KAFKA_BROKERS` | No | `kafka:9092` | `ingress`, `execution`, `billing`, `audit` |

## Build-Time Variable (Docker)

| Variable | Required | Default in `.env.example` | Local default in `.env` | Purpose |
| --- | --- | --- | --- | --- |
| `GOPROXY` | No | empty | `https://goproxy.cn,direct` | Go module proxy for Docker image build |

`GOPROXY` is build-time only and does not affect service runtime config.

## Validation Rules

- Unknown environment variables are ignored in M1.
- Missing variables fall back to defaults.
- Strict required-variable validation will be introduced in M2 per service domain.

## References

- `backend/packages/platform/configx/config.go`
- `backend/services/*/configs/service.env.example`
- `docker-compose.yml`
- `.env` and `.env.example`
