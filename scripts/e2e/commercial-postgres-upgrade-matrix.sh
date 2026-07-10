#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
POSTGRES_CONTAINER="axonhub-commercial-upgrade-postgres"
POSTGRES_PORT="${AXONHUB_COMMERCIAL_UPGRADE_POSTGRES_PORT:-15435}"
POSTGRES_USER="axonhub"
POSTGRES_PASSWORD="stage25_upgrade_password"
APP_PORT="${AXONHUB_COMMERCIAL_UPGRADE_APP_PORT:-18101}"
PAYMENT_SECRET="stage25-commercial-payment-secret-32-bytes"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/axonhub-stage25-upgrade.XXXXXX")"
CURRENT_BINARY="$TEMP_DIR/axonhub-current"
SERVER_LOG="$TEMP_DIR/server.log"
SERVER_PID=""

BASELINES=(
  "stage1:94106a72:base"
  "stage2:bf4368b5:base"
  "stage6:261e482a:extended"
  "stage11:185594ee:extended"
)

stop_server() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  SERVER_PID=""
}

cleanup() {
  stop_server
  docker rm -f "$POSTGRES_CONTAINER" >/dev/null 2>&1 || true
  rm -rf "$TEMP_DIR"
}

wait_for_postgres() {
  local attempts="${1:-30}"
  local i

  for ((i = 1; i <= attempts; i++)); do
    if docker exec "$POSTGRES_CONTAINER" pg_isready -U "$POSTGRES_USER" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  docker logs "$POSTGRES_CONTAINER" >&2 || true
  echo "PostgreSQL did not become ready."
  return 1
}

wait_for_health() {
  local attempts="${1:-60}"
  local i

  for ((i = 1; i <= attempts; i++)); do
    if curl --fail --silent --show-error "http://localhost:${APP_PORT}/health" >/dev/null 2>&1; then
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
  echo "AxonHub did not become healthy on port ${APP_PORT}."
  return 1
}

start_server() {
  local binary="$1"
  local database="$2"

  stop_server
  : > "$SERVER_LOG"
  AXONHUB_SERVER_PORT="$APP_PORT" \
  AXONHUB_DB_DIALECT=postgres \
  AXONHUB_DB_DSN="host=localhost port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${database} sslmode=disable" \
  AXONHUB_PAYMENT_SECRET_KEY="$PAYMENT_SECRET" \
  AXONHUB_LOG_OUTPUT=stdio \
  AXONHUB_LOG_LEVEL=info \
    "$binary" > "$SERVER_LOG" 2>&1 &
  SERVER_PID=$!
  wait_for_health 60
}

psql_query() {
  local database="$1"
  local query="$2"

  docker exec "$POSTGRES_CONTAINER" psql \
    -U "$POSTGRES_USER" \
    -d "$database" \
    -v ON_ERROR_STOP=1 \
    -Atqc "$query"
}

table_exists() {
  local database="$1"
  local table="$2"

  [[ "$(psql_query "$database" "SELECT to_regclass('public.${table}') IS NOT NULL")" == "t" ]]
}

