# Session Start Runbook

## Mandatory Order

1. Read `AGENTS.md`.
2. Read `docs/status/current.md`.
3. Read `docs/state/project_state.json`.
4. Read the latest accepted ADR in `docs/decisions/`.

## How to Select Latest ADR

1. Ignore `ADR-template.md`.
2. Consider only files matching `ADR-*.md`.
3. Prefer ADR files with `## Status` containing `Accepted`.
4. If multiple accepted ADRs exist, choose the latest by ADR filename date sequence.
5. If date sequence ties, choose the one with the latest git commit time.

If no accepted ADR exists:

1. Session may proceed.
2. If the session introduces architecture-impacting changes, create a new ADR in `docs/decisions/` and set status explicitly.

## Start Checklist

1. Confirm active milestone, `next_actions`, and `open_risks`.
2. Run `scripts/agent/check_state.sh` before coding.
3. Keep `trace_id`, audit fields, and event metadata requirements visible for implementation.

## Sandbox and Cache Policy

1. If a command fails due to sandbox permission limits, request permission escalation and rerun the original command.
2. Do not bypass sandbox limits by configuring project-local caches such as `pnpm-store`, `GOCACHE`, `GOMODCACHE`, `GOPATH/pkg/mod`, or `npm` cache.
