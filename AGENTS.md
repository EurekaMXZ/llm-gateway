# AGENTS

This file defines executable working rules for AI coding agents in `llm-gateway`.
Any item marked with `MUST` is a mandatory rule.

## 1. Normative Language

- `MUST`: mandatory for every session and PR.
- `SHOULD`: strong recommendation; deviation requires explicit note in status or PR.
- `MAY`: optional.

## 2. Project Objectives and Scope

- Project: `llm-gateway`
- V1 objective: Deliver a basic LLM API Gateway with protocol adaptation, policy routing, prompt templating, audit logging, and billing.
- V1 protocol boundary: OpenAI-compatible downstream and OpenAI-compatible upstream only.
- Delivery order: Docker Compose first, Kubernetes second.

### 2.1 V1 In-Scope

- OpenAI-compatible data plane ingress endpoints.
- Identity, API key lifecycle, routing, prompt templating, execution, billing, and audit core loops.
- Event-driven billing and audit persistence from execution outputs.

### 2.2 V1 Out-of-Scope

- Non-OpenAI downstream APIs.
- Non-OpenAI upstream providers.
- Hard multi-cluster production orchestration features before M5.

## 3. Architecture Baseline

### 3.1 Services and Responsibilities

#### ingress-service (control-plane)

- Sole public data-plane entrypoint.
- Exposes OpenAI-compatible APIs (at minimum `/v1/chat/completions`, `/v1/responses`).
- Orchestrates authentication, API-key checks, routing, prompt rendering, execution, and response adaptation.
- MUST NOT persist core domain state except transient request-scoped data.

#### identity-service

- Owns authentication, authorization, role hierarchy, and resource access checks.
- Role model: one `superuser`, `administrator`, `regular_user`.

#### apikey-service

- Owns downstream API key lifecycle, attribution, status, and model whitelist.
- Provides key validity and whitelist query interface for ingress-service.

#### execution-service

- Owns upstream provider configs and model catalog.
- Executes upstream calls and returns normalized result + usage metrics.
- Emits execution completion events.

#### routing-service

- Owns downstream-model routing policy engine rules.
- Evaluates routing policies for any downstream model name.
- Only matched policies rewrite target provider/model; when no policy matches, request MUST fall back to direct upstream model execution.

#### prompt-service

- Owns scene templates and variable schema.
- Renders prompt snippets from `scene + variables`.
- Returns structured render errors.

#### billing-service

- Owns pricing, usage records, wallet ledger, and bill aggregation.
- Bills by actual executed upstream `provider/model/token`.

#### audit-service

- Owns sensitive operation audit logs and invocation index records.
- Supports query by user, API key, and time range.

### 3.2 Cross-Service Invariants (MUST)

- Billing basis MUST always be actual upstream `provider/model/token`, never downstream alias.
- Routing policy evaluation MUST support any downstream model name; no policy match MUST fall back to direct upstream execution.
- Prompt injection MUST be controlled by downstream `scene + variables`.
- End-to-end `trace_id` MUST propagate across sync APIs and async events.

## 4. Permission and Resource Access Model

### 4.1 Role Rules (MUST)

- `superuser`: globally unique, full read/write access.
- `administrator`: manage owned subtree users/resources; no peer-admin takeover.
- `regular_user`: access own resources only.
- Peer users MUST NOT access each other's resources.

### 4.2 Resource Matrix (MUST)

| Resource | superuser | administrator | regular_user |
| --- | --- | --- | --- |
| Users | full | subtree CRUD | self read/update basic profile |
| API keys | full | subtree CRUD | own CRUD |
| Provider configs | full | subtree read (write only if delegated) | none |
| Model catalog | full | subtree read | read allowed models only |
| Custom models/policies | full | subtree CRUD | own read |
| Prompt templates | full | subtree CRUD | own CRUD |
| Pricing | full | read | none |
| Wallet | full | subtree read/adjust | own read/topup if enabled |
| Audit logs | full | subtree read | own read (non-sensitive fields) |
| Invocation logs | full | subtree read | own read |

If a row needs exception, it MUST be recorded in ADR and reflected in service-level contract docs.

## 5. API, Error, and Contract Standards

### 5.1 Downstream API Baseline (MUST)

- `ingress-service` MUST keep OpenAI-compatible request/response structure for supported endpoints.
- Unsupported OpenAI fields MAY be ignored only if behavior is documented.
- Protocol adaptation logic MUST be centralized in ingress-service.

### 5.2 Error Envelope (MUST)

All synchronous API errors MUST include:

- `error.code`: stable machine code (see 5.3).
- `error.message`: human-readable summary.
- `error.type`: category (`auth`, `permission`, `validation`, `upstream`, `rate_limit`, `internal`).
- `trace_id`: request trace id.

### 5.3 Error Code Convention (MUST)