graphql_request() {
  local token="$1"
  local payload="$2"

  curl --fail --silent --show-error \
    -X POST \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${token}" \
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

initialize_baseline() {
  local baseline="$1"
  local owner_email="stage25-${baseline}@example.com"
  local owner_password="Stage25-${baseline}-Password123"
  local response

  response="$(jq -n \
    --arg email "$owner_email" \
    --arg password "$owner_password" \
    --arg brand "AxonHub Stage 25 ${baseline}" \
    '{ownerEmail: $email, ownerPassword: $password, ownerFirstName: "Stage", ownerLastName: "TwentyFive", brandName: $brand}' | \
    curl --fail --silent --show-error \
      -X POST \
      -H 'Content-Type: application/json' \
      --data-binary @- \
      "http://localhost:${APP_PORT}/admin/system/initialize")"

  if ! jq -e '.success == true' >/dev/null <<<"$response"; then
    echo "Failed to initialize ${baseline}: $response"
    return 1
  fi
}

sign_in_baseline() {
  local baseline="$1"
  local response

  response="$(jq -n \
    --arg email "stage25-${baseline}@example.com" \
    --arg password "Stage25-${baseline}-Password123" \
    '{email: $email, password: $password}' | \
    curl --fail --silent --show-error \
      -X POST \
      -H 'Content-Type: application/json' \
      --data-binary @- \
      "http://localhost:${APP_PORT}/admin/auth/signin")"

  jq -er '.token' <<<"$response"
}

query_owner_id() {
  local token="$1"
  local payload response

  payload="$(jq -n '{query: "query Stage25Me { me { id } }", operationName: "Stage25Me", variables: {}}')"
  response="$(graphql_request "$token" "$payload")"
  assert_graphql_success "$response" '.data.me.id' >/dev/null
  jq -er '.data.me.id' <<<"$response"
}

query_project_id() {
  local token="$1"
  local payload response

  payload="$(jq -n '{query: "query Stage25Projects { projects(first: 1) { edges { node { id } } } }", operationName: "Stage25Projects", variables: {}}')"
  response="$(graphql_request "$token" "$payload")"
  assert_graphql_success "$response" '.data.projects.edges[0].node.id' >/dev/null
  jq -er '.data.projects.edges[0].node.id' <<<"$response"
}

seed_common_commercial_data() {
  local baseline="$1"
  local token="$2"
  local owner_id="$3"
  local project_id="$4"
  local provider_key="stage25-${baseline}-legacy-provider-key"
  local payload response order_no

  payload="$(jq -n \
    --arg user_id "$owner_id" \
    --arg idempotency_key "stage25-${baseline}-owner-credit" \
    --arg memo "Stage25 ${baseline} owner wallet credit" \
    '{
      query: "mutation Stage25AdjustBalance($input: AdjustUserBalanceInput!) { adjustUserBalance(input: $input) { id status amountMicros } }",
      operationName: "Stage25AdjustBalance",
      variables: {input: {userId: $user_id, direction: "credit", amount: "30.00", currency: "CNY", idempotencyKey: $idempotency_key, memo: $memo}}
    }')"
  response="$(graphql_request "$token" "$payload")"
  assert_graphql_success "$response" '.data.adjustUserBalance.id'

  payload="$(jq -n \
    --arg name "Stage25 ${baseline} ePay" \
    --arg key "$provider_key" \
    --arg base_url "http://localhost:${APP_PORT}" \
    '{
      query: "mutation Stage25SaveProvider($input: UpsertEPayPaymentProviderInput!) { upsertEPayPaymentProvider(input: $input) { id name status } }",
      operationName: "Stage25SaveProvider",
      variables: {input: {name: $name, status: "enabled", currency: "CNY", gatewayUrl: ($base_url + "/payment/simulate/epay/submit"), pid: "stage25-pid", key: $key, notifyUrl: ($base_url + "/payment/notify/epay"), returnUrl: ($base_url + "/billing"), type: "alipay", siteName: "AxonHub Stage 25"}}
    }')"
  response="$(graphql_request "$token" "$payload")"
  assert_graphql_success "$response" '.data.upsertEPayPaymentProvider.id'

  payload="$(jq -n \
    --arg project_id "$project_id" \
    --arg baseline "$baseline" \
    '{
      query: "mutation Stage25CreateManualOrder($input: CreateManualRechargeOrderInput!) { createManualRechargeOrder(input: $input) { id orderNo status amountMicros } }",
      operationName: "Stage25CreateManualOrder",
      variables: {input: {projectId: $project_id, amount: "7.00", currency: "CNY", metadata: {stage: "25", baseline: $baseline}}}
    }')"
  response="$(graphql_request "$token" "$payload")"
  assert_graphql_success "$response" '.data.createManualRechargeOrder.orderNo'
  order_no="$(jq -er '.data.createManualRechargeOrder.orderNo' <<<"$response")"

  payload="$(jq -n \
    --arg order_no "$order_no" \
    --arg event_key "stage25-${baseline}-manual-paid" \
    --arg trade_no "stage25-${baseline}-trade" \
    '{
      query: "mutation Stage25ConfirmManualPayment($input: ConfirmManualPaymentInput!) { confirmManualPayment(input: $input) { id orderNo status amountMicros } }",
      operationName: "Stage25ConfirmManualPayment",
      variables: {input: {orderNo: $order_no, eventKey: $event_key, externalTradeNo: $trade_no}}
    }')"
  response="$(graphql_request "$token" "$payload")"
  assert_graphql_success "$response" '.data.confirmManualPayment.id'

  payload="$(jq -n \
    --arg baseline "$baseline" \
    '{
      query: "mutation Stage25SavePrice($input: SaveBillingPriceRuleForm!) { saveBillingPriceRule(input: $input) { id enabled } }",
      operationName: "Stage25SavePrice",
      variables: {input: {scopeType: "global", scopeId: 0, modelPattern: ("stage25-" + $baseline + "-model"), price: {items: [{itemCode: "prompt_tokens", pricing: {mode: "usage_per_unit", usagePerUnit: "0.03"}}, {itemCode: "completion_tokens", pricing: {mode: "usage_per_unit", usagePerUnit: "0.06"}}]}, currency: "CNY", priority: 25, enabled: true, referenceId: ("stage25-" + $baseline + "-price")}}
    }')"
  response="$(graphql_request "$token" "$payload")"
  assert_graphql_success "$response" '.data.saveBillingPriceRule.id'
}

