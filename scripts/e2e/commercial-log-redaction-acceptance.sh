#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
APP_PORT="${AXONHUB_LOG_ACCEPTANCE_PORT:-18100}"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/axonhub-stage23-log-redaction.XXXXXX")"
BINARY_PATH="$TEMP_DIR/axonhub"
DATABASE_PATH="$TEMP_DIR/axonhub.db"
LOG_PATH="$TEMP_DIR/axonhub.log"
SERVER_STDERR="$TEMP_DIR/server.stderr"
SERVER_PID=""

OWNER_EMAIL="stage23-owner@example.com"
OWNER_PASSWORD="stage23-owner-password-canary"
PAYMENT_ENCRYPTION_KEY="stage23-commercial-payment-secret-32-bytes"
PAYMENT_PROVIDER_KEY="stage23-epay-provider-key-canary"
HEADER_API_KEY="sk-stage23-header-api-key-canary"
UPSTREAM_CREDENTIAL="stage23-upstream-credential-canary"
COOKIE_VALUE="stage23-cookie-value-canary"
BODY_PASSWORD="stage23-body-password-canary"
BODY_SECRET="stage23-body-secret-canary"
DSN_PASSWORD="stage23-dsn-password-canary"
QUERY_TOKEN="stage23-query-token-canary"
SAFE_MARKER="stage23-safe-log-marker"

cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$TEMP_DIR"
}

wait_for_health() {
  local attempts="${1:-60}"
  local i

  for ((i = 1; i <= attempts; i++)); do
    if curl --fail --silent --show-error "http://localhost:${APP_PORT}/health" >/dev/null 2>&1; then
      return 0
    fi
    if [[ -n "$SERVER_PID" ]] && ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
      cat "$SERVER_STDERR" >&2 || true
      echo "AxonHub exited before log-redaction acceptance became ready."
      return 1
    fi
    sleep 1
  done

  cat "$SERVER_STDERR" >&2 || true
  echo "AxonHub did not become healthy on port ${APP_PORT}."
  return 1
}

graphql_request() {
  local admin_token="$1"
  local payload="$2"

  curl --fail --silent --show-error \
    -X POST \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${admin_token}" \
    -d "$payload" \
    "http://localhost:${APP_PORT}/admin/graphql"
}

assert_graphql_success() {
  local response="$1"
  local path="$2"

  if ! jq -e ".errors == null and ${path} != null" >/dev/null <<<"$response"; then
    echo "GraphQL request failed: $response"
    return 1
  fi
}

trap cleanup EXIT

cd "$PROJECT_ROOT"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for the log-redaction acceptance script."
  exit 1
fi

if curl --fail --silent "http://localhost:${APP_PORT}/health" >/dev/null 2>&1; then
  echo "Port ${APP_PORT} is already serving an AxonHub health endpoint."
  exit 1
fi

echo "Running focused log redaction tests..."
go test ./internal/log -count=1

echo "Building the current AxonHub fork..."
go build -o "$BINARY_PATH" ./cmd/axonhub

echo "Starting AxonHub with debug file logging..."
AXONHUB_SERVER_PORT="$APP_PORT" \
AXONHUB_DB_DIALECT=sqlite3 \
AXONHUB_DB_DSN="file:${DATABASE_PATH}?cache=shared&_fk=1" \
AXONHUB_PAYMENT_SECRET_KEY="$PAYMENT_ENCRYPTION_KEY" \
AXONHUB_LOG_OUTPUT=file \
AXONHUB_LOG_FILE_PATH="$LOG_PATH" \
AXONHUB_LOG_LEVEL=debug \
AXONHUB_LOG_ENCODING=json \
  "$BINARY_PATH" >"$SERVER_STDERR" 2>&1 &
SERVER_PID=$!
wait_for_health 60

echo "Initializing the isolated application..."
INITIALIZE_RESPONSE="$(jq -n \
  --arg email "$OWNER_EMAIL" \
  --arg password "$OWNER_PASSWORD" \
  '{ownerEmail: $email, ownerPassword: $password, ownerFirstName: "Stage", ownerLastName: "TwentyThree", brandName: "AxonHub Log Acceptance"}' | \
  curl --fail --silent --show-error \
    -X POST \
    -H 'Content-Type: application/json' \
    --data-binary @- \
    "http://localhost:${APP_PORT}/admin/system/initialize")"
if ! jq -e '.success == true' >/dev/null <<<"$INITIALIZE_RESPONSE"; then
  echo "System initialization failed: $INITIALIZE_RESPONSE"
  exit 1
fi

SIGNIN_RESPONSE="$(jq -n \
  --arg email "$OWNER_EMAIL" \
  --arg password "$OWNER_PASSWORD" \
  '{email: $email, password: $password}' | \
  curl --fail --silent --show-error \
    -X POST \
    -H 'Content-Type: application/json' \
    --data-binary @- \
    "http://localhost:${APP_PORT}/admin/auth/signin")"
if ! ADMIN_TOKEN="$(jq -er '.token' <<<"$SIGNIN_RESPONSE")"; then
  echo "Owner sign-in did not return a token."
  exit 1
fi

