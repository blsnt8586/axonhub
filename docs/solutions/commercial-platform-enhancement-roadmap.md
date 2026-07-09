# AxonHub Commercial Platform Enhancement Roadmap

This document tracks the next commercial-platform enhancement stages after
`commercial-billing-roadmap.md` Stage 11. The intent is additive: keep AxonHub's
existing routing, scheduling, circuit breaker, tracing, wallet, ledger, payment,
subscription, promo, affiliate, notification, and audit modules intact, then
learn from `new-api` and `sub2api` where they are stronger.

## Logic Review

- AxonHub must not be rewritten into `new-api` or `sub2api`.
- AxonHub's current Channel and Orchestrator path stays the default runtime path.
- The user wallet remains the payer. Project-level price rules affect charge
  calculation, not wallet ownership.
- Existing wallet, ledger, payment order, redeem code, subscription, promo,
  affiliate, notification, and audit features are already covered by Stage 0-10
  in `commercial-billing-roadmap.md`; this roadmap must not duplicate them.
- `new-api` is useful for public registration, user/account operation views,
  token-style limits, and hourly usage aggregation.
- `sub2api` is useful for upstream account-pool modeling, account-level health,
  account-level usage, schedulability state, and retry/switch visibility.
- Account pools are an optional Channel enhancement. Channels without account
  pools must behave exactly as they do today.
- Every stage must include backend, frontend, tests, documentation updates, and
  one rollback-friendly commit.

## Current Baseline

- [x] Wallet, ledger, payment orders, simulated ePay, user recharge, sell price
  rules, admin billing console, and user wallet pages exist.
- [x] Billing admission, wallet holds, usage billing records, commercial reports,
  API key commercial limits, redeem codes, subscriptions, promo codes, affiliate
  rebates, billing notifications, and commercial audit logs exist.
- [x] AxonHub has strong Channel, Orchestrator, Trace, Request, UsageLog,
  ProviderQuotaStatus, and circuit-breaker foundations.
- [x] Public registration is a real product flow with backend registration
  settings, public sign-up, user wallet initialization, optional default
  project/API key creation, and browser smoke coverage.
- [ ] Upstream account pools are not first-class resources yet. Channel
  credentials and disabled API keys exist, but they are not equivalent to a
  schedulable account pool with per-account health, quota, proxy, and switch
  history.
- [ ] Long-range usage analytics still need production-grade aggregation tables
  instead of depending only on detail rows and ad hoc queries.

## Stage 12: Public Registration And Onboarding

Status: [x] Completed

Goal: turn the existing `/sign-up` page into a real self-service registration
flow while keeping owner/admin creation separate.

Backend scope:

- [x] Add registration settings: enabled, require approval, default user status,
  default project policy, optional default API key creation, optional signup
  wallet grant, and rate-limit settings.
- [x] Add a public registration API that creates only normal users. It must reuse
  existing password hashing and user creation logic, but it must not accept
  owner/admin role input from the public request.
- [x] Create or initialize the user billing account during registration.
- [x] Optionally create a default project and default user API key according to
  settings.
- [x] Enforce duplicate email checks, password strength, request rate limits,
  and registration-disabled behavior.
- [x] Add audit or system log records for successful registration and rejected
  registration attempts where useful.

Frontend scope:

- [x] Replace the current sign-up form placeholder behavior with a real API call.
- [x] Show registration-disabled, pending-approval, success, and validation
  states.
- [x] After successful registration, redirect to sign-in with a clear success
  message.
- [x] Add owner settings controls for registration policy.

Verification:

- [x] Registration disabled rejects public registration.
- [x] Public registration cannot create owner/admin users.
- [x] Duplicate email fails deterministically.
- [x] Successful registration creates user and billing account.
- [x] Optional default project/API key creation follows settings.
- [x] Frontend typecheck and build pass.

## Stage 13: User Account Profile And Usage Console

Status: [x] Completed

Goal: provide `new-api`-style user commercial visibility without changing the
existing AxonHub wallet or billing source of truth.

Backend scope:

- [x] Add user account summary queries for balance, held amount, available amount,
  credit limit, total recharge, total consumption, today's consumption, current
  month consumption, request count, failure count, top models, top projects, and
  top API keys.
- [x] Build summaries from existing BillingAccount, LedgerTransaction,
  UsageBillingRecord, Request, and UsageLog data.
- [x] Enforce access rules: users can see only themselves; owner can inspect any
  user.
- [x] Add filters by time range, project, API key, model, and request type.

Frontend scope:

- [x] Enhance the user wallet/account page with usage and billing summaries.
- [x] Add an owner user-detail commercial panel with wallet, ledger, usage
  charges, API key spend, recent requests, and recent billing failures.
- [x] Add empty, loading, error, and permission-denied states.

Verification:

- [x] User cannot read another user's profile.
- [x] Owner can inspect a selected user's commercial profile.
- [x] Summary totals reconcile with ledger and usage billing records.
- [x] Filters return stable results.

## Stage 14: Usage Aggregation Tables

Status: [x] Completed

