#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BASELINE_COMMIT="${AXONHUB_RECOVERY_BASELINE_COMMIT:-26584ccf}"
POSTGRES_CONTAINER="axonhub-recovery-postgres"
POSTGRES_PORT="${AXONHUB_RECOVERY_POSTGRES_PORT:-15434}"
POSTGRES_DATABASE="axonhub_recovery"
POSTGRES_USER="axonhub"
POSTGRES_PASSWORD="stage22_recovery_password"
LEGACY_PORT="18099"
OWNER_EMAIL="my@example.com"
OWNER_PASSWORD="pwd123456"
RECOVERY_USER_PASSWORD="SmokePass123"
PAYMENT_SECRET="stage22-commercial-payment-secret-32-bytes"
SIMULATED_EPAY_SECRET="axonhub-smoke-epay-secret"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/axonhub-stage22-recovery.XXXXXX")"
BASELINE_SOURCE="$TEMP_DIR/baseline"
BASELINE_BINARY="$TEMP_DIR/axonhub-baseline"
CURRENT_BINARY="$SCRIPT_DIR/axonhub-e2e"
BACKUP_FILE="$TEMP_DIR/commercial-backup.sql"
SERVER_LOG="$TEMP_DIR/server.log"
SERVER_PID=""

psql_query() {
  docker exec "$POSTGRES_CONTAINER" psql \
    -U "$POSTGRES_USER" \
    -d "$POSTGRES_DATABASE" \
    -v ON_ERROR_STOP=1 \
    -Atqc "$1"
}

stop_direct_server() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  SERVER_PID=""
}

cleanup() {
  stop_direct_server
  AXONHUB_E2E_DB_TYPE=postgres \
  AXONHUB_E2E_USE_EXISTING_DB=true \
  AXONHUB_E2E_KEEP_DB=true \
    "$SCRIPT_DIR/e2e-backend.sh" stop >/dev/null 2>&1 || true
  docker rm -f "$POSTGRES_CONTAINER" >/dev/null 2>&1 || true
  rm -rf "$TEMP_DIR"
}

wait_for_postgres() {
  local attempts="${1:-30}"
  local i

  for ((i = 1; i <= attempts; i++)); do
    if docker exec "$POSTGRES_CONTAINER" pg_isready \
      -U "$POSTGRES_USER" \
      -d "$POSTGRES_DATABASE" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  docker logs "$POSTGRES_CONTAINER" >&2 || true
  echo "PostgreSQL did not become ready."
  return 1
}

wait_for_health() {
  local port="$1"
  local attempts="${2:-60}"
  local i

  for ((i = 1; i <= attempts; i++)); do
    if curl --fail --silent --show-error "http://localhost:${port}/health" >/dev/null 2>&1; then
      return 0
    fi
    if [[ -n "$SERVER_PID" ]] && ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
      cat "$SERVER_LOG" >&2 || true
      echo "AxonHub exited before becoming healthy."
      return 1
    fi
    sleep 1
  done

  cat "$SERVER_LOG" >&2 || true
  echo "AxonHub health endpoint did not become ready on port ${port}."
  return 1
}

start_direct_server() {
  local binary="$1"
  local port="$2"

  stop_direct_server
  : > "$SERVER_LOG"

  AXONHUB_SERVER_PORT="$port" \
  AXONHUB_DB_DIALECT=postgres \
  AXONHUB_DB_DSN="host=localhost port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DATABASE} sslmode=disable" \
  AXONHUB_PAYMENT_SECRET_KEY="$PAYMENT_SECRET" \
  AXONHUB_LOG_OUTPUT=stdio \
  AXONHUB_LOG_LEVEL=info \
    "$binary" > "$SERVER_LOG" 2>&1 &
  SERVER_PID=$!

  wait_for_health "$port" 60
}

build_baseline() {
  mkdir -p "$BASELINE_SOURCE"
  git archive "$BASELINE_COMMIT" | tar -x -C "$BASELINE_SOURCE"

  pushd "$BASELINE_SOURCE" >/dev/null
  go build -o "$BASELINE_BINARY" ./cmd/axonhub
  popd >/dev/null
}

initialize_legacy_database() {
  local response

  response="$(curl --fail --silent --show-error \
    -X POST \
    -H 'Content-Type: application/json' \
    -d "{\"ownerEmail\":\"${OWNER_EMAIL}\",\"ownerPassword\":\"${OWNER_PASSWORD}\",\"ownerFirstName\":\"Stage\",\"ownerLastName\":\"TwentyTwo\",\"brandName\":\"AxonHub Recovery Acceptance\"}" \
    "http://localhost:${LEGACY_PORT}/admin/system/initialize")"

  if [[ "$response" != *'"success":true'* ]]; then
    echo "Legacy system initialization failed: $response"
    return 1
  fi
}

