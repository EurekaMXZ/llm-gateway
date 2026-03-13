# Current Status

## Date
- 2026-03-13

## Active Milestone
- M2: Identity and resource control plane (M1 completed)

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

## In Progress
- M2 bootstrap: control-plane API skeletons for identity and API key management.

## Blockers
- None

## Next Actions
1. Create identity-service API skeletons for auth, role checks, and subtree access validation.
2. Create apikey-service API skeletons for key lifecycle and model whitelist queries.
3. Define M2 permission test matrix and add initial integration tests.
