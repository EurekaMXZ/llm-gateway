# Current Status

## Date
- 2026-03-14

## Active Milestone
- M4: Event-driven audit and billing loop (M3 closeout completed on 2026-03-14)

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
- Updated governance docs: added mandatory `AGENTS.md` rule requiring synchronization among `AGENTS.md`, `docs/runbooks/*.md`, and `scripts/agent/*.sh`.
- Implemented M3 ingress OpenAI data-plane endpoint `POST /v1/chat/completions` with full orchestration path: `Auth -> Whitelist -> Template -> Routing -> Execution -> Response`.
- Added ingress data-plane service integration with `prompt-service`, `routing-service`, and `execution-service`, including no-policy direct execution fallback.
- Added deterministic ingress difficulty scoring (user-message based) and passed score into routing resolve API.
- Added execution-service M3 APIs: provider priority management and `POST /v1/execute/chat/completions`.
- Updated routing resolve semantics to return `200 matched=false` on no-policy match and support model+difficulty inputs.
- Added/updated unit tests for ingress dataplane, routing resolve semantics, and execution selection/validation paths.
- Added/updated OpenAPI contracts for ingress/execution/routing M3 behavior.
- Revalidated root backend tests after M3 implementation (`make test-go` PASS).
- Added M3 compose-level smoke script (`scripts/agent/m3_closeout_smoke.sh`) and Makefile target (`make m3-smoke`).
- Added `mock-openai` compose dependency (`kennethreitz/httpbin`) for deterministic upstream echo verification in local M3 E2E.
- Implemented ingress `/v1/responses` compatibility endpoint via chat-pipeline adaptation.
- Verified M3 closeout smoke end-to-end (`make m3-smoke` PASS), including policy-matched rewrite and no-policy direct execution paths.
- Fixed review P1: `/v1/responses` now normalizes structured `input_text` parts to chat-compatible content parts before execution forwarding.
- Fixed review P2: routing resolve now treats legacy invalid policy conditions as non-matching and continues evaluation (preserves fallback behavior).
- Fixed review P2: Postgres execution target resolution now distinguishes disabled `provider_id` as `provider_disabled` instead of `model_not_found`.
- Fixed review P2: denied data-plane calls (`/v1/chat/completions`, `/v1/responses`) now return standard error envelope instead of raw decision payload.
- Revalidated full backend test suite after fixes (`make test-go` PASS).

## In Progress
- M4 bootstrap planning and event contract implementation preparation.

## Blockers
- None

## Next Actions
1. Implement execution completion event producer in `execution-service` (`execution.invocation.completed`), including trace/idempotency metadata.
2. Implement `billing-service` consumer and idempotent usage persistence for `execution.invocation.completed`.
3. Implement `audit-service` consumer and invocation index persistence for `execution.invocation.completed`.
