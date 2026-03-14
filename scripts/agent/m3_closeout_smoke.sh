#!/usr/bin/env bash
set -euo pipefail

INGRESS_URL="${INGRESS_URL:-http://localhost:18080}"
IDENTITY_URL="${IDENTITY_URL:-http://localhost:18081}"
APIKEY_URL="${APIKEY_URL:-http://localhost:18082}"
EXECUTION_URL="${EXECUTION_URL:-http://localhost:18083}"
ROUTING_URL="${ROUTING_URL:-http://localhost:18084}"
PROMPT_URL="${PROMPT_URL:-http://localhost:18085}"
MOCK_OPENAI_URL="${MOCK_OPENAI_URL:-http://localhost:18088}"

MOCK_OPENAI_BASE_URL="${MOCK_OPENAI_BASE_URL:-http://mock-openai/anything/v1}"
BOOTSTRAP_USER="${M3_SUPERUSER_USERNAME:-root}"
BOOTSTRAP_PASS="${M3_SUPERUSER_PASSWORD:-pass}"

require_bin() {
  local bin="$1"
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "Missing required command: $bin"
    exit 1
  fi
}

request() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local auth="${4:-}"

  local out
  if [[ -n "$body" ]]; then
    if [[ -n "$auth" ]]; then
      out="$(curl -sS -X "$method" "$url" -H 'Content-Type: application/json' -H "Authorization: Bearer $auth" -d "$body" -w $'\n%{http_code}')"
    else
      out="$(curl -sS -X "$method" "$url" -H 'Content-Type: application/json' -d "$body" -w $'\n%{http_code}')"
    fi
  else
    if [[ -n "$auth" ]]; then
      out="$(curl -sS -X "$method" "$url" -H "Authorization: Bearer $auth" -w $'\n%{http_code}')"
    else
      out="$(curl -sS -X "$method" "$url" -w $'\n%{http_code}')"
    fi
  fi

  HTTP_BODY="${out%$'\n'*}"
  HTTP_CODE="${out##*$'\n'}"
}

request_ingress_chat() {
  local body="$1"
  local token="$2"
  local key="$3"
  local out
  out="$(curl -sS -X POST "$INGRESS_URL/v1/chat/completions" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer $token" \
    -H "X-API-Key: $key" \
    -d "$body" \
    -w $'\n%{http_code}')"
  HTTP_BODY="${out%$'\n'*}"
  HTTP_CODE="${out##*$'\n'}"
}

assert_status() {
  local expected="$1"
  if [[ "$HTTP_CODE" != "$expected" ]]; then
    echo "Request failed: expected HTTP $expected, got $HTTP_CODE"
    echo "Body: $HTTP_BODY"
    exit 1
  fi
}

assert_json_true() {
  local expr="$1"
  if ! jq -e "$expr" >/dev/null 2>&1 <<<"$HTTP_BODY"; then
    echo "Assertion failed: $expr"
    echo "Body: $HTTP_BODY"
    exit 1
  fi
}

health_check() {
  local name="$1"
  local url="$2"
  local retries=30
  local ok=0
  for ((i=1; i<=retries; i++)); do
    if request GET "$url"; then
      if [[ "$HTTP_CODE" == "200" ]]; then
        ok=1
        break
      fi
    fi
    sleep 2
  done
  if [[ "$ok" -ne 1 ]]; then
    echo "health check failed after retries: $name"
    echo "Last status: ${HTTP_CODE:-N/A}"
    echo "Last body: ${HTTP_BODY:-N/A}"
    exit 1
  fi
  echo "health ok: $name"
}

require_bin curl
require_bin jq

if [[ "${M3_SMOKE_SKIP_RECOVER:-0}" != "1" ]] && command -v docker >/dev/null 2>&1; then
  if docker compose version >/dev/null 2>&1; then
    docker compose up -d --build mock-openai identity-service apikey-service execution-service routing-service prompt-service ingress-service >/dev/null
  fi
fi

health_check ingress "$INGRESS_URL/healthz"
health_check identity "$IDENTITY_URL/healthz"
health_check apikey "$APIKEY_URL/healthz"
health_check execution "$EXECUTION_URL/healthz"
health_check routing "$ROUTING_URL/healthz"
health_check prompt "$PROMPT_URL/healthz"
health_check mock-openai "$MOCK_OPENAI_URL/status/200"

request POST "$IDENTITY_URL/v1/auth/bootstrap-superuser" "{\"username\":\"$BOOTSTRAP_USER\",\"password\":\"$BOOTSTRAP_PASS\",\"display_name\":\"Root\"}"
if [[ "$HTTP_CODE" != "201" && "$HTTP_CODE" != "409" ]]; then
  if [[ "$HTTP_CODE" == "400" ]] && jq -e '.error.message == "username already exists"' >/dev/null 2>&1 <<<"$HTTP_BODY"; then
    echo "bootstrap skipped: username already exists, continue with token flow"
  else
  echo "bootstrap superuser failed"
  echo "Body: $HTTP_BODY"
  exit 1
  fi
