#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELEASE_GATE="$SCRIPT_DIR/commercial-release-gate.sh"

assert_contains() {
  local output="$1"
  local expected="$2"

  if [[ "$output" != *"$expected"* ]]; then
    echo "Expected release-gate output to contain: $expected" >&2
    exit 1
  fi
}

assert_not_contains() {
  local output="$1"
  local unexpected="$2"

  if [[ "$output" == *"$unexpected"* ]]; then
    echo "Expected release-gate output not to contain: $unexpected" >&2
    exit 1
  fi
}

bash -n "$RELEASE_GATE"

quick_plan="$($RELEASE_GATE --quick --plan)"
assert_contains "$quick_plan" "Commercial release gate mode: quick"
assert_contains "$quick_plan" "Core Go regression"
assert_contains "$quick_plan" "GraphQL authorization and tenant isolation"
assert_contains "$quick_plan" "Gateway billing lifecycle"
assert_contains "$quick_plan" "Promo, payment, subscription, and affiliate self-service"
assert_not_contains "$quick_plan" "Fresh PostgreSQL and Docker deployment"

full_plan="$($RELEASE_GATE --full --plan)"
assert_contains "$full_plan" "Commercial release gate mode: full"
assert_contains "$full_plan" "GraphQL authorization and tenant isolation"
assert_contains "$full_plan" "Responsive browser acceptance"
assert_contains "$full_plan" "Runtime log redaction"
assert_contains "$full_plan" "Fresh PostgreSQL and Docker deployment"
assert_contains "$full_plan" "PostgreSQL backup and recovery"
assert_contains "$full_plan" "Historical PostgreSQL upgrade matrix"

if "$RELEASE_GATE" --unknown >/dev/null 2>&1; then
  echo "Unknown release-gate options must fail." >&2
  exit 1
fi

echo "Commercial release gate contract tests passed."
