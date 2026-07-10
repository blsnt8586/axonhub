#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT/frontend"

echo "Checking responsive smoke TypeScript..."
pnpm exec tsc --noEmit

echo "Building the production frontend..."
pnpm build

cd "$PROJECT_ROOT"

echo "Running desktop and mobile commercial responsive smoke..."
./scripts/e2e/e2e-test.sh commercial-responsive-smoke.spec.ts

echo "Commercial responsive acceptance passed."