fi

request POST "$IDENTITY_URL/v1/auth/token" "{\"username\":\"$BOOTSTRAP_USER\",\"password\":\"$BOOTSTRAP_PASS\"}"
assert_status "200"
TOKEN="$(jq -r '.access_token // empty' <<<"$HTTP_BODY")"
OWNER_ID="$(jq -r '.user.id // empty' <<<"$HTTP_BODY")"
if [[ -z "$TOKEN" || -z "$OWNER_ID" ]]; then
  echo "failed to parse identity token or owner id"
  echo "Body: $HTTP_BODY"
  exit 1
fi

TS="$(date +%s)"
MATCH_MODEL="m3_alias_${TS}"
ROUTE_TARGET_MODEL="m3_route_target_${TS}"
# Keep direct model unique per run to avoid matching stale provider/model rows
# from previous smoke runs when execution target selection orders by priority+created_at.
DIRECT_MODEL="m3_direct_${TS}"
MATCH_UPSTREAM_MODEL="m3-upstream-route-${TS}"
DIRECT_UPSTREAM_MODEL="m3-upstream-direct-${TS}"
SCENE_NAME="m3_scene_${TS}"

request POST "$APIKEY_URL/v1/keys" "{\"owner_id\":\"$OWNER_ID\",\"name\":\"m3-smoke-key-${TS}\",\"allowed_models\":[\"$MATCH_MODEL\",\"$DIRECT_MODEL\"]}"
assert_status "201"
PLAIN_KEY="$(jq -r '.api_key // empty' <<<"$HTTP_BODY")"
if [[ -z "$PLAIN_KEY" ]]; then
  echo "failed to parse plaintext api key"
  echo "Body: $HTTP_BODY"
  exit 1
fi

request POST "$EXECUTION_URL/v1/providers" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"owner_id\":\"$OWNER_ID\",\"name\":\"m3-provider-route-${TS}\",\"protocol\":\"openai-compatible\",\"base_url\":\"$MOCK_OPENAI_BASE_URL\",\"api_key\":\"sk-route\",\"priority\":50}"
assert_status "201"
PROVIDER_ROUTE_ID="$(jq -r '.provider.id // empty' <<<"$HTTP_BODY")"
if [[ -z "$PROVIDER_ROUTE_ID" ]]; then
  echo "failed to parse routed provider id"
  exit 1
fi

request POST "$EXECUTION_URL/v1/providers" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"owner_id\":\"$OWNER_ID\",\"name\":\"m3-provider-direct-${TS}\",\"protocol\":\"openai-compatible\",\"base_url\":\"$MOCK_OPENAI_BASE_URL\",\"api_key\":\"sk-direct\",\"priority\":10}"
assert_status "201"
PROVIDER_DIRECT_ID="$(jq -r '.provider.id // empty' <<<"$HTTP_BODY")"
if [[ -z "$PROVIDER_DIRECT_ID" ]]; then
  echo "failed to parse direct provider id"
  exit 1
fi

request POST "$EXECUTION_URL/v1/models" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"owner_id\":\"$OWNER_ID\",\"provider_id\":\"$PROVIDER_ROUTE_ID\",\"name\":\"$ROUTE_TARGET_MODEL\",\"upstream_model\":\"$MATCH_UPSTREAM_MODEL\"}"
assert_status "201"

request POST "$EXECUTION_URL/v1/models" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"owner_id\":\"$OWNER_ID\",\"provider_id\":\"$PROVIDER_DIRECT_ID\",\"name\":\"$DIRECT_MODEL\",\"upstream_model\":\"$DIRECT_UPSTREAM_MODEL\"}"
assert_status "201"

request POST "$ROUTING_URL/v1/policies" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"owner_id\":\"$OWNER_ID\",\"custom_model\":\"$MATCH_MODEL\",\"target_provider_id\":\"$PROVIDER_ROUTE_ID\",\"target_model\":\"$ROUTE_TARGET_MODEL\",\"priority\":10}"
assert_status "201"

request POST "$PROMPT_URL/v1/templates" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"owner_id\":\"$OWNER_ID\",\"scene\":\"$SCENE_NAME\",\"content\":\"Follow instruction for {{name}}\",\"variables\":[{\"name\":\"name\",\"type\":\"string\",\"required\":true}]}"
assert_status "201"

request_ingress_chat "{\"model\":\"$MATCH_MODEL\",\"scene\":\"$SCENE_NAME\",\"variables\":{\"name\":\"smoke\"},\"messages\":[{\"role\":\"user\",\"content\":\"please route\"}]}" "$TOKEN" "$PLAIN_KEY"
assert_status "200"
assert_json_true ".json.model == \"$MATCH_UPSTREAM_MODEL\""

request_ingress_chat "{\"model\":\"$DIRECT_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"no policy direct\"}]}" "$TOKEN" "$PLAIN_KEY"
assert_status "200"
assert_json_true ".json.model == \"$DIRECT_UPSTREAM_MODEL\""

echo "M3 closeout smoke test: PASS"
