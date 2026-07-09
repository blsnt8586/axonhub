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
- [ ] Public registration is not a real product flow yet: the frontend sign-up
  form does not call a backend registration API, and the public routes only
  expose health, payment callbacks, system status/initialize, sign-in, and OAuth.
- [ ] Upstream account pools are not first-class resources yet. Channel
  credentials and disabled API keys exist, but they are not equivalent to a
  schedulable account pool with per-account health, quota, proxy, and switch
  history.
- [ ] Long-range usage analytics still need production-grade aggregation tables
  instead of depending only on detail rows and ad hoc queries.

## Stage 12: Public Registration And Onboarding

Status: [ ] Planned

Goal: turn the existing `/sign-up` page into a real self-service registration
flow while keeping owner/admin creation separate.

Backend scope:

- [ ] Add registration settings: enabled, require approval, default user status,
  default project policy, optional default API key creation, optional signup
  wallet grant, and rate-limit settings.
- [ ] Add a public registration API that creates only normal users. It must reuse
  existing password hashing and user creation logic, but it must not accept
  owner/admin role input from the public request.
- [ ] Create or initialize the user billing account during registration.
- [ ] Optionally create a default project and default user API key according to
  settings.
- [ ] Enforce duplicate email checks, password strength, request rate limits,
  and registration-disabled behavior.
- [ ] Add audit or system log records for successful registration and rejected
  registration attempts where useful.

Frontend scope:

- [ ] Replace the current sign-up form placeholder behavior with a real API call.
- [ ] Show registration-disabled, pending-approval, success, and validation
  states.
- [ ] After successful registration, redirect to sign-in with a clear success
  message.
- [ ] Add owner settings controls for registration policy.

Verification:

- [ ] Registration disabled rejects public registration.
- [ ] Public registration cannot create owner/admin users.
- [ ] Duplicate email fails deterministically.
- [ ] Successful registration creates user and billing account.
- [ ] Optional default project/API key creation follows settings.
- [ ] Frontend typecheck and build pass.

## Stage 13: User Account Profile And Usage Console

Status: [ ] Planned

Goal: provide `new-api`-style user commercial visibility without changing the
existing AxonHub wallet or billing source of truth.

Backend scope:

- [ ] Add user account summary queries for balance, held amount, available amount,
  credit limit, total recharge, total consumption, today's consumption, current
  month consumption, request count, failure count, top models, top projects, and
  top API keys.
- [ ] Build summaries from existing BillingAccount, LedgerTransaction,
  UsageBillingRecord, Request, and UsageLog data.
- [ ] Enforce access rules: users can see only themselves; owner can inspect any
  user.
- [ ] Add filters by time range, project, API key, model, and request type.

Frontend scope:

- [ ] Enhance the user wallet/account page with usage and billing summaries.
- [ ] Add an owner user-detail commercial panel with wallet, ledger, usage
  charges, API key spend, recent requests, and recent billing failures.
- [ ] Add empty, loading, error, and permission-denied states.

Verification:

- [ ] User cannot read another user's profile.
- [ ] Owner can inspect a selected user's commercial profile.
- [ ] Summary totals reconcile with ledger and usage billing records.
- [ ] Filters return stable results.

## Stage 14: Usage Aggregation Tables

Status: [ ] Planned

Goal: introduce production-grade hourly and daily aggregation so dashboards do
not depend on scanning detail logs over long time ranges.

Backend scope:

- [ ] Add hourly and daily usage aggregate entities.
- [ ] Aggregate dimensions: user, API key, project, channel, model, request type,
  status, and an optional upstream account ID reserved for Stage 15+.
- [ ] Aggregate metrics: request count, success count, error count, prompt
  tokens, completion tokens, total tokens, user charge micros, upstream cost
  micros, and gross margin micros.
- [ ] Update usage billing completion to enqueue or apply aggregate updates
  idempotently.
- [ ] Add a rebuild task that can regenerate aggregates from detail rows.
- [ ] Keep detail UsageLog and UsageBillingRecord as the source of truth.

Frontend scope:

- [ ] Move user and owner trend charts to aggregate-backed queries.
- [ ] Add model, project, channel, and API key rankings from aggregates.
- [ ] Keep detail request and billing pages for auditing and debugging.

Verification:

- [ ] Aggregate totals match seeded detail rows.
- [ ] Rebuild is repeatable and idempotent.
- [ ] Duplicate usage billing does not double count aggregates.
- [ ] Long time-range dashboard queries use aggregate data.

## Stage 15: Upstream Account Pool Model

Status: [ ] Planned

Goal: add `sub2api`-style upstream account pools as a first-class Channel
enhancement without removing existing Channel credentials.

Backend scope:

- [ ] Add an UpstreamAccount entity owned by a Channel.
- [ ] Store account name, Channel ID, credential type, encrypted credentials,
  status, schedulable flag, priority, weight, concurrency limit, optional proxy
  configuration, rate multiplier, expires_at, last_used_at, error message,
  rate_limit_reset_at, overload_until, cooldown_until, cooldown_reason,
  quota_limit_micros, and quota_used_micros.
