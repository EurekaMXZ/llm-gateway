#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TODAY="$(date +%F)"
HISTORY_FILE="$ROOT_DIR/docs/status/history/$TODAY.md"

printf "Session close checklist:\n"
printf "1. Update docs/status/current.md\n"
printf "2. Update docs/status/history/%s.md\n" "$TODAY"
printf "3. Update docs/state/project_state.json\n"
printf "4. Record ADR if architecture changed\n"
printf "5. Verify last_verified_commit and updated_at fields\n"
printf "6. Note escalation usage if sandbox permission was required\n"
printf "7. Run M2 smoke script when applicable: make m2-smoke\n"

if [[ ! -f "$HISTORY_FILE" ]]; then
  printf "Creating history file for %s\n" "$TODAY"
  cat > "$HISTORY_FILE" <<'HISTORY'
# Session Log

## Summary
- 

## Decisions
- 

## Validation
- Commands:
- Result:

## Changed Files
- 

## Next
1. 
HISTORY
fi

printf "Conflict handling reminder:\n"
printf '%s\n' "- Rebase before writing shared status/state files when concurrent sessions exist."
printf '%s\n' "- Keep factual history entries; do not delete validated records."

printf "Checklist prepared.\n"