Goal: introduce production-grade hourly and daily aggregation so dashboards do
not depend on scanning detail logs over long time ranges.

Backend scope:

- [x] Add hourly and daily usage aggregate entities.
- [x] Aggregate dimensions: user, API key, project, channel, model, request type,
  status, and an optional upstream account ID reserved for Stage 15+.
- [x] Aggregate metrics: request count, success count, error count, prompt
  tokens, completion tokens, total tokens, user charge micros, upstream cost
  micros, and gross margin micros.
- [x] Update usage billing completion to enqueue or apply aggregate updates
  idempotently.
- [x] Add a rebuild task that can regenerate aggregates from detail rows.
- [x] Keep detail UsageLog and UsageBillingRecord as the source of truth.

Frontend scope:

- [x] Move user and owner trend charts to aggregate-backed queries.
- [x] Add model, project, channel, and API key rankings from aggregates.
- [x] Keep detail request and billing pages for auditing and debugging.

Verification:

- [x] Aggregate totals match seeded detail rows.
- [x] Rebuild is repeatable and idempotent.
- [x] Duplicate usage billing does not double count aggregates.
- [x] Long time-range dashboard queries use aggregate data.

## Stage 15: Upstream Account Pool Model

Status: [x] Completed

Goal: add `sub2api`-style upstream account pools as a first-class Channel
enhancement without removing existing Channel credentials.

Backend scope:

- [x] Add an UpstreamAccount entity owned by a Channel.
- [x] Store account name, Channel ID, credential type, encrypted credentials,
  status, schedulable flag, priority, weight, concurrency limit, optional proxy
  configuration, rate multiplier, expires_at, last_used_at, error message,
  rate_limit_reset_at, overload_until, cooldown_until, cooldown_reason,
  quota_limit_micros, and quota_used_micros.
- [x] Add an UpstreamAccountPool or equivalent grouping model for model/project
  targeting when a Channel needs more than one pool.
- [x] Keep existing Channel credentials as fallback when no account pool is
  configured.
- [x] Treat upstream credentials as sensitive data: never return plaintext
  credentials through GraphQL or REST.

Frontend scope:

- [x] Add an account-pool tab to Channel detail pages.
- [x] Support account create, edit, enable, disable, delete/archive, and test
  operations.
- [x] Display status, schedulability, quota, cooldown, last used time, failure
  reason, priority, weight, and concurrency.

Verification:

- [x] Channels without account pools keep current behavior.
- [x] Disabled, expired, quota-exhausted, and cooling-down accounts are not
  schedulable.
- [x] Account credentials are write-only from the browser's perspective.
- [x] Non-owner users cannot manage upstream accounts.

## Stage 16: Account-Level Scheduling And Circuit Breaker Integration

Status: [x] Completed

Goal: combine AxonHub's existing Channel-level routing and circuit breaker with
finer account-level retry and health behavior.

Backend scope:

- [x] Keep existing Channel selection unchanged.
- [x] After Channel selection, choose an UpstreamAccount only when the Channel has
  an enabled account pool.
- [x] Filter accounts by active status, schedulable flag, expiry, cooldown,
  overload, rate-limit reset, quota, and concurrency.
- [x] Score candidates by priority, weight, current load, recent error rate, and
  recent latency.
- [x] On account-level failure, classify errors:
  - 401/403: disable account or apply long cooldown.
  - 429: set rate_limit_reset_at.
  - 5xx/529: set overload_until or short cooldown.
  - network/proxy errors: set cooldown_until and cooldown_reason.
- [x] Retry the same request on another eligible account in the same Channel
  where the request semantics allow retry.
- [x] If all accounts fail, return a clear account-pool exhaustion error and let
  existing Channel-level failure handling continue.

Frontend scope:

- [x] Show account selected, account retry count, and account-level failure reason
  on request and trace detail views.
- [x] Show Channel-level account-pool health summaries.
- [x] Add owner actions to manually recover, disable, or test cooling accounts.

Implementation note:

- Request execution detail now shows selected upstream account and account retry
  count. Trace pages currently render trace/span metadata without a direct
  request-execution relation; upstream account trace metadata is deferred to Stage
  17 switch history / account usage monitoring to avoid showing inferred data.

Verification:

- [x] 429 on one account switches to another eligible account.
- [x] 401/403 removes the account from future scheduling.
- [x] All-account exhaustion is reported clearly.
- [x] Channels without account pools still pass existing orchestrator tests.
- [x] Account-level changes do not regress Channel-level circuit breaker behavior.

## Stage 17: Account Usage Monitoring And Switch History

Status: [x] Completed

Goal: make upstream account health, cost, margin, and switching behavior visible
to operators.

Backend scope:

- [x] Add upstream account ID to usage detail and aggregate records for requests
  that use account pools.
- [x] Add account switch history records with request ID, Channel ID, from account,
  to account, reason, error code, latency, and timestamp.
- [x] Add account monitoring queries for request count, success rate, error rate,
  429 count, 5xx count, average latency, first-token latency, user charge,
  upstream cost, gross margin, quota usage, and recent errors.
