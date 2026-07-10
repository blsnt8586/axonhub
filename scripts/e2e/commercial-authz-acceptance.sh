#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

echo "Running commercial GraphQL authorization and tenant-isolation acceptance..."
go test ./internal/server/gql \
  -run 'TestCommercial|TestBillingResolvers(UserBillingQueriesAreScopedToCurrentUser|RejectsUserBillingAdminOperationsForNonOwner|UserCanCreateMyEPayRechargeCheckout|RejectsMyEPayCheckoutForUnrelatedProject)' \
  -count=1

echo "Running complete GraphQL, API, and commercial service regressions..."
go test ./internal/server/gql ./internal/server/api ./internal/server/biz -count=1

echo "Commercial GraphQL authorization acceptance passed."
