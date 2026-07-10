#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

echo "Running commercial self-service backend closure tests..."
go test ./internal/server/gql -run 'TestCommercialSelfService' -count=1
go test ./internal/server/gql ./internal/server/api ./internal/server/biz -count=1

echo "Checking and building the commercial frontend..."
cd "$PROJECT_ROOT/frontend"
pnpm exec tsc --noEmit
pnpm build

echo "Building the current backend for browser acceptance..."
cd "$PROJECT_ROOT"
go build -o scripts/e2e/axonhub-e2e ./cmd/axonhub

echo "Running the browser promo, subscription, payment, and affiliate closure..."
./scripts/e2e/e2e-test.sh commercial-smoke.spec.ts

echo "Commercial self-service acceptance passed."
