#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

echo "Running gateway commercial lifecycle acceptance..."
go test ./internal/server/orchestrator \
  -run 'TestGatewayCommercialLifecycle|TestBillingAdmission|TestChatCompletionOrchestrator_Process_SameChannelRetryNextModel' \
  -count=1

echo "Running billing domain regression acceptance..."
go test ./internal/server/biz \
  -run 'TestAdmission|TestBillingHold|TestBillingOutbox|TestPricingService|TestSubscription|TestExpiredSubscription|TestUsageBillingProcessor' \
  -count=1

echo "Running complete orchestrator and billing service package tests..."
go test ./internal/server/orchestrator ./internal/server/biz -count=1

echo "Commercial gateway acceptance passed."
