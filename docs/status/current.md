# Current Status

## Date
- 2026-03-14

## Active Milestone
- M3: Data plane main flow (M2 closeout completed on 2026-03-13)

## Completed
- Rewrote `AGENTS.md` with executable architecture and process constraints.
- Created governance docs, runbooks, state file, schema, and PR checklist baseline.
- Bootstrapped 8 backend service modules via `go mod init`.
- Added baseline service layers (`cmd/internal/migrations/configs`) and health endpoint entrypoints.
- Added shared backend platform package for trace middleware and structured logging.
- Added shared env-based config loader package (`configx`) and service config templates.
- Migrated all 8 services from temporary `net/http` entrypoints to Gin baseline.
- Bootstrapped `frontend/apps/admin` via `pnpm create vue`.
- Created `frontend/packages/sdk` skeleton package.
- Added root `docker-compose.yml` with Postgres, Redis, Kafka, and 8 service containers.
- Added local compose operation runbook and root `Makefile` bootstrap targets.
- Added ADR for M1 framework/config dependency decision (`gin` + `caarlos0/env`).
- Updated Docker build strategy to source `GOPROXY` from root `.env` and left `.env.example` empty.
- Reverted Dockerfile fixed image digest pinning and validated `routing-service` docker build success with `GOPROXY=https://goproxy.cn,direct`.
- Completed full `docker compose up --build` smoke test and verified all 8 services return healthy responses on `/healthz`.
- Published service-level configuration contract in `contracts/config/service-config-contract.md`.
- Fixed review issue: `make test-go` now fails fast on first module test failure.
- Fixed review issue: Kafka now uses dual advertised listeners to support both Compose-internal and host-side clients.
- Implemented M2 identity-service APIs (`bootstrap superuser`, `token`, `token validate`, `permission check`, `create user`) with JWT and hierarchy checks.
- Implemented M2 apikey-service APIs (`create/get/enable/disable/validate`) with lifecycle and model whitelist validation.
- Implemented M2 execution-service provider/model catalog management APIs (owner constraints + Postgres persistence + tests).
- Implemented M2 routing-service policy management and resolve APIs (owner constraints + Postgres persistence + tests).
- Implemented M2 prompt-service template/variable management and render validation APIs (422 structured issues + Postgres persistence + tests).
- Implemented M2 billing-service pricing/wallet/transaction APIs (owner constraints + balance checks + Postgres persistence + tests).
- Completed M2 persistence wiring for identity/apikey/execution/routing/prompt/billing with startup schema ensure and readiness DB checks.
- Completed ingress control-plane integration: `/v1/control/validate` calling identity + apikey validation and checking key-owner consistency.
- Added OpenAPI contracts for `identity/apikey/execution/routing/prompt/billing`.
- Added executable M2 closeout smoke script `scripts/agent/m2_closeout_smoke.sh` and Makefile target `m2-smoke`.
- Verified compose-level M2 closeout smoke (`make m2-smoke`) PASS against running stack.
- Fixed review P1: `/v1/users` now requires bearer token auth and enforces caller-based create-user permission scope.
- Fixed review P1: management write checks in execution/routing/prompt/billing now accept explicit subtree write authorization (`actor_can_write`) instead of superuser/self only.
- Fixed review P2: apikey validation now rejects model-scoped keys when request model is empty (`reason=model_required`).
- Fixed review P2: superuser uniqueness now enforced atomically through repository constraints (`uq_users_single_superuser`) plus in-memory lock-time guard.
- Revalidated affected services and root Go test aggregation after fixes (`make test-go` PASS).

## In Progress
- M3 planning and ingress main data-plane orchestration bootstrap.

## Blockers
- None

## Next Actions
1. Build M3 ingress orchestration pipeline skeleton (`Auth -> Whitelist -> Template -> Routing -> Execution -> Response`).
2. Define/implement upstream execution adapter invocation path for OpenAI-compatible flows.
3. Add end-to-end test path for explicit concrete model bypass vs custom-model policy routing behavior.