seed_extended_commercial_data() {
  local baseline="$1"
  local token="$2"
  local payload response redeem_code plan_id

  payload="$(jq -n \
    --arg prefix "S25${baseline}" \
    --arg notes "Stage25 ${baseline} redeem" \
    '{
      query: "mutation Stage25CreateRedeem($input: CreateRedeemCodesInput!) { createRedeemCodes(input: $input) { id code status } }",
      operationName: "Stage25CreateRedeem",
      variables: {input: {count: 1, type: "balance", amount: "3.00", currency: "CNY", prefix: $prefix, notes: $notes}}
    }')"
  response="$(graphql_request "$token" "$payload")"
  assert_graphql_success "$response" '.data.createRedeemCodes[0].code'
  redeem_code="$(jq -er '.data.createRedeemCodes[0].code' <<<"$response")"

  payload="$(jq -n \
    --arg code "$redeem_code" \
    '{
      query: "mutation Stage25Redeem($input: RedeemCodeInput!) { redeemCode(input: $input) { id code status } }",
      operationName: "Stage25Redeem",
      variables: {input: {code: $code}}
    }')"
  response="$(graphql_request "$token" "$payload")"
  assert_graphql_success "$response" '.data.redeemCode.id'

  payload="$(jq -n \
    --arg name "Stage25 ${baseline} Plan" \
    '{
      query: "mutation Stage25SavePlan($input: SaveSubscriptionPlanInput!) { saveSubscriptionPlan(input: $input) { id name status } }",
      operationName: "Stage25SavePlan",
      variables: {input: {name: $name, description: "Stage25 upgrade plan", period: "month", periodDays: 30, price: "5.00", currency: "CNY", includedAmount: "20.00", supportedModelIDs: [], supportedProjectIDs: [], supportedGroupIDs: [], allowWalletFallback: true, status: "enabled", sortOrder: 25}}
    }')"
  response="$(graphql_request "$token" "$payload")"
  assert_graphql_success "$response" '.data.saveSubscriptionPlan.id'
  plan_id="$(jq -er '.data.saveSubscriptionPlan.id' <<<"$response")"

  payload="$(jq -n \
    --arg plan_id "$plan_id" \
    '{
      query: "mutation Stage25PurchasePlan($input: PurchaseSubscriptionPlanInput!) { purchaseSubscriptionPlan(input: $input) { id status includedAmountMicros } }",
      operationName: "Stage25PurchasePlan",
      variables: {input: {planId: $plan_id}}
    }')"
  response="$(graphql_request "$token" "$payload")"
  assert_graphql_success "$response" '.data.purchaseSubscriptionPlan.id'
}

