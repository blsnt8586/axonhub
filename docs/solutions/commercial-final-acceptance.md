# Commercial Operations Final Acceptance

Date: 2026-07-09

This document is the Stage 18 acceptance record for the commercial AxonHub fork.
It verifies the added commercial modules as one product flow instead of only as
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

## Manual Browser Smoke Checklist

These are acceptance steps for a running environment. They are intentionally
listed as manual checks until a dedicated browser-smoke stage adds robust seeded
Playwright fixtures for commercial flows.

User flow:

- Register a new account from the public sign-up page.
- Sign in with the registered email/password.
- Confirm the wallet page shows balance, held amount, credit limit, and
  available amount.
- Create a simulated ePay recharge checkout and complete the simulated notify
  path.
- Redeem a valid redeem code.
- Create or inspect the default API key.
- Call the API with the key and confirm wallet/usage/billing data updates.
- Purchase a subscription plan and confirm included quota is consumed before
  wallet fallback.

Owner flow:

- Sign in as owner.
- Inspect users, billing accounts, ledgers, usage charges, payment orders,
  payment events, price rules, providers, reports, subscriptions, redeem codes,
  account pools, switch history, and account health.
- Save commercial operation settings with a reason.
- Run maintenance manually and confirm the audit log row appears.
- Verify ePay and upstream account secrets are write-only in the browser.

Responsive smoke:

- Repeat the wallet and owner monitoring views at desktop width.
- Repeat the same key views at a mobile width and confirm forms, tables, and
  dialogs remain usable without overlapping controls.

## Deployment Acceptance

SQLite development mode:

- Covered by the final lifecycle test and focused service tests, all using
  SQLite in-memory databases with migrations.
- A running single-node SQLite smoke remains part of the production checklist.

Docker/PostgreSQL:

- Must still be run before production enablement because Stage 18 did not start
  a PostgreSQL-backed Docker stack.
- Required command shape:

```bash
docker compose up --build
go test ./internal/server/... -count=1
```

Use the project-specific production `.env` and verify that
`AXONHUB_PAYMENT_SECRET_KEY` is set before saving real payment providers.

## Residual Production Risks

- Browser commercial smoke is documented but not yet automated. The existing
  Playwright suite covers core admin surfaces, but not the complete user wallet
  and payment loop.
- Full frontend lint is not yet a clean release gate. Typecheck and production
  build pass, but repository-wide lint needs a separate cleanup stage.
- Docker/PostgreSQL startup and backup/restore are still deployment acceptance
  tasks, not unit-test evidence.
- Frontend production build may keep Vite's large chunk warning. This is not a
  functional failure, but should be reviewed before high-traffic deployment.
- Logs must be reviewed in a real running environment with production log
  sinks; unit tests only verify that persisted provider config does not contain
  payment secrets in plaintext.

## Recommended Release Gate

Do not enable `billing.mode=enforce` in production until all of the following
are true:

- Backend focused tests pass.
- Frontend typecheck and production build pass.
- Docker/PostgreSQL stack starts and healthcheck passes.
- Manual browser smoke checklist passes for both owner and normal user.
- Log sampling confirms no payment secrets, upstream account secrets, or full
  API keys are emitted.
- A database backup and rollback point exist.