- Format: `<service>.<category>.<name>`
- Example: `identity.auth.invalid_token`, `routing.policy.no_match`, `execution.upstream.timeout`
- Categories: `auth`, `permission`, `validation`, `not_found`, `conflict`, `rate_limit`, `dependency`, `internal`.
- HTTP mapping baseline:
  - `auth` -> `401`
  - `permission` -> `403`
  - `validation` -> `400`
  - `not_found` -> `404`
  - `conflict` -> `409`
  - `rate_limit` -> `429`
  - `dependency` -> `502/503`
  - `internal` -> `500`

### 5.4 Contract Versioning (MUST)

- Contract changes MUST be backward-compatible within minor version.
- Breaking changes MUST increment version and include migration note in ADR.
- `docs/state/project_state.json` MUST track current API and event contract versions.

## 6. Data and Event Boundaries

### 6.1 Database Strategy (MUST)

- One PostgreSQL cluster, multi-schema isolation by service.
- Schema naming SHOULD follow `svc_<service>`.
- Cross-service direct writes are forbidden.
- Cross-service reads SHOULD go through service API/events; direct DB reads require ADR.

### 6.2 Persistence Baseline (MUST)

Tables storing mutable business data MUST include:

- `id`
- `created_at`
- `updated_at`
- `created_by` (when user initiated)
- `updated_by` (when user initiated)

Sensitive fields (password hash, API keys, provider secrets) MUST NOT be logged in plaintext.

### 6.3 Event Topics (V1)

- `execution.invocation.completed`
- `audit.operation.recorded`
- `billing.usage.recorded`

### 6.4 Event Semantics (MUST)

Each event MUST include metadata:

- `event_id` (globally unique)
- `event_type`
- `event_version`
- `occurred_at` (RFC3339 UTC)
- `trace_id`
- `producer`
- `idempotency_key`

Delivery semantics:

- Producers/consumers assume at-least-once delivery.
- Consumers MUST implement idempotent processing by `idempotency_key` or `event_id`.
- Failed consumption MUST use retry + dead-letter strategy with bounded retries.

## 7. Mandatory Engineering Standards

### 7.1 External Dependencies First (MUST)

- Prefer mature external libraries before custom implementation.
- For critical paths, Build-vs-Buy rationale MUST be recorded in ADR or PR description.

### 7.2 Scaffold-First Module Initialization (MUST)

- Backend submodules: initialize via `go mod init`.
- Frontend submodules: initialize via `pnpm create vue` (or official equivalent).
- Do not fake scaffolds by manually creating core bootstrap files.

### 7.3 Coding and Layout Baseline (MUST)

- Backend: Go + Gin + Postgres + Kafka + Redis.
- Frontend: Vue3 + TypeScript + Element Plus + UnoCSS + Pinia + Axios.
- Recommended backend layout for each service:
  - `cmd/<service>/main.go`
  - `internal/app`
  - `internal/domain`
  - `internal/infra`
  - `internal/interfaces`
  - `migrations`
  - `configs`

### 7.4 Governance Model

- These rules are document-level governance and review criteria.
- Maintainers make final integration decisions.
- Rules are not CI hard-blockers by default unless maintainers promote specific checks.

### 7.5 Sandbox Permission and Cache Policy (MUST)

- If an agent command fails due to sandbox permission limits, the agent MUST request permission escalation and rerun the original command.
- Agents MUST NOT bypass sandbox limits by changing cache paths into the repository workspace.
- It is strictly forbidden to configure project-local cache directories for tooling caches such as `pnpm-store`, `GOCACHE`, `GOMODCACHE`, `GOPATH/pkg/mod`, `npm` cache, or similar runtime/build caches for the purpose of bypassing permission constraints.
- If escalation is used, the session notes SHOULD briefly record which command required escalation and why.

### 7.6 Documentation and Script Consistency (MUST)

- `AGENTS.md`, `docs/runbooks/*.md`, and `scripts/agent/*.sh` MUST describe the same operational policy for session lifecycle, sandbox escalation, and cache-path restrictions.
- If any rule changes in one of the above locations, the corresponding documents/scripts MUST be updated in the same change.
- Helper scripts MUST remain executable and reflect current mandatory fields/steps from this specification.

## 8. Session Memory and State Synchronization (MANDATORY)

### 8.1 Session Start Procedure (MUST)

Read in strict order:

1. `AGENTS.md`
2. `docs/status/current.md`
3. `docs/state/project_state.json`
4. Latest accepted ADR in `docs/decisions/`

Recommended helper command:

- `scripts/agent/session_start.sh`

### 8.2 How to Determine "Latest ADR" (MUST)

- Ignore `ADR-template.md`.
- Candidate files MUST match `ADR-*.md`.
- Prefer ADR with status `Accepted`.
- If multiple accepted ADRs exist, pick latest by ADR filename date sequence; tie-breaker is latest git commit time.

If no accepted ADR exists:

- Session MAY proceed.
- First architecture-impacting change in that session MUST create an ADR and mark status.

### 8.3 Session End Procedure (MUST)

Must update:

1. `docs/status/current.md`
2. `docs/status/history/YYYY-MM-DD.md`
3. `docs/state/project_state.json`

