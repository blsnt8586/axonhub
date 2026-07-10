#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

echo "Running user workspace, membership, and suspended-key backend contracts..."
go test ./internal/server/biz \
  -run 'TestAuthService_AuthenticateAPIKeyRejectsDeactivatedUserOwnedKeys|TestUserWorkspaceService' \
  -count=1

echo "Checking and building the commercial frontend..."
cd "$PROJECT_ROOT/frontend"
pnpm exec tsc --noEmit
pnpm build

echo "Building the current backend for the release role matrix..."
cd "$PROJECT_ROOT"
go build -o scripts/e2e/axonhub-e2e ./cmd/axonhub

echo "Running owner, existing-member, new-user, suspended-user, and no-channel browser acceptance..."
./scripts/e2e/e2e-test.sh commercial-release-roles.spec.ts

echo "Commercial user workspace release acceptance passed."