commercial_manifest() {
  local database="$1"

  psql_query "$database" "
    SELECT 'users|' || string_agg(id || ':' || email || ':' || status || ':' || is_owner, ';' ORDER BY id) FROM users;
    SELECT 'projects|' || string_agg(id || ':' || name || ':' || status, ';' ORDER BY id) FROM projects;
    SELECT 'user_projects|' || string_agg(id || ':' || user_id || ':' || project_id || ':' || is_owner || ':' || scopes::text, ';' ORDER BY id) FROM user_projects;
    SELECT 'api_keys|' || string_agg(id || ':' || COALESCE(user_id, 0) || ':' || project_id || ':' || name || ':' || type || ':' || status || ':' || md5(key), ';' ORDER BY id) FROM api_keys;
    SELECT 'billing_price_rules|' || string_agg(id || ':' || scope_type || ':' || scope_id || ':' || model_pattern || ':' || currency || ':' || priority || ':' || enabled || ':' || reference_id || ':' || price::text, ';' ORDER BY id) FROM billing_price_rules;
    SELECT 'billing_accounts|' || string_agg(id || ':' || owner_type || ':' || owner_id || ':' || currency || ':' || balance_micros || ':' || held_balance_micros || ':' || credit_limit_micros || ':' || status, ';' ORDER BY id) FROM billing_accounts;
    SELECT 'ledger_transactions|' || string_agg(id || ':' || billing_account_id || ':' || direction || ':' || amount_micros || ':' || currency || ':' || type || ':' || status || ':' || idempotency_key || ':' || reference_type || ':' || reference_id || ':' || memo, ';' ORDER BY id) FROM ledger_transactions;
    SELECT 'ledger_entries|' || string_agg(id || ':' || ledger_transaction_id || ':' || account_side || ':' || direction || ':' || amount_micros || ':' || currency, ';' ORDER BY id) FROM ledger_entries;
    SELECT 'payment_orders|' || string_agg(id || ':' || order_no || ':' || project_id || ':' || billing_account_id || ':' || provider_type || ':' || purpose || ':' || amount_micros || ':' || currency || ':' || status || ':' || COALESCE(external_trade_no, ''), ';' ORDER BY id) FROM payment_orders;
    SELECT 'payment_events|' || string_agg(id || ':' || event_key || ':' || COALESCE(payment_order_id, 0) || ':' || provider_type || ':' || event_type || ':' || status || ':' || error, ';' ORDER BY id) FROM payment_events;
    SELECT 'payment_providers|' || string_agg(id || ':' || name || ':' || provider_type || ':' || status || ':' || currency, ';' ORDER BY id) FROM payment_provider_instances;
  "

  if table_exists "$database" redeem_codes; then
    psql_query "$database" "SELECT 'redeem_codes|' || string_agg(id || ':' || code || ':' || type || ':' || status || ':' || amount_micros || ':' || currency || ':' || COALESCE(used_by_id, 0), ';' ORDER BY id) FROM redeem_codes"
  fi
  if table_exists "$database" subscription_plans; then
    psql_query "$database" "SELECT 'subscription_plans|' || string_agg(id || ':' || name || ':' || period || ':' || period_days || ':' || price_micros || ':' || currency || ':' || included_amount_micros || ':' || status, ';' ORDER BY id) FROM subscription_plans"
  fi
  if table_exists "$database" user_subscriptions; then
    psql_query "$database" "SELECT 'user_subscriptions|' || string_agg(id || ':' || user_id || ':' || COALESCE(plan_id, 0) || ':' || status || ':' || period_days || ':' || included_amount_micros || ':' || used_amount_micros || ':' || currency, ';' ORDER BY id) FROM user_subscriptions"
  fi
}

verify_current_tables() {
  local database="$1"
  local missing

  missing="$(psql_query "$database" "
    WITH expected(name) AS (
      VALUES
        ('billing_accounts'),
        ('ledger_transactions'),
        ('ledger_entries'),
        ('payment_orders'),
        ('payment_events'),
        ('payment_provider_instances'),
        ('redeem_codes'),
        ('subscription_plans'),
        ('user_subscriptions'),
        ('usage_billing_records'),
        ('usage_hourly_aggregates'),
        ('usage_daily_aggregates'),
        ('upstream_account_pools'),
        ('upstream_accounts')
    )
    SELECT name FROM expected WHERE to_regclass('public.' || name) IS NULL ORDER BY name;
  ")"

  if [[ -n "$missing" ]]; then
    echo "Missing current tables in ${database}:"
    echo "$missing"
    return 1
  fi
}