Required minimum fields to refresh in `project_state.json`:

- `next_actions`
- `last_verified_commit`
- `updated_at`

Recommended helper command:

- `scripts/agent/session_close.sh`

### 8.4 `project_state.json` Required Schema (MUST)

`project_state.json` MUST remain valid JSON and include at least:

- `project_version`: string
- `current_milestone`: enum `M0|M1|M2|M3|M4|M5`
- `services`: string[]
- `api_contract_versions`: object
- `event_contract_versions`: object
- `open_risks`: string[]
- `next_actions`: string[]
- `last_verified_commit`: string (`UNCOMMITTED` or git SHA)
- `updated_at`: RFC3339 UTC timestamp

### 8.5 Session History Entry Template (MUST)

Each `docs/status/history/YYYY-MM-DD.md` entry MUST contain:

- `Summary`
- `Decisions`
- `Validation` (commands/tests run and result)
- `Changed Files` (key paths)
- `Next`

### 8.6 Concurrency and Conflict Handling (MUST)

When multiple agents/sessions touch status files:

- Rebase before final write.
- Keep factual history; do not delete prior entries.
- If `current.md` conflicts, preserve latest factual state and move older notes into history snapshot.
- If `project_state.json` conflicts, keep newest `updated_at` and merge `next_actions` without dropping still-open actions.

### 8.7 Session Close Acceptance Gate (MUST)

Before ending a coding session, verify:

- Code/contracts/docs consistency checked.
- Status files synchronized.
- New assumptions/trade-offs recorded in ADR or status notes.
- `last_verified_commit` reflects current verified commit or `UNCOMMITTED`.

## 9. Directory Planning Baseline

- `contracts/`: sync API and async event contracts.
- `backend/services/`: 8 microservices.
- `backend/packages/`: shared backend packages.
- `frontend/apps/`: frontend apps (including admin).
- `frontend/packages/`: frontend shared packages and SDK.
- `docs/`: specs, ADRs, status, runbooks.
- `scripts/agent/`: session bootstrap/check/close helpers.

## 10. Milestones (Execution Order)

### M0 Documentation and Decision Baseline

Deliverables:

- Service boundaries finalized.
- Permission matrix finalized.
- Event topics and error code standards finalized.
- `current.md`, history snapshots, and state file operational.

Verification:

- Documentation internally consistent.
- At least one accepted ADR exists or explicit note explains no ADR yet.

### M1 Infrastructure and Engineering Skeleton

Deliverables:

- 8 service code skeletons.
- Unified configuration and logging baseline.
- Trace ID propagation skeleton.
- Docker Compose bootstrapping Postgres/Kafka/Redis + services.

Verification:

- Service module init commands recorded (`go mod init`, scaffold logs).
- Compose starts core dependencies and services for local debugging.

### M2 Identity and Resource Control Plane

Deliverables:

- Superuser bootstrap flow.
- 3-tier permission system.
- Isolation enforcement and resource management APIs.

Verification:

- Permission checks covered by tests.
- Unauthorized cross-scope access denied by design and tests.

### M3 Data Plane Main Flow

Deliverables:

- End-to-end ingress pipeline: auth -> whitelist -> template -> routing -> execution -> response.
- Routing semantics: matched policy rewrites target provider/model; no policy match performs direct upstream execution for requested model.

Verification:

- Integration tests cover policy-matched rewrite path and no-policy direct-execution path.

### M4 Event-Driven Audit and Billing Loop

Deliverables:

- Kafka event production/consumption loop.
- Billing persistence and wallet deduction flow.
- Audit persistence and queryable indices.

Verification:

- Idempotent consume behavior verified.
- Reprocessing same event does not duplicate bill/audit records.

### M5 Stability and Deployment

Deliverables:

- Rate limiting, circuit breaker, retries, idempotency, replay protection.
- Load test reports.
- Kubernetes manifests + operation runbooks.

Verification:

- Stability controls validated under load and failure injection.
- Deployment docs are runnable by maintainers.

## 11. Definition of Done (DoD)

A milestone/task is done only when all are true:

- Code, contracts, and docs are consistent.
- Session memory files are synchronized.
- Key assumptions/trade-offs are recorded in ADR or PR notes.
- Maintainers can review and run the change independently.

## 12. Recommended Working Checklist for Agents

Before coding:

1. Run `scripts/agent/session_start.sh`.
2. Confirm active milestone and `next_actions`.
3. Identify whether task changes architecture/contracts (if yes, prepare ADR update).

During coding:

1. Keep service boundaries intact.
2. Preserve traceability (`trace_id`, audit fields, event metadata).
3. Record Build-vs-Buy and major trade-offs.

Before handoff:

1. Validate implementation and tests.
2. Update `docs/status/current.md` and `docs/status/history/YYYY-MM-DD.md`.
3. Update `docs/state/project_state.json` (`next_actions`, `last_verified_commit`, `updated_at`).
4. Add/update ADR when architecture/contract decisions changed.
