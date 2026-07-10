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
- [x] Upstream account pools are first-class Channel resources with
  per-account credentials, schedulability, health, quota, cooldown, scheduling,
  switch history, monitoring, and browser-smoke coverage.
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

Status: [x] Completed - automated backend, browser, Docker, and fresh PostgreSQL
acceptance are recorded in `commercial-final-acceptance.md`; production-scale
upgrade rehearsal and external log-sink sampling remain release gates.

Goal: verify the enhanced commercial platform as a coherent product, not only a
set of backend features.

Backend acceptance:

- [x] Public registration, login, wallet recharge, redeem, subscription purchase,
  API key limits, billing admission, usage billing, account-pool scheduling, and
  account switching work together.
- [x] Docker/PostgreSQL startup works with all commercial and account-pool tables.
- [x] SQLite single-node development mode still works.
- [x] Runtime logs do not contain payment secrets, upstream account secrets, or
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
- [x] Desktop and mobile layouts are usable for core commercial and account
  monitoring workflows.
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

## Stage 20: Account Pool Browser Smoke And Roadmap Alignment

Status: [x] Completed

Goal: verify the upstream account-pool product loop in the browser and align the
roadmap baseline with the completed Stage 15-17 implementation.

Backend scope:

- [x] Keep the existing Channel, UpstreamAccountPool, UpstreamAccount,
  account-level scheduling, switch history, and monitoring services unchanged.
- [x] Seed a dedicated smoke Channel through GraphQL so the browser test does
  not depend on pre-existing operator data.
- [x] Exercise the existing GraphQL create-pool and create-account mutations
  through browser-managed UI actions.

Frontend scope:

- [x] Promote account-pool management to a first-class Channel row action
  instead of leaving it only under the overflow menu.
- [x] Add stable test identifiers for Channel name filtering and account-pool
  management controls.
- [x] Add Playwright coverage for opening a Channel's upstream-account dialog,
  creating an account pool, creating a write-only-credential upstream account,
  and confirming account inventory state.
- [x] Add Playwright coverage for the account monitoring list and single-account
  detail page, including quota/cooldown, recent executions, and switch-history
  sections.
- [x] Preserve the existing dense admin-table UI pattern; no new marketing-style
  or card-heavy surfaces were introduced.

Verification:

- [x] `pnpm exec tsc --noEmit`
- [x] `pnpm build`
- [x] `./scripts/e2e/e2e-test.sh upstream-accounts-smoke.spec.ts`
- [x] Browser smoke confirms account pools can be managed as first-class
  operator resources and monitored outside the Channel detail dialog.

## Stage 21: Docker/PostgreSQL Commercial Production Acceptance

Status: [x] Completed

Goal: verify that the current fork builds as a production container, starts
against a fresh PostgreSQL database, runs the commercial browser flows on
PostgreSQL, and preserves data across an application restart.

Deployment scope:

- [x] Build the frontend with the repository's locked pnpm configuration and a
  Node.js version supported by pnpm 11.
- [x] Build the current fork as `axonhub:local` instead of starting the upstream
  published image.
- [x] Make the development Compose ports configurable so acceptance can run
  without interfering with existing services.
- [x] Pass `AXONHUB_PAYMENT_SECRET_KEY` through Compose for production payment
  provider encryption.
- [x] Keep custom `config.yml` mounting optional so the default stack starts
  from a clean checkout.

Automated acceptance:

- [x] Add `scripts/e2e/commercial-postgres-acceptance.sh` to provision an
  isolated PostgreSQL instance and run both commercial and account-pool browser
  smoke suites against it.
- [x] Verify commercial, aggregate, and upstream-account tables are created by
  fresh-database migrations.
- [x] Verify wallet, payment order, subscription, pool, and account smoke data
  is persisted in PostgreSQL.
- [x] Restart the backend against the retained test database and verify health.
- [x] Build the production Docker image, start the Compose stack, wait for
  container healthchecks, and verify the isolated `/health` endpoint.
- [x] Clean up Stage 21 containers, networks, and volumes without touching
  unrelated local services.

Verification:

- [x] `./scripts/e2e/commercial-postgres-acceptance.sh`
- [x] PostgreSQL browser smoke: 3 tests passed.
- [x] Docker image frontend and Go production builds passed.
- [x] Compose PostgreSQL and AxonHub containers became healthy.
- [x] All required commercial and upstream-account tables were present after
  Compose startup.

