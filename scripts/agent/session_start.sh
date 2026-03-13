#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DECISIONS_DIR="$ROOT_DIR/docs/decisions"

is_accepted_adr() {
  local file="$1"
  awk '
    BEGIN { in_status=0; accepted=0 }
    /^##[[:space:]]+Status/ { in_status=1; next }
    in_status == 1 && /^##[[:space:]]+/ { in_status=0 }
    in_status == 1 && tolower($0) ~ /accepted/ { accepted=1; exit }
    END { exit accepted ? 0 : 1 }
  ' "$file"
}

adr_date_key() {
  local base="$1"
  if [[ "$base" =~ ^ADR-([0-9]{4})-([0-9]{2})-([0-9]{2}) ]]; then
    printf "%s%s%s" "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}"
    return
  fi
  if [[ "$base" =~ ^ADR-([0-9]{8}) ]]; then
    printf "%s" "${BASH_REMATCH[1]}"
    return
  fi
  printf "%s" "$base"
}

latest_accepted_adr() {
  local latest_file=""
  local latest_key=""
  local latest_commit_ts="0"

  shopt -s nullglob
  local file
  for file in "$DECISIONS_DIR"/ADR-*.md; do
    local base
    local key
    local commit_ts
    base="$(basename "$file")"
    [[ "$base" == "ADR-template.md" ]] && continue
    is_accepted_adr "$file" || continue

    key="$(adr_date_key "$base")"
    commit_ts="$(git -C "$ROOT_DIR" log -1 --format=%ct -- "$file" 2>/dev/null || echo 0)"

    if [[ -z "$latest_file" || "$key" > "$latest_key" || ( "$key" == "$latest_key" && "$commit_ts" -gt "$latest_commit_ts" ) ]]; then
      latest_file="$file"
      latest_key="$key"
      latest_commit_ts="$commit_ts"
    fi
  done
  shopt -u nullglob

  if [[ -n "$latest_file" ]]; then
    printf "%s" "$latest_file"
  fi
}

printf "Reading project memory files...\n"
for f in \
  "$ROOT_DIR/AGENTS.md" \
  "$ROOT_DIR/docs/status/current.md" \
  "$ROOT_DIR/docs/state/project_state.json"; do
  if [[ -f "$f" ]]; then
    printf '%s\n' "- $f"
  else
    printf "Missing required file: %s\n" "$f"
    exit 1
  fi
done

printf "Latest accepted ADR:\n"
LATEST_ADR="$(latest_accepted_adr)"
if [[ -n "$LATEST_ADR" ]]; then
  printf '%s\n' "- $LATEST_ADR"
else
  printf '%s\n' "- None found (session may proceed; create ADR for architecture-impacting changes)"
fi

printf "Policy reminder:\n"
printf '%s\n' "- If sandbox permission errors occur, request escalation and rerun."
printf '%s\n' "- Do not configure project-local cache paths (pnpm-store/GOCACHE/GOMODCACHE/npm cache) to bypass restrictions."
printf "Session bootstrap check complete.\n"
