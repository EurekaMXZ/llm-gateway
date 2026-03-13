# ADR-20260313-01: M1 Baseline Framework and Config Parser

## Status
- Accepted

## Context
- M1 requires a unified backend HTTP baseline, logging style, and trace propagation across all 8 services.
- M1 also requires a shared configuration loading baseline.
- Pure custom implementations for HTTP middleware stack and env parsing increase maintenance cost and inconsistency risk across services.

## Decision
- Adopt `github.com/gin-gonic/gin` as the HTTP framework baseline for all backend services.
- Adopt `github.com/caarlos0/env/v11` as the shared environment configuration parser.
- Implement reusable wrappers in `backend/packages/platform`:
  - `ginx`: Gin engine bootstrap + trace/log middleware + common handlers
  - `configx`: shared base config loading
  - `logx` + `trace`: structured logging and trace id context handling

## Alternatives Considered
- Option A: Stay with `net/http` + custom middleware only.
  - Rejected due to duplicated boilerplate and weaker consistency for future API middleware growth.
- Option B: Use `viper` for config parsing.
  - Rejected for M1 baseline because requirements are env-first and can be met with lighter dependency surface.

## Consequences
- Positive
  - Faster service bootstrap consistency.
  - Lower middleware duplication.
  - Clear base for auth/rate-limit/observability expansion in M2+.
- Negative
  - Adds third-party dependency management burden.
  - Requires dependency download in constrained environments.