## Stage 22: Historical Migration And PostgreSQL Recovery Acceptance

Status: [x] Completed

Goal: verify that an existing pre-commercial AxonHub PostgreSQL database can be
upgraded without losing legacy records, and that the resulting commercial
database can be backed up, destroyed, restored, and used through the browser.

Historical upgrade scope:

- [x] Build the pre-commercial baseline commit `26584ccf` from the local Git
  history in an isolated temporary directory.
- [x] Initialize the historical schema with an owner, default project, and
  system settings.
- [x] Start the current fork against the same PostgreSQL database and allow the
  normal application migration path to create commercial and account-pool
  tables.
- [x] Compare the legacy owner identity and legacy user, project, and system
  counts before and after migration.
- [x] Run the normal commercial browser lifecycle on the upgraded database.

Backup and recovery scope:

- [x] Create a real plain-format `pg_dump` with ownership and privilege metadata
  excluded for portable restoration.
- [x] Reject the backup if the simulated ePay provider secret appears in
  plaintext.
- [x] Drop and recreate the isolated test database before restoring the dump.
- [x] Compare wallet, ledger, payment, provider, subscription, redeem, user, and
  project manifests before and after restore.
- [x] Add browser smoke that signs in as the restored normal user and verifies
  an active wallet, paid order, and active subscription through GraphQL and the
  billing page.
- [x] Clean up the dedicated PostgreSQL container and temporary backup/build
  artifacts without touching unrelated services.

Verification:

- [x] `./scripts/e2e/commercial-postgres-recovery-acceptance.sh`
- [x] Upgraded-database commercial browser smoke: 2 tests passed.
- [x] Restored-database commercial recovery browser smoke: 2 tests passed.
- [x] Pre-commercial owner, project, user, and system manifests matched across
  migration.
- [x] Commercial record manifests matched across backup and restore.
- [x] Restored user could sign in and read wallet, paid order, and active
  subscription state.

## Stage 23: Production Log Secret Redaction And Runtime Audit

Status: [x] Completed

Goal: prevent application logs from exposing commercial payment secrets,
upstream credentials, API keys, authentication tokens, cookies, passwords, or
database credentials, including at debug level.

Backend scope:

- [x] Add a redacting zap core at the final application log write boundary so
  existing and future logger call sites use the same policy.
- [x] Redact sensitive field names after punctuation/case normalization,
  including authorization, API key, token, password, secret, credential,
  cookie, DSN, and provider-key variants.
- [x] Recursively sanitize reflected maps, structs, slices, JSON bodies,
  GraphQL variables, request headers, and object marshalers while preserving
  non-sensitive diagnostic fields.
- [x] Sanitize free-form log messages, errors, DSN URLs, Bearer values, OpenAI
  style `sk-` keys, Google API keys, and JWT-shaped values.
- [x] Apply redaction to fields attached through `WithFields` and logger names,
  not only fields passed directly to one log statement.

Runtime acceptance:

- [x] Add `scripts/e2e/commercial-log-redaction-acceptance.sh` with an isolated
  SQLite application and debug file logging.
- [x] Save an ePay-compatible provider with a canary key through GraphQL so raw
  GraphQL variables exercise the redactor.
- [x] Create a real service-account API key and use it in an Authorization
  header against the webhook debug path.
- [x] Send API key, Cookie, query token, password, secret, and PostgreSQL DSN
  canaries through logged request structures.
- [x] Fail when any full canary appears in the runtime log, require redaction
  markers, and require a non-sensitive marker to remain visible.
- [x] Keep external reverse-proxy, container-runtime, and log-collector sampling
  as a production deployment check because those systems do not use AxonHub's
  logger core.

Verification:

- [x] `go test ./internal/log -count=1`
- [x] `go test ./internal/server/api ./internal/server/gql ./internal/server/biz ./internal/server/orchestrator -count=1`
- [x] `./scripts/e2e/commercial-log-redaction-acceptance.sh`
- [x] Runtime scan confirmed owner password, payment encryption key, ePay key,
  admin JWT, service API key, X-API-Key, Cookie, body password/secret, DSN
  password, and query token were absent from application logs.
- [x] Runtime logs retained `[REDACTED]` markers and non-sensitive request data.

## Stage 24: Desktop And Mobile Commercial Responsive Acceptance

Status: [x] Completed

