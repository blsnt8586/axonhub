#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEMP_FILE="$(mktemp "${TMPDIR:-/tmp}/axonhub-commercial-compose.XXXXXX.json")"

cleanup() {
  rm -f "$TEMP_FILE"
}

trap cleanup EXIT
cd "$PROJECT_ROOT"

docker compose --env-file deploy/commercial.env.example config --format json > "$TEMP_FILE"

jq -e '
  .services.axonhub.environment.AXONHUB_BILLING_MODE == "warn" and
  .services.axonhub.environment.AXONHUB_BILLING_SUBJECT == "user" and
  .services.axonhub.environment.AXONHUB_BILLING_BLOCK_WHEN_NO_PRICE_RULE == "true" and
  .services.axonhub.environment.AXONHUB_DB_DISABLE_AUTO_MIGRATION == "false" and
  .services.axonhub.environment.AXONHUB_PAYMENT_SECRET_KEY != "" and
  .services.axonhub.ports[0].host_ip == "127.0.0.1" and
  .services.postgres.ports[0].host_ip == "127.0.0.1"
' "$TEMP_FILE" >/dev/null

echo "Commercial deployment configuration test passed."
