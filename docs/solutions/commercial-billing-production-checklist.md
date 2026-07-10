# Commercial Billing Production Checklist

This checklist tracks Stage 11 production deployment and end-to-end acceptance for the AxonHub commercial billing fork.

## Business Contract

- [x] User wallet is the default payer.
- [x] Project-level price rules affect charge calculation, not wallet ownership.
- [x] Balance changes go through ledger transactions instead of direct balance mutation.
- [x] Payment return pages are read-only; wallet credit requires verified async notification or explicit admin make-up.

## Configuration

- [x] Default `billing.subject` is `user`.
- [ ] Production config sets `billing.mode` deliberately: `disabled`, `warn`, or `enforce`.
- [x] Stage 11 runtime sets `AXONHUB_PAYMENT_SECRET_KEY` before saving encrypted provider secrets.
- [ ] Database auto-migration strategy is decided before first production deployment.
- [x] Local dev public URL and callback URLs are aligned for the Stage 11 simulated ePay provider.
- [x] ePay-compatible provider credentials are configured with write-only secrets in the browser.
- [x] Commercial maintenance worker switches are explicitly reviewed in the admin console.

## Database And Migration

- [x] Fresh PostgreSQL database can auto-migrate all commercial and upstream-account tables.
- [x] Pre-commercial PostgreSQL database can migrate without losing owner, user, project, or system records.
- [x] Stage 1, Stage 2, Stage 6, and Stage 11 commercial databases migrate without losing wallet, ledger, order, provider, redeem, or subscription records.
- [x] Legacy plaintext ePay provider keys are encrypted transactionally during current-version startup.
- [ ] Production-scale database upgrade is rehearsed with the deployment dataset and measured downtime.
- [ ] Rollback point is tagged before enabling `warn` or `enforce` billing mode.
- [x] PostgreSQL backup and restore procedure is tested with commercial tables included.

## Owner Acceptance

- [x] Owner can sign in after deployment.
- [x] Owner can open the admin billing console.
- [x] Owner can view wallet accounts, ledger rows, usage charges, payment orders, payment events, price rules, providers, reports, subscriptions, redeem codes, promo codes, affiliate rules, notifications, and operations.
- [x] Owner can save commercial operation settings.
- [x] Owner can run commercial maintenance manually and see an audit log row.
- [x] Owner can save an ePay-compatible provider without the secret being returned to the browser.

## User Acceptance

- [x] User can open the wallet page and see balance, held amount, credit limit, and available amount.
- [x] User can create a recharge checkout.
- [x] Simulated ePay notification credits the user wallet once and stays idempotent on retry.
- [x] User can redeem a valid redeem code.
- [ ] User can apply a valid promo code to recharge or subscription checkout.
- [ ] User can bind an affiliate invite code once.
- [x] User can buy a subscription plan.
- [ ] Subscription quota is consumed before wallet fallback where applicable.

## Gateway Acceptance

- [ ] API key creation supports commercial limits.
- [ ] Request admission uses user wallet by default.
- [ ] Project price rules affect the computed charge.
- [ ] Insufficient user balance is rejected in `enforce` mode.
- [ ] `warn` mode records billing failures without blocking upstream routing.
- [ ] Failed usage billing outbox rows can be retried after balance or price-rule repair.
- [ ] Existing AxonHub scheduling and circuit-breaker behavior remains intact.

## Frontend Verification

- [x] `pnpm exec tsc --noEmit` passes.
- [x] `pnpm build` passes.
- [x] Browser login flow works on the dev server.
- [x] Admin billing operation controls render without console-blocking errors.
- [x] User billing page renders without console-blocking errors.
- [x] Responsive layout is usable at desktop and mobile widths for billing and upstream-account monitoring.

## Backend Verification

- [x] `go test ./conf ./internal/server/biz ./internal/server/gql ./internal/server/api -count=1` passes.
- [x] `go test ./internal/server/orchestrator -run BillingAdmission -count=1` passes.
- [x] Health endpoint returns success while the app is running.
- [ ] GraphQL billing queries and mutations return expected authorization failures for non-owner users.

## Deployment Verification

- [x] Current-fork Docker image starts with PostgreSQL-backed `docker-compose`.
- [x] SQLite single-node deployment starts with a dedicated Stage 11 data file.
- [x] Healthcheck is wired for container orchestration.
- [x] AxonHub application logs do not contain provider secrets, payment notify secrets, or full API keys.
- [ ] Reverse-proxy, container-runtime, and external log-collector samples do not contain secrets.
- [ ] Large frontend chunk warning is reviewed and accepted or split before high-traffic production.

## Stage 11 Acceptance Defects Resolved

- [x] Fixed commercial GraphQL resolver panics for billing notification/settings/hold IDs and related foreign-key GUID fields.
- [x] Fixed Vite dev proxy so `/admin/billing` remains a frontend SPA route instead of being proxied to backend `/admin`.
- [x] Fixed concurrent first-load user wallet creation so multiple billing queries do not race into SQLite table locks.
- [x] Fixed public ePay return/notify/simulated checkout handlers so they do not require a logged-in user context.
- [x] Fixed local simulated ePay checkout so custom admin-configured provider keys are used for generated notify signatures.
