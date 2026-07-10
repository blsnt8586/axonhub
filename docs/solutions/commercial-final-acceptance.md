# Commercial Operations Final Acceptance

Date: 2026-07-09

This document is the Stage 18 acceptance record for the commercial AxonHub fork,
updated with Stage 19-24 browser-smoke, Docker/PostgreSQL, recovery, log
redaction, and responsive evidence. It
verifies the added commercial modules as one product flow instead of only as
isolated services.

## Acceptance Scope

Stage 18 covers the production-facing commercial loop:

- Public registration and password login.
- User wallet recharge through simulated ePay notification.
- Redeem-code credit.
- Subscription purchase and subscription quota consumption.
- User-scoped API key commercial limits and billing admission.
- Usage billing through user wallet, with project price rules affecting price
  calculation only.
- Upstream account usage attribution, switch history, and monitoring summary.
- Commercial maintenance and aggregate rebuild idempotency.
- Frontend build/type safety and known manual smoke points.

## Automated Evidence

| Area | Evidence | What It Proves |
| --- | --- | --- |
| Coherent backend commercial lifecycle | `go test ./internal/server/biz -run TestCommercialOperationsFinalAcceptanceUserCommercialLifecycle -count=1` | Registration, login, user wallet recharge, redeem, subscription purchase, admission, usage billing, account monitoring, switch history, encrypted ePay config, and maintenance rebuild work in one SQLite database. |
| Registration | `internal/server/biz/registration_test.go` | Registration can be disabled/enabled, creates activated/deactivated users, creates a user billing account, grants signup balance, creates default project/API key, audits, and rate-limits. |
| Payment | `internal/server/biz/payment_test.go` | Manual recharge and simulated ePay checkout/notify are idempotent; invalid signatures, money mismatch, late notify, provider encryption, and reserved simulation names are handled. |
| Redeem codes | `internal/server/biz/redeem_code_test.go` | Redeem codes credit user wallets once, reject invalid states, support admin create/redeem audit, and maintain ledger references. |
| Subscriptions | `internal/server/biz/subscription_test.go` | Purchases debit user wallet, active quota skips wallet charge, expired subscriptions fall back to wallet, and reset/expiry workers are repeatable. |
| API key commercial limits | `internal/server/biz/api_key_commercial_limits_test.go` | Total/daily/monthly/single-request limits are checked before request admission and before usage billing. |
| Billing admission | `internal/server/orchestrator/billing_admission_test.go` | Gateway admission denies insufficient billing state in enforce mode and preserves non-user/no-service behavior. |
| Usage billing and aggregates | `internal/server/biz/usage_billing_test.go`, `internal/server/biz/usage_aggregate_test.go` | Usage records are charged once, project price rules override global pricing, outbox retry is repeatable, aggregates rebuild deterministically, and upstream account dimensions propagate. |
| GraphQL authorization | `internal/server/gql/*_test.go` | Owner/user resolver boundaries and secret redaction are covered for billing and upstream account surfaces. |
| Commercial browser smoke | `./scripts/e2e/e2e-test.sh commercial-smoke.spec.ts` | Owner billing console entry, public registration, user wallet rendering, redeem-code redemption, subscription purchase, and simulated ePay recharge are covered in Playwright. |
| Account-pool browser smoke | `./scripts/e2e/e2e-test.sh upstream-accounts-smoke.spec.ts` | Channel account-pool dialog, pool creation, write-only account credential creation, monitoring list, and account detail views are covered in Playwright. |
| PostgreSQL and Docker acceptance | `./scripts/e2e/commercial-postgres-acceptance.sh` | Both browser smoke suites pass against PostgreSQL, commercial/account-pool data is persisted, an application restart retains database health, the current fork image builds, Compose services become healthy, and fresh-database migrations create the required tables. |
| Historical migration and recovery | `./scripts/e2e/commercial-postgres-recovery-acceptance.sh` | A pre-commercial PostgreSQL database preserves legacy records during current-fork migration; the upgraded commercial database survives a real dump, drop, recreate, and restore cycle; and the restored user can read wallet, paid-order, and active-subscription state in the browser. |
| Runtime log redaction | `./scripts/e2e/commercial-log-redaction-acceptance.sh` | Debug application logs redact payment/provider secrets, JWTs, full service API keys, authorization headers, cookies, body credentials, DSN passwords, and query tokens while preserving safe diagnostic fields. |
| Desktop/mobile responsive acceptance | `./scripts/e2e/commercial-responsive-acceptance.sh` | User billing, owner billing operations, upstream-account monitoring, and account detail pages pass desktop/mobile overflow and control-overlap checks with eight full-page screenshot attachments. |
| Frontend compilation | `pnpm exec tsc --noEmit`, `pnpm build` from `frontend/` | The commercial UI remains type-safe and production-buildable. |

`pnpm lint` was also run during Stage 18. It failed on existing repository-wide
frontend lint debt, including unused variables, empty catch blocks,
`@ts-ignore` usage, hook ordering, and console statements in unrelated files.
No Stage 18 frontend source files were changed.

## Final Lifecycle Assertions

`TestCommercialOperationsFinalAcceptanceUserCommercialLifecycle` intentionally
asserts the business contract that was easy to regress during earlier stages:

- A registered user receives a `user` billing account; the default project and
  API key belong to that user.
- User self-recharge creates an order attached to the project but credits the
  user billing account.
- ePay provider config does not persist the simulated notify key in plaintext.
- Redeem code credit, subscription purchase, subscription coverage, and wallet
  fallback all update the same user wallet.
- API key commercial limits are checked alongside the user wallet and
  subscription quota.
- Usage billing records include upstream account IDs when routed through an
  account pool.
- Account switch history is visible in monitoring detail.
- Aggregate rebuild can be run repeatedly through commercial maintenance with
  stable row counts.

## Browser Smoke Checklist

Stage 19 adds automated Playwright coverage for the core browser loop:

- Owner opens the commercial billing console and core commercial tabs render.
- Public registration creates a normal user.
- The registered user signs in through API-backed credentials and opens the
  wallet page.
- A seeded redeem code can be redeemed from the browser.
- A seeded subscription plan can be purchased from the browser.
- A simulated ePay recharge returns through the payment callback and the wallet
  page shows a paid order.

Stage 20 adds automated Playwright coverage for the upstream account-pool
operator loop:

- Owner seeds a dedicated Channel and opens the upstream-account dialog.
- Owner creates an account pool with model targeting.
- Owner creates an upstream account whose credential remains write-only in the
  browser.
- Owner opens the account monitoring page and sees the created account.
- Owner opens the single-account detail page and sees quota/cooldown, recent
  executions, and switch-history sections.

The remaining manual checks are broader release checks that need real provider
configuration, API traffic, and responsive review.

User flow:

- Create or inspect the default API key.
- Call the API with the key and confirm wallet/usage/billing data updates.
- Confirm subscription included quota is consumed before wallet fallback under a
  real API request path.

Owner flow:

- Inspect users, billing accounts, ledgers, usage charges, payment orders,
  payment events, price rules, providers, reports, subscriptions, redeem codes,
  account pools, switch history, and account health.
- Save commercial operation settings with a reason.
- Run maintenance manually and confirm the audit log row appears.
- Verify ePay and upstream account secrets are write-only in the browser.

Responsive smoke is automated for user billing, owner billing Operations,
upstream-account monitoring, and account detail at desktop and mobile widths.
Broader device/browser coverage remains a release-specific decision.

## Deployment Acceptance

SQLite development mode:

- Covered by the final lifecycle test and focused service tests, all using
  SQLite in-memory databases with migrations.
- A running single-node SQLite smoke remains part of the production checklist.

Docker/PostgreSQL:

- Stage 21 adds repeatable acceptance for a fresh PostgreSQL database and the
  current fork's production Docker image.
- Run the complete isolated acceptance with:

```bash
./scripts/e2e/commercial-postgres-acceptance.sh
```

The script runs browser smoke against PostgreSQL, verifies persisted commercial
and account-pool rows, restarts the application against the retained database,
builds `axonhub:local`, starts an isolated Compose stack, waits for healthchecks,
and verifies all required fresh-database migrations. It cleans up only its own
test containers, network, and volume.

Stage 22 adds historical upgrade and disaster-recovery acceptance:

```bash
./scripts/e2e/commercial-postgres-recovery-acceptance.sh
```

This script initializes the pre-commercial `26584ccf` schema, upgrades it with
the current fork, runs the commercial browser lifecycle, creates a real
PostgreSQL dump, drops and recreates the database, restores it, compares core
records, and runs a restored-user browser smoke. The dump is also checked for
the simulated ePay secret in plaintext.

Use the project-specific production `.env` and verify that
`AXONHUB_PAYMENT_SECRET_KEY` is set before saving real payment providers.

## Residual Production Risks

- Browser commercial smoke is automated for the core owner/user billing loop,
  owner account-pool management/monitoring, and desktop/mobile responsive
  layouts. Deeper real API-traffic and production-provider checks remain manual
  release tasks.
- Full frontend lint is not yet a clean release gate. Typecheck and production
  build pass, but repository-wide lint needs a separate cleanup stage.
- Fresh PostgreSQL startup, current-fork Docker builds, Compose healthchecks,
  pre-commercial database upgrade, backup/restore, migration coverage, and
  application restart persistence are automated. Upgrade testing from an older
  commercial fork snapshot and a production-scale backup/restore drill remain
  deployment acceptance tasks.
- Frontend production build may keep Vite's large chunk warning. This is not a
  functional failure, but should be reviewed before high-traffic deployment.
- Application log redaction is covered with debug-level runtime canaries, and
  Stage 22 verifies that the simulated ePay secret does not appear in a plain
  PostgreSQL dump. Production reverse proxies, container runtimes, and external
  log collectors still require sampling because their logs do not pass through
  AxonHub's redacting logger core.

## Recommended Release Gate

Do not enable `billing.mode=enforce` in production until all of the following
are true:

- Backend focused tests pass.
- Frontend typecheck and production build pass.
- Stage 21 Docker/PostgreSQL acceptance passes for the release commit.
- Automated commercial and account-pool browser smoke passes, and remaining
  manual browser checklist items pass for real API traffic and responsive
  layouts.
- Automated AxonHub log-redaction acceptance passes, and production external
  log-sink sampling confirms no secrets are emitted outside the application
  logger.
- A database backup and rollback point exist.