verify_provider_encrypted() {
  local baseline="$1"
  local database="$2"
  local plaintext="stage25-${baseline}-legacy-provider-key"
  local config

  config="$(psql_query "$database" "SELECT config::text FROM payment_provider_instances WHERE name = 'Stage25 ${baseline} ePay'")"
  if [[ "$config" == *"$plaintext"* ]]; then
    echo "Legacy provider key remained plaintext after ${baseline} upgrade."
    return 1
  fi
  if [[ "$config" != *"enc:v1:"* ]]; then
    echo "Migrated provider config for ${baseline} is not encrypted."
    return 1
  fi
}

build_baseline() {
  local baseline="$1"
  local commit="$2"
  local source_dir="$TEMP_DIR/${baseline}-source"
  local binary="$TEMP_DIR/axonhub-${baseline}"

  mkdir -p "$source_dir"
  git archive "$commit" | tar -x -C "$source_dir"
  pushd "$source_dir" >/dev/null
  go build -o "$binary" ./cmd/axonhub
  popd >/dev/null
  printf '%s\n' "$binary"
}

run_baseline_upgrade() {
  local baseline="$1"
  local commit="$2"
  local feature_set="$3"
  local database="axonhub_${baseline}"
  local baseline_binary token owner_id project_id
  local manifest_before manifest_after manifest_second

  echo "Creating PostgreSQL database for ${baseline} (${commit})..."
  docker exec "$POSTGRES_CONTAINER" createdb -U "$POSTGRES_USER" "$database"

  echo "Building ${baseline} baseline..."
  baseline_binary="$(build_baseline "$baseline" "$commit")"

  echo "Initializing and seeding ${baseline} through its own business APIs..."
  start_server "$baseline_binary" "$database"
  initialize_baseline "$baseline"
  token="$(sign_in_baseline "$baseline")"
  owner_id="$(query_owner_id "$token")"
  project_id="$(query_project_id "$token")"
  seed_common_commercial_data "$baseline" "$token" "$owner_id" "$project_id"
  if [[ "$feature_set" == "extended" ]]; then
    seed_extended_commercial_data "$baseline" "$token"
  fi
  manifest_before="$(commercial_manifest "$database")"
  stop_server

  echo "Upgrading ${baseline} to the current fork..."
  start_server "$CURRENT_BINARY" "$database"
  verify_current_tables "$database"
  verify_provider_encrypted "$baseline" "$database"
  manifest_after="$(commercial_manifest "$database")"
  stop_server

  if [[ "$manifest_before" != "$manifest_after" ]]; then
    echo "Commercial manifest changed during ${baseline} upgrade."
    diff -u <(printf '%s\n' "$manifest_before") <(printf '%s\n' "$manifest_after") || true
    return 1
  fi

  echo "Restarting current fork to verify ${baseline} migration idempotency..."
  start_server "$CURRENT_BINARY" "$database"
  manifest_second="$(commercial_manifest "$database")"
  stop_server

  if [[ "$manifest_after" != "$manifest_second" ]]; then
    echo "Commercial manifest changed on the second ${baseline} startup."
    diff -u <(printf '%s\n' "$manifest_after") <(printf '%s\n' "$manifest_second") || true
    return 1
  fi

  echo "${baseline} commercial upgrade acceptance passed."
}

trap cleanup EXIT

cd "$PROJECT_ROOT"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for the commercial upgrade matrix."
  exit 1
fi

if curl --fail --silent "http://localhost:${APP_PORT}/health" >/dev/null 2>&1; then
  echo "Port ${APP_PORT} is already serving an AxonHub health endpoint."
  exit 1
fi

echo "Starting isolated PostgreSQL for the commercial upgrade matrix..."
docker rm -f "$POSTGRES_CONTAINER" >/dev/null 2>&1 || true
docker run -d \
  --name "$POSTGRES_CONTAINER" \
  -e POSTGRES_DB=postgres \
  -e POSTGRES_USER="$POSTGRES_USER" \
  -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
  -p "${POSTGRES_PORT}:5432" \
  postgres:15-alpine >/dev/null
wait_for_postgres 30

echo "Building the current AxonHub fork..."
go build -o "$CURRENT_BINARY" ./cmd/axonhub

for entry in "${BASELINES[@]}"; do
  IFS=: read -r baseline commit feature_set <<<"$entry"
  run_baseline_upgrade "$baseline" "$commit" "$feature_set"
done

echo "Commercial PostgreSQL upgrade matrix passed for all baselines."