legacy_manifest() {
  psql_query "
    SELECT 'owner|' || id || '|' || email || '|' || first_name || '|' || last_name
      FROM users WHERE email = '${OWNER_EMAIL}';
    SELECT 'users|' || count(*) FROM users;
    SELECT 'projects|' || count(*) FROM projects;
    SELECT 'systems|' || count(*) FROM systems;
  "
}

commercial_manifest() {
  local recovery_email="$1"

  psql_query "
    SELECT 'users|' || count(*) FROM users;
    SELECT 'projects|' || count(*) FROM projects;
    SELECT 'billing_accounts|' || count(*) FROM billing_accounts;
    SELECT 'ledger_transactions|' || count(*) FROM ledger_transactions;
    SELECT 'ledger_entries|' || count(*) FROM ledger_entries;
    SELECT 'payment_orders|' || count(*) FROM payment_orders;
    SELECT 'payment_events|' || count(*) FROM payment_events;
    SELECT 'payment_provider_instances|' || count(*) FROM payment_provider_instances;
    SELECT 'subscription_plans|' || count(*) FROM subscription_plans;
    SELECT 'user_subscriptions|' || count(*) FROM user_subscriptions;
    SELECT 'redeem_codes|' || count(*) FROM redeem_codes;
    SELECT 'recovery_user|' || u.id || '|' || u.email || '|' || b.id || '|' || b.balance_micros || '|' || p.status || '|' || s.status || '|' || sp.name
      FROM users u
      JOIN billing_accounts b ON b.owner_type = 'user' AND b.owner_id = u.id
      JOIN payment_orders p ON p.billing_account_id = b.id AND p.status = 'paid'
      JOIN user_subscriptions s ON s.user_id = u.id AND s.status = 'active'
      JOIN subscription_plans sp ON sp.id = s.plan_id
      WHERE u.email = '${recovery_email}';
  "
}

verify_commercial_tables() {
  local missing

  missing="$(psql_query "
    WITH expected(name) AS (
      VALUES
        ('billing_accounts'),
        ('billing_account_bindings'),
        ('ledger_transactions'),
        ('ledger_entries'),
        ('payment_orders'),
        ('payment_events'),
        ('payment_provider_instances'),
        ('subscription_plans'),
        ('user_subscriptions'),
        ('redeem_codes'),
        ('usage_billing_records'),
        ('usage_hourly_aggregates'),
        ('usage_daily_aggregates'),
        ('upstream_account_pools'),
        ('upstream_accounts'),
        ('upstream_account_switch_histories')
    )
    SELECT name
      FROM expected
      WHERE to_regclass('public.' || name) IS NULL
      ORDER BY name;
  ")"

  if [[ -n "$missing" ]]; then
    echo "Missing tables after historical database migration:"
    echo "$missing"
    return 1
  fi
}

run_commercial_smoke() {
  AXONHUB_ADMIN_EMAIL="$OWNER_EMAIL" \
  AXONHUB_ADMIN_PASSWORD="$OWNER_PASSWORD" \
  AXONHUB_PAYMENT_SECRET_KEY="$PAYMENT_SECRET" \
  AXONHUB_E2E_DB_TYPE=postgres \
  AXONHUB_E2E_DB_DIALECT=postgres \
  AXONHUB_E2E_DB_DSN="host=localhost port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DATABASE} sslmode=disable" \
  AXONHUB_E2E_USE_EXISTING_DB=true \
  AXONHUB_E2E_KEEP_DB=true \
    "$SCRIPT_DIR/e2e-test.sh" commercial-smoke.spec.ts
}

run_recovery_smoke() {
  local recovery_email="$1"

  AXONHUB_ADMIN_EMAIL="$OWNER_EMAIL" \
  AXONHUB_ADMIN_PASSWORD="$OWNER_PASSWORD" \
  AXONHUB_RECOVERY_USER_EMAIL="$recovery_email" \
  AXONHUB_RECOVERY_USER_PASSWORD="$RECOVERY_USER_PASSWORD" \
  AXONHUB_PAYMENT_SECRET_KEY="$PAYMENT_SECRET" \
  AXONHUB_E2E_DB_TYPE=postgres \
  AXONHUB_E2E_DB_DIALECT=postgres \
  AXONHUB_E2E_DB_DSN="host=localhost port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DATABASE} sslmode=disable" \
  AXONHUB_E2E_USE_EXISTING_DB=true \
  AXONHUB_E2E_KEEP_DB=true \
    "$SCRIPT_DIR/e2e-test.sh" commercial-recovery-smoke.spec.ts
}