- [x] Add filters by time range, Channel, provider type, status, model, and
  account.

Frontend scope:

- [x] Add an owner account-pool monitoring page.
- [x] Add single-account detail views with usage trend, error trend, cost/margin,
  recent requests, recent switch history, and quota status.
- [x] Add account health summary badges in Channel management.

Verification:

- [x] Requests using account pools are linked to final account ID.
- [x] Account switch history is complete and queryable.
- [x] Aggregates by account reconcile with detail rows.
- [x] Ordinary users cannot view upstream account secrets or operator-only
  account diagnostics.

## Stage 18: Commercial Operations Final Acceptance

Status: [x] Completed - automated backend acceptance added; Docker/PostgreSQL
and browser smoke remain release-gate checks in
`commercial-final-acceptance.md`.

Goal: verify the enhanced commercial platform as a coherent product, not only a
set of backend features.

Backend acceptance:

- [x] Public registration, login, wallet recharge, redeem, subscription purchase,
  API key limits, billing admission, usage billing, account-pool scheduling, and
  account switching work together.
- [ ] Docker/PostgreSQL startup works with all commercial and account-pool tables.
- [x] SQLite single-node development mode still works.
- [ ] Runtime logs do not contain payment secrets, upstream account secrets, or
  full API keys.
- [x] Persisted payment provider config does not contain ePay secrets in
  plaintext.
- [x] Aggregate rebuild and maintenance workers are repeatable.
- [x] Any unresolved Stage 11 production checklist items are either completed or
  explicitly carried as production risks.

Frontend acceptance:

- [x] User browser smoke confirms registration, sign-in, recharge, wallet,
  redeem, subscription purchase, and paid order visibility.
- [x] Owner browser smoke confirms the commercial billing console entry and core
  commercial operation tabs.
- [ ] Desktop and mobile layouts are usable.
- [x] Production build passes.

Verification:

- [x] Focused backend tests for changed business services pass.
- [x] Orchestrator account-pool tests pass.
- [x] GraphQL authorization tests pass.
- [x] Frontend typecheck and build pass.
- [ ] Frontend lint is clean; full `pnpm lint` is currently blocked by existing
  repository-wide lint debt outside this stage.
- [x] Browser smoke tests cover owner and normal user commercial billing flows.

## Stage 19: Commercial Browser Smoke Automation

Status: [x] Completed

Goal: add repeatable browser smoke coverage for the owner commercial console and
normal-user commercial lifecycle.

Backend scope:

- [x] Allow normal users to read their own subscription records after a
  self-service purchase without requiring owner billing scopes.
- [x] Allow normal users to read enabled subscription plans needed by public
  plan listings and purchased subscription edge resolution.
- [x] Return a purchase result with the plan edge preloaded so GraphQL
  transaction boundaries do not leave browser mutations with a closed
  transaction-backed entity.
- [x] Add GraphQL regression coverage for the exact purchase selection set used
  by the browser, including the production GraphQL transactioner.

Frontend scope:

- [x] Add Playwright helpers for API sign-in, commercial registration enablement,
  seeded redeem codes, subscription plans, sell price rules, and simulated ePay
  providers.
- [x] Add a commercial smoke spec that covers owner `/admin/billing` entry,
  public `/sign-up`, normal-user `/billing`, redeem-code redemption,
  subscription purchase, and simulated ePay recharge.
- [x] Fix billing and admin billing commercial profile panels so missing optional
  arrays from the API do not crash the page.
- [x] Make simulated ePay assertions wait for the real return URL instead of
  treating the current `/billing` route as completion.

Verification:

- [x] `go test ./internal/server/gql -run 'TestBilling(GraphQLUserCanPurchaseSubscriptionPlanSelection|ResolversUserCanPurchaseAndReadOwnSubscriptionPlan|ResolversRejectsPriceRuleManagementForNonOwner)' -count=1`
- [x] `pnpm exec tsc --noEmit`
- [x] `./scripts/e2e/e2e-test.sh commercial-smoke.spec.ts`
- [x] Browser smoke confirms owner console entry, public registration, user
  wallet rendering, redeem redemption, subscription purchase, and simulated ePay
  paid order visibility.

## Completion Log

Append one line per completed enhancement stage.

| Stage | Commit | Date | Notes |
| --- | --- | --- | --- |
| Stage 12 | this commit | 2026-07-09 | Added public registration settings, real sign-up API/UI, user wallet initialization, optional default project/API key, signup grant, rate limiting, audit log, and focused tests. |
| Stage 17 | this commit | 2026-07-09 | Added upstream-account usage dimensions, switch history, owner monitoring/detail pages, and focused backend/frontend verification. |
| Stage 18 | this commit | 2026-07-09 | Added final commercial lifecycle acceptance test, Stage 18 acceptance evidence, and release-gate checklist for remaining deployment/browser smoke. |
| Stage 19 | this commit | 2026-07-09 | Added commercial Playwright smoke automation for owner billing, public registration, user wallet, redeem, subscription purchase, and simulated ePay recharge; fixed subscription GraphQL edge resolution across transaction boundaries. |
