#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MODE="quick"
PLAN_ONLY=false

usage() {
  cat <<'EOF'
Usage: ./scripts/e2e/commercial-release-gate.sh [--quick|--full] [--plan]

  --quick  Run core backend, authorization, gateway, and self-service browser checks.
  --full   Run the quick gate plus responsive, redaction, PostgreSQL, recovery,
           and historical upgrade acceptance.
  --plan   Print the selected checks without executing them.
EOF
}

while (($# > 0)); do
  case "$1" in
    --quick)
      MODE="quick"
      ;;
    --full)
      MODE="full"
      ;;
    --plan)
      PLAN_ONLY=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

declare -a STEP_NAMES=()
declare -a STEP_COMMANDS=()

add_step() {
  STEP_NAMES+=("$1")
  STEP_COMMANDS+=("$2")
}

add_step "Core Go regression" \
  "go test ./conf ./internal/server/biz ./internal/server/gql ./internal/server/api ./internal/server/orchestrator -count=1"
add_step "Production Compose configuration" \
  "./scripts/e2e/commercial-deployment-config-test.sh"
add_step "GraphQL authorization and tenant isolation" \
  "./scripts/e2e/commercial-authz-acceptance.sh"
add_step "Gateway billing lifecycle" \
  "./scripts/e2e/commercial-gateway-acceptance.sh"
add_step "Promo, payment, subscription, and affiliate self-service" \
  "./scripts/e2e/commercial-self-service-acceptance.sh"

if [[ "$MODE" == "full" ]]; then
  add_step "Responsive browser acceptance" \
    "./scripts/e2e/commercial-responsive-acceptance.sh"
  add_step "Runtime log redaction" \
    "./scripts/e2e/commercial-log-redaction-acceptance.sh"
  add_step "Fresh PostgreSQL and Docker deployment" \
    "./scripts/e2e/commercial-postgres-acceptance.sh"
  add_step "PostgreSQL backup and recovery" \
    "./scripts/e2e/commercial-postgres-recovery-acceptance.sh"
  add_step "Historical PostgreSQL upgrade matrix" \
    "./scripts/e2e/commercial-postgres-upgrade-matrix.sh"
fi

echo "Commercial release gate mode: $MODE"
echo "Checks: ${#STEP_NAMES[@]}"
for i in "${!STEP_NAMES[@]}"; do
  printf '  %d. %s\n' "$((i + 1))" "${STEP_NAMES[$i]}"
  printf '     %s\n' "${STEP_COMMANDS[$i]}"
done

if [[ "$PLAN_ONLY" == true ]]; then
  exit 0
fi

required_commands=(go pnpm docker curl jq git)
for command_name in "${required_commands[@]}"; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Required command is not installed: $command_name" >&2
    exit 1
  fi
done

if ! docker info >/dev/null 2>&1; then
  echo "Docker is not available to the current user." >&2
  exit 1
fi

cd "$PROJECT_ROOT"
gate_started_at=$SECONDS

for i in "${!STEP_NAMES[@]}"; do
  step_started_at=$SECONDS
  echo
  echo "[$((i + 1))/${#STEP_NAMES[@]}] ${STEP_NAMES[$i]}"
  bash -lc "${STEP_COMMANDS[$i]}"
  echo "Completed in $((SECONDS - step_started_at))s: ${STEP_NAMES[$i]}"
done

echo
echo "Commercial release gate ($MODE) passed in $((SECONDS - gate_started_at))s."
