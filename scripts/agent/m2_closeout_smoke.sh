#!/usr/bin/env bash
set -euo pipefail

INGRESS_URL="${INGRESS_URL:-http://localhost:18080}"
IDENTITY_URL="${IDENTITY_URL:-http://localhost:18081}"
APIKEY_URL="${APIKEY_URL:-http://localhost:18082}"
EXECUTION_URL="${EXECUTION_URL:-http://localhost:18083}"
ROUTING_URL="${ROUTING_URL:-http://localhost:18084}"
PROMPT_URL="${PROMPT_URL:-http://localhost:18085}"
BILLING_URL="${BILLING_URL:-http://localhost:18086}"

BOOTSTRAP_USER="${M2_SUPERUSER_USERNAME:-root}"
BOOTSTRAP_PASS="${M2_SUPERUSER_PASSWORD:-pass}"

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
  local base="$2"
  local retries=30
  local ok=0
  for ((i=1; i<=retries; i++)); do
    if request GET "$base/healthz"; then
      if [[ "$HTTP_CODE" == "200" ]]; then
        ok=1
        break
      fi
    fi
    sleep 2
  done
  if [[ "$ok" -ne 1 ]]; then
    echo "healthz check failed after retries: $name"
    echo "Last status: ${HTTP_CODE:-N/A}"
    echo "Last body: ${HTTP_BODY:-N/A}"
    exit 1
  fi
  echo "healthz ok: $name"
}

require_bin curl
require_bin jq

if [[ "${M2_SMOKE_SKIP_RECOVER:-0}" != "1" ]] && command -v docker >/dev/null 2>&1; then
  if docker compose version >/dev/null 2>&1; then
    docker compose up -d identity-service apikey-service execution-service routing-service prompt-service billing-service ingress-service >/dev/null
  fi
fi

health_check ingress "$INGRESS_URL"
health_check identity "$IDENTITY_URL"
health_check apikey "$APIKEY_URL"
health_check execution "$EXECUTION_URL"
health_check routing "$ROUTING_URL"
health_check prompt "$PROMPT_URL"
health_check billing "$BILLING_URL"

request POST "$IDENTITY_URL/v1/auth/bootstrap-superuser" "{\"username\":\"$BOOTSTRAP_USER\",\"password\":\"$BOOTSTRAP_PASS\",\"display_name\":\"Root\"}"
if [[ "$HTTP_CODE" != "201" && "$HTTP_CODE" != "409" ]]; then
  echo "bootstrap superuser failed"
  echo "Body: $HTTP_BODY"
  exit 1
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

request POST "$APIKEY_URL/v1/keys" "{\"owner_id\":\"$OWNER_ID\",\"name\":\"m2-smoke-key\",\"allowed_models\":[\"gpt-4o-mini\"]}"
assert_status "201"
PLAIN_KEY="$(jq -r '.api_key // empty' <<<"$HTTP_BODY")"
if [[ -z "$PLAIN_KEY" ]]; then
  echo "failed to parse plaintext api key"
  echo "Body: $HTTP_BODY"
  exit 1
fi

request POST "$INGRESS_URL/v1/control/validate" "{\"api_key\":\"$PLAIN_KEY\",\"model\":\"gpt-4o-mini\"}" "$TOKEN"
assert_status "200"
assert_json_true '.allowed == true'

TS="$(date +%s)"
PROVIDER_NAME="m2-provider-$TS"
CUSTOM_MODEL="m2_custom_$TS"
SCENE_NAME="m2_scene_$TS"

request POST "$EXECUTION_URL/v1/providers" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"owner_id\":\"$OWNER_ID\",\"name\":\"$PROVIDER_NAME\",\"protocol\":\"openai-compatible\",\"base_url\":\"https://example.com/v1\",\"api_key\":\"sk-smoke\"}"
assert_status "201"
PROVIDER_ID="$(jq -r '.provider.id // empty' <<<"$HTTP_BODY")"
if [[ -z "$PROVIDER_ID" ]]; then
  echo "failed to parse provider id"
  echo "Body: $HTTP_BODY"
  exit 1
fi

request POST "$EXECUTION_URL/v1/models" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"owner_id\":\"$OWNER_ID\",\"provider_id\":\"$PROVIDER_ID\",\"name\":\"m2-model-$TS\",\"upstream_model\":\"gpt-4o-mini\"}"
assert_status "201"

request POST "$ROUTING_URL/v1/policies" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"owner_id\":\"$OWNER_ID\",\"custom_model\":\"$CUSTOM_MODEL\",\"target_provider_id\":\"$PROVIDER_ID\",\"target_model\":\"gpt-4o-mini\",\"priority\":10}"
assert_status "201"

request POST "$ROUTING_URL/v1/policies/resolve" "{\"owner_id\":\"$OWNER_ID\",\"custom_model\":\"$CUSTOM_MODEL\"}"
assert_status "200"
assert_json_true '.matched == true'

request POST "$PROMPT_URL/v1/templates" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"owner_id\":\"$OWNER_ID\",\"scene\":\"$SCENE_NAME\",\"content\":\"Hi {{name}}\",\"variables\":[{\"name\":\"name\",\"type\":\"string\",\"required\":true}]}"
assert_status "201"

request POST "$PROMPT_URL/v1/render" "{\"owner_id\":\"$OWNER_ID\",\"scene\":\"$SCENE_NAME\",\"variables\":{\"name\":\"smoke\"}}"
assert_status "200"
assert_json_true '.prompt | type == "string" and length > 0'

request POST "$BILLING_URL/v1/prices" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"owner_id\":\"$OWNER_ID\",\"provider_id\":\"$PROVIDER_ID\",\"model\":\"gpt-4o-mini\",\"input_price_per_1k\":0.2,\"output_price_per_1k\":0.4,\"currency\":\"USD\"}"
assert_status "200"

request POST "$BILLING_URL/v1/wallets/$OWNER_ID/topup" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"amount_cents\":500,\"reason\":\"smoke_topup\"}"
assert_status "200"

request POST "$BILLING_URL/v1/wallets/$OWNER_ID/deduct" "{\"actor_id\":\"$OWNER_ID\",\"actor_is_superuser\":false,\"amount_cents\":200,\"reason\":\"smoke_deduct\"}"
assert_status "200"

request GET "$BILLING_URL/v1/wallets/$OWNER_ID"
assert_status "200"
assert_json_true '.wallet.balance_cents >= 300'

echo "M2 closeout smoke test: PASS"