Goal: verify that normal-user billing and owner commercial/account-monitoring
workflows remain usable without horizontal page overflow or overlapping
controls on desktop and mobile viewports.

Frontend test scope:

- [x] Seed a registered normal user with signup credit, redeemed balance, and an
  active subscription through the real registration and GraphQL APIs.
- [x] Seed an owner-visible Channel, upstream account pool, and upstream account
  so monitoring and detail pages contain realistic data.
- [x] Verify `/billing`, `/admin/billing`, `/channels/accounts`, and the linked
  account detail route at `1440x1000` and `390x844`.
- [x] Confirm recharge, redeem, subscription, admin Operations, monitoring, and
  account-detail controls are visible and reachable.
- [x] Detect document-level horizontal overflow for each page state.
- [x] Detect intersections between rendered buttons, inputs, textareas, selects,
  and tabs across the full page, not only the initial viewport.
- [x] Attach eight full-page screenshots to the Playwright report for visual
  review.
- [x] Use the real account link from the monitoring table for detail navigation
  so route encoding matches production behavior.

Visual review:

- [x] Mobile wallet metrics stack without clipping and long explanatory text
  wraps inside the viewport.
- [x] Mobile billing-admin tabs remain horizontally scrollable and the
  Operations tab can be brought into view and activated.
- [x] Mobile upstream-account metrics and filters stack without document
  overflow.
- [x] Long Channel and upstream-account names wrap on the account detail page
  without covering navigation or actions.
- [x] No production frontend component changes were required after automated
  and screenshot review.

Verification:

- [x] `pnpm exec tsc --noEmit`
- [x] `pnpm build`
- [x] `./scripts/e2e/e2e-test.sh commercial-responsive-smoke.spec.ts`
- [x] `./scripts/e2e/commercial-responsive-acceptance.sh`
- [x] Playwright setup plus responsive suite: 2 tests passed.
- [x] Eight desktop/mobile page states passed overflow and overlap assertions.

## Stage 25: Historical Commercial PostgreSQL Upgrade Matrix

Status: [x] Completed

Goal: prove that databases created by multiple earlier commercial stages can be
upgraded to the current fork without losing business records, duplicating ledger
effects, or retaining legacy plaintext payment-provider keys.

Upgrade baselines:

- [x] Stage 1 `94106a72`: wallet holds and pre-authorization era.
- [x] Stage 2 `bf4368b5`: hardened payment-order lifecycle era.
- [x] Stage 6 `261e482a`: redeem-code and subscription era.
- [x] Stage 11 `185594ee`: production-operations acceptance era.

Historical data preparation:

- [x] Build every baseline from the local Git history in an isolated temporary
  directory.
- [x] Start every historical binary against its own PostgreSQL database and
  initialize a real owner and default project.
- [x] Use each baseline's own GraphQL APIs to create a user wallet balance,
  ledger entries, a confirmed manual recharge order, payment event, and ePay
  provider.
- [x] For Stage 6 and Stage 11, additionally create and redeem a code, create a
  subscription plan, and purchase an active subscription.

Migration hardening:

- [x] Detect legacy plaintext ePay keys during commercial defaults startup and
  replace them with `enc:v1:` encrypted values.
- [x] Validate provider configuration before rewriting it.
- [x] Migrate all legacy provider keys in one transaction so an invalid provider
  rolls back earlier provider updates instead of leaving a partial migration.
- [x] Leave already-encrypted provider configs unchanged across repeated
  startups.
- [x] Log only provider ID/name when a legacy secret is migrated.

Data verification:

- [x] Compare stable row-level manifests for users, projects, billing accounts,
  ledger transactions, ledger entries, payment orders, payment events, and
  provider metadata before and after every upgrade.
- [x] Compare redeem codes, plans, and user subscriptions for baselines where
  those modules existed.
- [x] Confirm the legacy plaintext provider key is absent and `enc:v1:` is
  present after upgrade.
- [x] Restart the current fork a second time and confirm manifests remain
  unchanged, proving migration idempotency and no duplicate balance posting.
- [x] Clean up the dedicated PostgreSQL container and all historical build
  artifacts without touching unrelated services.

Verification:

- [x] `go test ./internal/server/biz -run 'TestCommercialOperationsEnsureDefaults' -count=1`
- [x] `go test ./internal/server/biz -count=1`
- [x] `./scripts/e2e/commercial-postgres-upgrade-matrix.sh`
- [x] Stage 1, Stage 2, Stage 6, and Stage 11 upgrade acceptance passed.
- [x] Four second-start manifests matched their first upgraded manifests.

