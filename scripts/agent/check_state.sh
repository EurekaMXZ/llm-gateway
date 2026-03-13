#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STATE_FILE="$ROOT_DIR/docs/state/project_state.json"
FAILED=0

if [[ ! -f "$STATE_FILE" ]]; then
  echo "Missing state file: $STATE_FILE"
  exit 1
fi

if command -v jq >/dev/null 2>&1; then
  if jq -e '
    (.project_version | type == "string" and length > 0) and
    (.current_milestone | type == "string" and IN("M0", "M1", "M2", "M3", "M4", "M5")) and
    (.services | type == "array" and length > 0 and all(.[]; type == "string" and length > 0)) and
    (.api_contract_versions | type == "object") and
    (.event_contract_versions | type == "object") and
    (.open_risks | type == "array" and all(.[]; type == "string")) and
    (.next_actions | type == "array" and all(.[]; type == "string")) and
    (.last_verified_commit | type == "string" and (. == "UNCOMMITTED" or test("^[0-9a-fA-F]{7,40}$"))) and
    (.updated_at | type == "string" and test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"))
  ' "$STATE_FILE" >/dev/null; then
    echo "State file schema check (jq): PASS"
  else
    echo "State file schema check (jq): FAIL"
    FAILED=1
  fi
else
  echo "jq not found; running fallback state checks."
  for key in \
    project_version \
    current_milestone \
    services \
    api_contract_versions \
    event_contract_versions \
    open_risks \
    next_actions \
    last_verified_commit \
    updated_at; do
    if ! rg -q "\"$key\"[[:space:]]*:" "$STATE_FILE"; then
      echo "Fallback check missing key: $key"
      FAILED=1
    fi
  done

  MILESTONE="$(sed -n 's/.*"current_milestone"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$STATE_FILE" | head -n1)"
  if [[ ! "$MILESTONE" =~ ^M[0-5]$ ]]; then
    echo "Fallback check invalid current_milestone: ${MILESTONE:-<empty>}"
    FAILED=1
  fi

  VERIFIED_COMMIT="$(sed -n 's/.*"last_verified_commit"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$STATE_FILE" | head -n1)"
  if [[ "$VERIFIED_COMMIT" != "UNCOMMITTED" && ! "$VERIFIED_COMMIT" =~ ^[0-9a-fA-F]{7,40}$ ]]; then
    echo "Fallback check invalid last_verified_commit: ${VERIFIED_COMMIT:-<empty>}"
    FAILED=1
  fi

  UPDATED_AT="$(sed -n 's/.*"updated_at"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$STATE_FILE" | head -n1)"
  if [[ ! "$UPDATED_AT" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]; then
    echo "Fallback check invalid updated_at: ${UPDATED_AT:-<empty>}"
    FAILED=1
  fi
fi

is_path_under_repo() {
  local path="$1"
  [[ -z "$path" ]] && return 1
  local abs_path
  abs_path="$(realpath -m "$path" 2>/dev/null || printf "%s" "$path")"
  [[ "$abs_path" == "$ROOT_DIR" || "$abs_path" == "$ROOT_DIR/"* ]]
}

check_forbidden_cache_env() {
  local name="$1"
  local value="$2"
  if [[ -n "$value" ]] && is_path_under_repo "$value"; then
    echo "Forbidden cache path in repo detected: $name=$value"
    FAILED=1
  fi
}

check_forbidden_cache_env "GOCACHE" "${GOCACHE:-}"
check_forbidden_cache_env "GOMODCACHE" "${GOMODCACHE:-}"
check_forbidden_cache_env "PNPM_STORE_DIR" "${PNPM_STORE_DIR:-}"
check_forbidden_cache_env "NPM_CONFIG_CACHE" "${NPM_CONFIG_CACHE:-}"

if [[ -n "${GOPATH:-}" ]]; then
  check_forbidden_cache_env "GOPATH/pkg/mod" "${GOPATH}/pkg/mod"
fi

for forbidden_path in \
  "$ROOT_DIR/.pnpm-store" \
  "$ROOT_DIR/pnpm-store" \
  "$ROOT_DIR/.gocache" \
  "$ROOT_DIR/.gomodcache" \
  "$ROOT_DIR/.npm"; do
  if [[ -e "$forbidden_path" ]]; then
    echo "Forbidden project-local cache directory detected: $forbidden_path"
    FAILED=1
  fi
done

if [[ "$FAILED" -ne 0 ]]; then
  echo "State/cache policy checks: FAIL"
  exit 1
fi

echo "State/cache policy checks: PASS"
echo "Reminder: if sandbox permission fails, request escalation and rerun the original command."