backup_database() {
  docker exec "$POSTGRES_CONTAINER" pg_dump \
    -U "$POSTGRES_USER" \
    --no-owner \
    --no-privileges \
    "$POSTGRES_DATABASE" > "$BACKUP_FILE"

  if [[ ! -s "$BACKUP_FILE" ]]; then
    echo "PostgreSQL backup file is empty."
    return 1
  fi

  if grep -Fq "$SIMULATED_EPAY_SECRET" "$BACKUP_FILE"; then
    echo "Plaintext simulated ePay secret was found in the PostgreSQL backup."
    return 1
  fi
}

restore_database() {
  docker exec "$POSTGRES_CONTAINER" dropdb \
    --force \
    -U "$POSTGRES_USER" \
    "$POSTGRES_DATABASE"
  docker exec "$POSTGRES_CONTAINER" createdb \
    -U "$POSTGRES_USER" \
    "$POSTGRES_DATABASE"
  docker exec -i "$POSTGRES_CONTAINER" psql \
    -U "$POSTGRES_USER" \
    -d "$POSTGRES_DATABASE" \
    -v ON_ERROR_STOP=1 < "$BACKUP_FILE" >/dev/null
}

trap cleanup EXIT

cd "$PROJECT_ROOT"

echo "Starting isolated PostgreSQL recovery acceptance..."
docker rm -f "$POSTGRES_CONTAINER" >/dev/null 2>&1 || true
docker run -d \
  --name "$POSTGRES_CONTAINER" \
  -e POSTGRES_DB="$POSTGRES_DATABASE" \
  -e POSTGRES_USER="$POSTGRES_USER" \
  -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
  -p "${POSTGRES_PORT}:5432" \
  postgres:15-alpine >/dev/null
wait_for_postgres 30

echo "Building the pre-commercial baseline ${BASELINE_COMMIT}..."
build_baseline

echo "Initializing a historical PostgreSQL database..."
start_direct_server "$BASELINE_BINARY" "$LEGACY_PORT"
initialize_legacy_database
LEGACY_BEFORE="$(legacy_manifest)"
stop_direct_server

echo "Building the current AxonHub fork..."
go build -o "$CURRENT_BINARY" ./cmd/axonhub

echo "Migrating the historical database to the current schema..."
start_direct_server "$CURRENT_BINARY" "$LEGACY_PORT"
verify_commercial_tables
LEGACY_AFTER="$(legacy_manifest)"
stop_direct_server

if [[ "$LEGACY_BEFORE" != "$LEGACY_AFTER" ]]; then
  echo "Legacy owner, project, or system records changed during migration."
  diff -u <(printf '%s\n' "$LEGACY_BEFORE") <(printf '%s\n' "$LEGACY_AFTER") || true
  exit 1
fi

echo "Running the commercial browser lifecycle on the upgraded database..."
run_commercial_smoke

RECOVERY_USER_EMAIL="$(psql_query "SELECT email FROM users WHERE email LIKE 'commercial-smoke-%@example.com' ORDER BY id DESC LIMIT 1")"
if [[ -z "$RECOVERY_USER_EMAIL" ]]; then
  echo "Commercial smoke user was not persisted in PostgreSQL."
  exit 1
fi

MANIFEST_BEFORE="$(commercial_manifest "$RECOVERY_USER_EMAIL")"

echo "Backing up the upgraded commercial database..."
backup_database

echo "Dropping, recreating, and restoring the PostgreSQL database..."
restore_database
MANIFEST_AFTER="$(commercial_manifest "$RECOVERY_USER_EMAIL")"

if [[ "$MANIFEST_BEFORE" != "$MANIFEST_AFTER" ]]; then
  echo "Commercial records changed during PostgreSQL backup and restore."
  diff -u <(printf '%s\n' "$MANIFEST_BEFORE") <(printf '%s\n' "$MANIFEST_AFTER") || true
  exit 1
fi

echo "Running browser smoke against the restored commercial database..."
run_recovery_smoke "$RECOVERY_USER_EMAIL"

echo "Historical migration and PostgreSQL backup/restore acceptance passed."