## Stage 26: Gateway Commercial Billing End-To-End Acceptance

Status: [x] Completed

Goal: prove that commercial billing is enforced from the real
`ChatCompletionOrchestrator.Process` path without replacing or weakening
AxonHub's existing channel scheduling and retry behavior.

Gateway lifecycle:

- [x] Execute a real OpenAI-compatible chat-completion request with project and
  user API key context through inbound transformation, admission, candidate
  selection, mocked upstream execution, usage persistence, and usage billing.
- [x] Verify `billing.mode=enforce` rejects a zero-balance user with HTTP 402
  before request persistence, candidate execution, or usage creation.
- [x] Verify project price rules take precedence over global rules while the
  resulting debit remains attached to the API key owner's user wallet.
- [x] Verify request billing holds are captured for the actual charge and leave
  no residual held balance after a successful response.
- [x] Verify `billing.mode=warn` continues upstream routing when the wallet is
  empty, records a failed outbox event, and charges once after wallet repair and
  worker retry.

Subscription and scheduling compatibility:

- [x] Verify an active matching subscription consumes included quota before the
  wallet and does not create a wallet hold.
- [x] Verify an exhausted subscription with wallet fallback enabled returns to
  wallet hold/capture billing without mutating consumed subscription quota.
- [x] Verify an upstream 500 can switch to a second Channel under the existing
  retry policy while producing one usage billing record and one usage debit.
- [x] Keep the acceptance implementation in tests and scripts; no production UI
  change is required because Stage 26 validates an existing gateway contract.

Verification:

- [x] `go test ./internal/server/orchestrator -run 'TestGatewayCommercialLifecycle' -count=1`
- [x] `go test ./internal/server/orchestrator ./internal/server/biz -count=1`
- [x] `./scripts/e2e/commercial-gateway-acceptance.sh`

## Completion Log

Append one line per completed enhancement stage.

| Stage | Commit | Date | Notes |
| --- | --- | --- | --- |
| Stage 12 | this commit | 2026-07-09 | Added public registration settings, real sign-up API/UI, user wallet initialization, optional default project/API key, signup grant, rate limiting, audit log, and focused tests. |
| Stage 17 | this commit | 2026-07-09 | Added upstream-account usage dimensions, switch history, owner monitoring/detail pages, and focused backend/frontend verification. |
| Stage 18 | this commit | 2026-07-09 | Added final commercial lifecycle acceptance test, Stage 18 acceptance evidence, and release-gate checklist for remaining deployment/browser smoke. |
| Stage 19 | this commit | 2026-07-09 | Added commercial Playwright smoke automation for owner billing, public registration, user wallet, redeem, subscription purchase, and simulated ePay recharge; fixed subscription GraphQL edge resolution across transaction boundaries. |
| Stage 20 | this commit | 2026-07-09 | Added account-pool browser smoke automation for Channel account-pool management, write-only account credentials, account monitoring, and account detail pages; aligned the roadmap baseline with completed account-pool stages. |
| Stage 21 | this commit | 2026-07-10 | Added isolated PostgreSQL commercial acceptance, persistent-data restart checks, current-fork Docker builds, Compose health verification, and fresh commercial/account-pool migration checks. |
| Stage 22 | this commit | 2026-07-10 | Added pre-commercial PostgreSQL upgrade acceptance, real pg_dump/drop/restore validation, commercial data manifest comparison, backup secret scanning, and restored-user browser smoke. |
| Stage 23 | this commit | 2026-07-10 | Added final-write-boundary structured log redaction, nested request/error sanitization, focused logger tests, and runtime canary scanning across GraphQL and webhook paths. |
| Stage 24 | this commit | 2026-07-10 | Added desktop/mobile commercial responsive smoke, realistic billing/account seed data, full-page overflow and control-overlap detection, account-link route verification, and eight screenshot attachments. |
| Stage 25 | this commit | 2026-07-10 | Added Stage 1/2/6/11 PostgreSQL upgrade matrix, historical API data seeding, row-level commercial manifest comparison, transactional legacy provider-key encryption, and second-start idempotency checks. |
| Stage 26 | this commit | 2026-07-10 | Added real orchestrator commercial billing acceptance for enforce/warn modes, project pricing, user wallets, holds, subscriptions, outbox recovery, and cross-channel retry compatibility. |
