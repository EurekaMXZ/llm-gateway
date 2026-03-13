# Session Close Runbook

## Mandatory Updates

1. Update `docs/status/current.md`.
2. Append/update `docs/status/history/YYYY-MM-DD.md`.
3. Update `docs/state/project_state.json`.
4. Record ADR updates when architecture/contract decisions changed.

## `project_state.json` Minimum Fields

1. `next_actions`
2. `last_verified_commit`
3. `updated_at` (RFC3339 UTC)

## History Entry Required Sections

1. `Summary`
2. `Decisions`
3. `Validation` (commands/tests run and results)
4. `Changed Files` (key paths)
5. `Next`

## Conflict Handling

1. Rebase before final status/state file write when multiple sessions are active.
2. Keep factual history; do not remove prior validated records.
3. For `current.md` conflicts, keep latest factual state and move superseded notes into the day history snapshot.
4. For `project_state.json` conflicts, keep newest `updated_at` and merge `next_actions` without dropping still-open actions.

## Session Close Acceptance Gate

1. Code/contracts/docs consistency verified.
2. Memory/state files synchronized.
3. New assumptions/trade-offs recorded in ADR or status notes.
4. `last_verified_commit` reflects verified commit SHA or `UNCOMMITTED`.
5. If current milestone is M2, run `make m2-smoke` (or document why skipped).

## Sandbox and Cache Policy Reminder

1. If escalation was required, note command + reason in session notes.
2. Do not create project-local cache paths to bypass sandbox restrictions.
