# Local Compose Runbook

## Scope

This runbook defines one-command local bootstrap for M1/M2 baseline services.

## Prerequisites

1. Docker with `docker compose` support.
2. Go 1.26+ for local service testing.
3. Configure root `.env` for Go module proxy used in Docker build.
4. Keep `.env.example` as template with empty `GOPROXY` default.
5. Infrastructure host ports are configurable via `.env` (`POSTGRES_HOST_PORT`, `REDIS_HOST_PORT`, `ZOOKEEPER_HOST_PORT`, `KAFKA_HOST_PORT`).
6. `curl` and `jq` for smoke scripts.

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

## M2 Control-Plane Validation Smoke

1. Bootstrap superuser and get token from identity:
- `curl -sS -X POST http://localhost:18081/v1/auth/bootstrap-superuser -H 'Content-Type: application/json' -d '{"username":"root","password":"pass","display_name":"Root"}'`
- `TOKEN=$(curl -sS -X POST http://localhost:18081/v1/auth/token -H 'Content-Type: application/json' -d '{"username":"root","password":"pass"}' | jq -r '.access_token')`

2. Create API key bound to same owner (`owner_id` should be identity `user.id`):
- `curl -sS -X POST http://localhost:18082/v1/keys -H 'Content-Type: application/json' -d '{"owner_id":"<identity_user_id>","name":"dev-key","allowed_models":["gpt-4o-mini"]}'`

3. Validate via ingress aggregated endpoint:
- `curl -sS -X POST http://localhost:18080/v1/control/validate -H "Authorization: Bearer ${TOKEN}" -H 'Content-Type: application/json' -d '{"api_key":"<plaintext_api_key>","model":"gpt-4o-mini"}'`

## M2 Closeout Smoke Script

Run one command to validate the current M2 management/control-plane baseline:
- `make m2-smoke`

Script:
- `scripts/agent/m2_closeout_smoke.sh`

Covered checks:
- Health checks for ingress/identity/apikey/execution/routing/prompt/billing
- Identity bootstrap + token
- API key create + ingress control-plane validate
- Execution provider/model create
- Routing policy create + resolve
- Prompt template create + render
- Billing price set + wallet topup/deduct/balance

If a service exited during initial dependency warm-up, rerunning `make m2-smoke` is safe; the script will attempt to bring core services up before validation.

## M3 Data-Plane Quick Check

After M2 control-plane setup (token + API key + provider/model + optional route/template), verify ingress data-plane path:

1. Call ingress chat completions:
- `curl -sS -X POST http://localhost:18080/v1/chat/completions -H "Authorization: Bearer ${TOKEN}" -H "X-API-Key: <plaintext_api_key>" -H 'Content-Type: application/json' -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'`

2. Expected baseline:
- HTTP `200` for allowed request.
- If route policy matches, target model/provider is rewritten before execution.
- If no route policy matches, ingress falls back to direct execution with requested model.

## M3 Closeout Smoke Script

Run one command to validate M3 data-plane baseline (matched rewrite + no-policy direct execution):
- `make m3-smoke`

Script:
- `scripts/agent/m3_closeout_smoke.sh`

The script uses Compose service `mock-openai` (`kennethreitz/httpbin`) as deterministic upstream mock.

## Notes

1. If command execution fails due sandbox cache permissions, request escalation and rerun.
2. Do not route tool caches into project-local directories to bypass permissions.
3. Current default local value is `GOPROXY=https://goproxy.cn,direct` in `.env`.
4. Default mapped infra ports are `15432/16379/12181/19092` to reduce collision with local host services.
