#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
E2E_POSTGRES_CONTAINER="axonhub-e2e-postgres"
COMPOSE_PROJECT="axonhub-commercial-acceptance"
COMPOSE_PASSWORD="stage21_postgres_password"
PAYMENT_SECRET="stage21-commercial-payment-secret-32-bytes"
COMPOSE_APP_PORT="18090"
COMPOSE_POSTGRES_PORT="15433"

EXPECTED_TABLES=(
  billing_accounts
  billing_account_bindings
  ledger_transactions
  ledger_entries
  payment_orders
  payment_events
  payment_provider_instances
  billing_price_rules
  subscription_plans
  user_subscriptions
  redeem_codes
  usage_billing_records
  usage_hourly_aggregates
  usage_daily_aggregates
  upstream_account_pools
  upstream_accounts
  upstream_account_switch_histories
)

cleanup() {
  AXONHUB_E2E_DB_TYPE=postgres AXONHUB_E2E_KEEP_DB=true \
    "$SCRIPT_DIR/e2e-backend.sh" stop >/dev/null 2>&1 || true
  docker rm -f "$E2E_POSTGRES_CONTAINER" >/dev/null 2>&1 || true
  run_compose down --volumes --remove-orphans >/dev/null 2>&1 || true
}

run_compose() {
  DB_PASSWORD="$COMPOSE_PASSWORD" \
  AXONHUB_PAYMENT_SECRET_KEY="$PAYMENT_SECRET" \
  AXONHUB_PORT="$COMPOSE_APP_PORT" \
  POSTGRES_PORT="$COMPOSE_POSTGRES_PORT" \
    docker compose -p "$COMPOSE_PROJECT" "$@"
}

verify_tables() {
  local container="$1"
  local database="$2"
  local user="$3"
  local values=""
  local table

  for table in "${EXPECTED_TABLES[@]}"; do
    if [[ -n "$values" ]]; then
      values+=","
    fi
    values+="('$table')"
  done

  local missing
  missing="$(docker exec "$container" psql -U "$user" -d "$database" -Atqc \
    "WITH expected(name) AS (VALUES $values) SELECT name FROM expected WHERE to_regclass('public.' || name) IS NULL ORDER BY name")"
  if [[ -n "$missing" ]]; then
    echo "Missing PostgreSQL tables:"
    echo "$missing"
    return 1
  fi
}

wait_for_tables() {
  local container="$1"
  local database="$2"
  local user="$3"
  local attempts="${4:-60}"
  local i

  for ((i = 1; i <= attempts; i++)); do
    if verify_tables "$container" "$database" "$user" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  verify_tables "$container" "$database" "$user"
}

verify_seeded_commercial_data() {
  local result
  result="$(docker exec "$E2E_POSTGRES_CONTAINER" psql -U axonhub -d axonhub_e2e -Atqc \
    "SELECT (SELECT count(*) FROM billing_accounts) > 0 AND (SELECT count(*) FROM payment_orders) > 0 AND (SELECT count(*) FROM user_subscriptions) > 0 AND (SELECT count(*) FROM upstream_account_pools) > 0 AND (SELECT count(*) FROM upstream_accounts) > 0")"
  if [[ "$result" != "t" ]]; then
    echo "Commercial smoke did not persist all expected PostgreSQL records."
    return 1
  fi
}

wait_for_health() {
  local url="$1"
  local attempts="${2:-60}"
  local i

  for ((i = 1; i <= attempts; i++)); do
    if curl --fail --silent --show-error "$url" >/dev/null; then
      return 0
    fi
    sleep 1
  done

  echo "Health endpoint did not become ready: $url"
  return 1
}

wait_for_container_health() {
  local container="$1"
  local attempts="${2:-90}"
  local i
  local status

  for ((i = 1; i <= attempts; i++)); do
    status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null || true)"
    if [[ "$status" == "healthy" ]]; then
      return 0
    fi
    if [[ "$status" == "unhealthy" || "$status" == "exited" || "$status" == "dead" ]]; then
      docker logs "$container" >&2 || true
      echo "Container failed health validation: $container ($status)"
      return 1
    fi
    sleep 1
  done

  docker logs "$container" >&2 || true
  echo "Container did not become healthy: $container"
  return 1
}

trap cleanup EXIT

cd "$PROJECT_ROOT"

echo "Building the current backend for PostgreSQL browser acceptance..."
go build -o scripts/e2e/axonhub-e2e ./cmd/axonhub

echo "Running commercial browser smoke against PostgreSQL..."
AXONHUB_E2E_KEEP_DB=true \
  ./scripts/e2e/e2e-test.sh --db-type postgres \
  commercial-smoke.spec.ts upstream-accounts-smoke.spec.ts

echo "Verifying commercial and account-pool PostgreSQL tables..."
wait_for_tables "$E2E_POSTGRES_CONTAINER" axonhub_e2e axonhub 30
verify_seeded_commercial_data

echo "Restarting the backend against the preserved PostgreSQL database..."
AXONHUB_E2E_DB_TYPE=postgres \
AXONHUB_E2E_USE_EXISTING_DB=true \
AXONHUB_E2E_KEEP_DB=true \
  ./scripts/e2e/e2e-backend.sh start
wait_for_health "http://localhost:8099/health" 30
AXONHUB_E2E_DB_TYPE=postgres AXONHUB_E2E_KEEP_DB=true \
  ./scripts/e2e/e2e-backend.sh stop

docker rm -f "$E2E_POSTGRES_CONTAINER" >/dev/null

echo "Building and starting the current Docker image with PostgreSQL..."
run_compose up -d --build
wait_for_container_health axonhub-app 90
wait_for_health "http://localhost:${COMPOSE_APP_PORT}/health" 30

echo "Verifying the Docker Compose PostgreSQL schema..."
wait_for_tables axonhub-postgres axonhub axonhub 60

echo "PostgreSQL commercial acceptance passed."