PROJECT_PAYLOAD="$(jq -n '{
  query: "query Stage23Projects { projects(first: 1) { edges { node { id } } } }",
  operationName: "Stage23Projects",
  variables: {}
}')"
PROJECT_RESPONSE="$(graphql_request "$ADMIN_TOKEN" "$PROJECT_PAYLOAD")"
assert_graphql_success "$PROJECT_RESPONSE" '.data.projects.edges[0].node.id'
PROJECT_ID="$(jq -er '.data.projects.edges[0].node.id' <<<"$PROJECT_RESPONSE")"

echo "Creating a service API key through GraphQL..."
CREATE_KEY_PAYLOAD="$(jq -n \
  --arg project_id "$PROJECT_ID" \
  '{
    query: "mutation Stage23CreateAPIKey($input: CreateAPIKeyInput!) { createAPIKey(input: $input) { id key name type } }",
    operationName: "Stage23CreateAPIKey",
    variables: {
      input: {
        name: "Stage 23 Log Acceptance",
        type: "service_account",
        scopes: [],
        projectID: $project_id
      }
    }
  }')"
CREATE_KEY_RESPONSE="$(graphql_request "$ADMIN_TOKEN" "$CREATE_KEY_PAYLOAD")"
assert_graphql_success "$CREATE_KEY_RESPONSE" '.data.createAPIKey.key'
SERVICE_API_KEY="$(jq -er '.data.createAPIKey.key' <<<"$CREATE_KEY_RESPONSE")"

echo "Saving an ePay provider with a canary secret through GraphQL..."
CREATE_PROVIDER_PAYLOAD="$(jq -n \
  --arg key "$PAYMENT_PROVIDER_KEY" \
  --arg base_url "http://localhost:${APP_PORT}" \
  '{
    query: "mutation Stage23SaveProvider($input: UpsertEPayPaymentProviderInput!) { upsertEPayPaymentProvider(input: $input) { id name status } }",
    operationName: "Stage23SaveProvider",
    variables: {
      input: {
        name: "Stage 23 ePay Provider",
        status: "enabled",
        currency: "CNY",
        gatewayUrl: ($base_url + "/payment/simulate/epay/submit"),
        pid: "stage23-pid",
        key: $key,
        notifyUrl: ($base_url + "/payment/notify/epay"),
        returnUrl: ($base_url + "/billing"),
        type: "alipay",
        siteName: "AxonHub Stage 23"
      }
    }
  }')"
CREATE_PROVIDER_RESPONSE="$(graphql_request "$ADMIN_TOKEN" "$CREATE_PROVIDER_PAYLOAD")"
assert_graphql_success "$CREATE_PROVIDER_RESPONSE" '.data.upsertEPayPaymentProvider.id'

echo "Sending headers, query values, and JSON body canaries through the webhook logger..."
WEBHOOK_BODY="$(jq -n \
  --arg password "$BODY_PASSWORD" \
  --arg secret "$BODY_SECRET" \
  --arg api_key "$HEADER_API_KEY" \
  --arg upstream_credential "$UPSTREAM_CREDENTIAL" \
  --arg dsn "postgres://axonhub:${DSN_PASSWORD}@postgres:5432/axonhub" \
  --arg safe "$SAFE_MARKER" \
  '{password: $password, secret: $secret, api_key: $api_key, credentials: {api_key: $upstream_credential}, dsn: $dsn, safe: $safe}')"
curl --fail --silent --show-error \
  -X POST \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${SERVICE_API_KEY}" \
  -H "X-API-Key: ${HEADER_API_KEY}" \
  -H "Cookie: session=${COOKIE_VALUE}" \
  -d "$WEBHOOK_BODY" \
  "http://localhost:${APP_PORT}/openapi/webhook/echo?token=${QUERY_TOKEN}&safe=${SAFE_MARKER}" >/dev/null

sleep 1
kill "$SERVER_PID" >/dev/null 2>&1 || true
wait "$SERVER_PID" 2>/dev/null || true
SERVER_PID=""

if [[ ! -s "$LOG_PATH" ]]; then
  echo "AxonHub did not produce a log file."
  exit 1
fi

echo "Scanning runtime logs for sensitive canaries..."
for secret in \
  "$OWNER_PASSWORD" \
  "$PAYMENT_ENCRYPTION_KEY" \
  "$PAYMENT_PROVIDER_KEY" \
  "$ADMIN_TOKEN" \
  "$SERVICE_API_KEY" \
  "$HEADER_API_KEY" \
  "$UPSTREAM_CREDENTIAL" \
  "$COOKIE_VALUE" \
  "$BODY_PASSWORD" \
  "$BODY_SECRET" \
  "$DSN_PASSWORD" \
  "$QUERY_TOKEN"; do
  if grep -Fq "$secret" "$LOG_PATH"; then
    echo "Sensitive canary leaked into runtime logs: $secret"
    exit 1
  fi
done

if ! grep -Fq '[REDACTED]' "$LOG_PATH"; then
  echo "Runtime logs did not contain any redaction markers."
  exit 1
fi

if ! grep -Fq "$SAFE_MARKER" "$LOG_PATH"; then
  echo "Safe webhook marker was unexpectedly removed from runtime logs."
  exit 1
fi

echo "Commercial runtime log redaction acceptance passed."