- [ ] Add an UpstreamAccountPool or equivalent grouping model for model/project
  targeting when a Channel needs more than one pool.
- [ ] Keep existing Channel credentials as fallback when no account pool is
  configured.
- [ ] Treat upstream credentials as sensitive data: never return plaintext
  credentials through GraphQL or REST.

Frontend scope:

- [ ] Add an account-pool tab to Channel detail pages.
- [ ] Support account create, edit, enable, disable, delete/archive, and test
  operations.
- [ ] Display status, schedulability, quota, cooldown, last used time, failure
  reason, priority, weight, and concurrency.

Verification:

- [ ] Channels without account pools keep current behavior.
- [ ] Disabled, expired, quota-exhausted, and cooling-down accounts are not
  schedulable.
- [ ] Account credentials are write-only from the browser's perspective.
- [ ] Non-owner users cannot manage upstream accounts.

## Stage 16: Account-Level Scheduling And Circuit Breaker Integration

Status: [ ] Planned

Goal: combine AxonHub's existing Channel-level routing and circuit breaker with
finer account-level retry and health behavior.

Backend scope:

- [ ] Keep existing Channel selection unchanged.
- [ ] After Channel selection, choose an UpstreamAccount only when the Channel has
  an enabled account pool.
- [ ] Filter accounts by active status, schedulable flag, expiry, cooldown,
  overload, rate-limit reset, quota, and concurrency.
- [ ] Score candidates by priority, weight, current load, recent error rate, and
  recent latency.
- [ ] On account-level failure, classify errors:
  - 401/403: disable account or apply long cooldown.
  - 429: set rate_limit_reset_at.
  - 5xx/529: set overload_until or short cooldown.
  - network/proxy errors: set cooldown_until and cooldown_reason.
- [ ] Retry the same request on another eligible account in the same Channel
  where the request semantics allow retry.
- [ ] If all accounts fail, return a clear account-pool exhaustion error and let
  existing Channel-level failure handling continue.

Frontend scope:

- [ ] Show account selected, account retry count, and account-level failure reason
  on request and trace detail views.
- [ ] Show Channel-level account-pool health summaries.
- [ ] Add owner actions to manually recover, disable, or test cooling accounts.

Verification:

- [ ] 429 on one account switches to another eligible account.
- [ ] 401/403 removes the account from future scheduling.
- [ ] All-account exhaustion is reported clearly.
- [ ] Channels without account pools still pass existing orchestrator tests.
- [ ] Account-level changes do not regress Channel-level circuit breaker behavior.

## Stage 17: Account Usage Monitoring And Switch History

Status: [ ] Planned

Goal: make upstream account health, cost, margin, and switching behavior visible
to operators.

Backend scope:

- [ ] Add upstream account ID to usage detail and aggregate records for requests
  that use account pools.
- [ ] Add account switch history records with request ID, Channel ID, from account,
  to account, reason, error code, latency, and timestamp.
- [ ] Add account monitoring queries for request count, success rate, error rate,
  429 count, 5xx count, average latency, first-token latency, user charge,
  upstream cost, gross margin, quota usage, and recent errors.
- [ ] Add filters by time range, Channel, provider type, status, model, and
  account.

Frontend scope:

- [ ] Add an owner account-pool monitoring page.
- [ ] Add single-account detail views with usage trend, error trend, cost/margin,
  recent requests, recent switch history, and quota status.
- [ ] Add account health summary badges in Channel management.

Verification:

- [ ] Requests using account pools are linked to final account ID.
- [ ] Account switch history is complete and queryable.
- [ ] Aggregates by account reconcile with detail rows.
- [ ] Ordinary users cannot view upstream account secrets or operator-only
  account diagnostics.

## Stage 18: Commercial Operations Final Acceptance

Status: [ ] Planned

Goal: verify the enhanced commercial platform as a coherent product, not only a
set of backend features.

Backend acceptance:

- [ ] Public registration, login, wallet recharge, redeem, subscription purchase,
  API key limits, billing admission, usage billing, account-pool scheduling, and
  account switching work together.
- [ ] Docker/PostgreSQL startup works with all commercial and account-pool tables.
- [ ] SQLite single-node development mode still works.
- [ ] Logs do not contain payment secrets, upstream account secrets, or full API
  keys.
- [ ] Aggregate rebuild and maintenance workers are repeatable.
- [ ] Any unresolved Stage 11 production checklist items are either completed or
  explicitly carried as production risks.

Frontend acceptance:

- [ ] User can register, sign in, recharge, create an API key, call the API, and
  view wallet/usage/billing data.
- [ ] Owner can inspect users, ledgers, usage charges, reports, account pools,
  account switching, and account health.
- [ ] Desktop and mobile layouts are usable.
- [ ] Production build passes.

Verification:

- [ ] Focused backend tests for changed business services pass.
- [ ] Orchestrator account-pool tests pass.
- [ ] GraphQL authorization tests pass.
- [ ] Frontend typecheck, targeted lint, and build pass.
- [ ] Browser smoke tests cover owner and normal user flows.

## Completion Log

Append one line per completed enhancement stage.

| Stage | Commit | Date | Notes |
| --- | --- | --- | --- |
