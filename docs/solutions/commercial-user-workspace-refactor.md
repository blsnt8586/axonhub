# AxonHub Commercial User Workspace Refactor

This document defines the user-facing commercial workspace refactor that closes
the product-model gap found after public registration was enabled. It is a
subordinate workstream of the existing commercial production acceptance work;
it does not add or renumber Stage 0-28.

## Status

- [x] Review the ordinary-user model in `sub2api`.
- [x] Review the ordinary-user model in `new-api`.
- [x] Review the current AxonHub frontend and backend authorization split.
- [x] Define the target user, project, API key, billing, and administrator
  boundaries.
- [ ] Complete Workstream W3-W8 below.

## Review Snapshot

The comparison is based on these local repository snapshots:

| Project | Commit | Role in this review |
| --- | --- | --- |
| `sub2api` | `17b6481f` | User-owned keys, groups, usage, subscriptions, purchase, and account-pool boundary |
| `new-api` | `becc18e3` | Fixed user console, token controls, user logs, wallet, registration defaults, and subscription purchase |
| `axonhub` | `1285770d` | Project workspace, RBAC, routing, circuit breaker, account pools, tracing, and commercial modules |

These are implementation snapshots, not permanent claims about upstream latest
versions.

## Comparative Findings

### sub2api ordinary-user model

`sub2api` has no Project entity. Its commercial ownership chain is:

```text
User
  -> Balance
  -> User-owned API Keys
  -> Key-selected Group
  -> Usage attributed by user_id, api_key_id, and group_id
```

An authenticated ordinary user receives a fixed product surface: dashboard,
API keys, usage, available channels, monitoring, subscriptions, purchase,
orders, redeem, affiliate, and profile. Basic product access is not granted by
administrator-assigned scopes.

API key create, list, update, and delete operations are always filtered by the
authenticated user ID. A user can select only Groups that are public, explicitly
allowed for that user, or unlocked by an active subscription. Channel secrets,
upstream accounts, account pools, proxies, and scheduler administration remain
administrator-only.

### new-api ordinary-user model

`new-api` also has no Project entity. Its commercial ownership chain is:

```text
User
  -> User quota / wallet-like balance
  -> User-owned Tokens
  -> Token-selected Group and model/IP/quota restrictions
  -> Logs and aggregate usage attributed by user_id and token_id
```

The frontend exposes Playground, overview/dashboard, API keys, common/task
usage logs, wallet, and profile to every authenticated user. The Admin navigation
group is role-gated separately. Token, model, group, log, top-up, and
subscription-self routes use user authentication rather than administrator
permissions.

Token CRUD enforces `user_id` in the query and mutation paths. A token supports
status, expiry, finite or unlimited quota, model restrictions, allowed IPs,
Group selection, and optional cross-Group retry. Users can query only their
usable Groups and enabled models. User logs are filtered by `logs.user_id`;
administrator log routes are separate.

Registration creates a normal user in the default Group and applies the
configured new-user quota. It can optionally create a default token. Wallet
recharge, redemption, affiliate transfer, payment history, subscription plan
listing, self subscription state, and subscription purchase are authenticated
user operations. Channel, global model, user, redemption-code issuance,
subscription-plan administration, and system settings are administrator
operations.

### Shared lesson from sub2api and new-api

Both systems distinguish product capabilities from administration permissions:

- An authenticated active user can consume AI, manage their own credentials,
  inspect their own usage, and manage their own money without receiving admin
  scopes.
- User resource isolation is expressed by ownership filters such as `user_id`,
  not by exposing global administrator queries and relying on frontend hiding.
- Groups or plans decide availability and price behavior; they do not transfer
  ownership of the user wallet or administrator control of upstream channels.
- Administrator capabilities are separate routes and menus.

## Current AxonHub Defect

AxonHub registration correctly creates:

```text
User
  -> BillingAccount owned by User
  -> default Project membership with is_owner=true
  -> optional user API Key owned by User and Project
```

The backend authorization rule already treats a system owner or project owner
as having project-level scopes inside that project. The frontend independently
reimplemented the check and considered only the membership `scopes` array.
Consequently, a valid membership such as:

```json
{
  "isOwner": true,
  "scopes": []
}
```

was rejected by `RouteGuard`. Login then redirected every non-system-owner to
`/project/playground`, so the first post-login screen became Access Denied.

This is not a registration-data defect and must not be fixed by granting system
scopes such as `read_channels`, `read_system`, or `read_billing` to new users.

## Target Product Model

AxonHub keeps Project because it provides useful workspace, isolation, pricing,
and collaboration semantics missing from the other two systems:

```text
User
  -> user-owned wallet and commercial account
  -> one or more Project memberships
       -> user-owned API Keys scoped to the Project
       -> requests, traces, usage, prompts, and project price rules
  -> recharge orders, subscriptions, redeem history, and affiliate records

Administrator
  -> Channels and provider credentials
  -> upstream account pools and scheduling
  -> circuit breakers and global routing
  -> global model catalog and price rules
  -> all users, wallets, payment providers, and operations
```

### Capability layers

The frontend and backend must use three explicit capability layers:

1. **Authenticated user capability**: profile, wallet, recharge, orders,
   subscriptions, redeem, affiliate, and user dashboard.
2. **Project consumer capability**: project switch, own API keys, Playground,
   own requests, own usage, usable models, and prices. Membership is required;
   administrator system scopes are not.
3. **Project/system administration capability**: project members and roles,
   shared/service-account keys, project configuration, Channels, upstream
   accounts, routing, global prices, payment providers, and all-user reports.

Project ownership remains meaningful for managing that workspace, but the
ordinary-user navigation defaults to the consumer surface. Being the owner of a
self-created Project does not make the user a system administrator.

### Resource visibility

| Resource | Ordinary user | Project owner/admin | System owner/admin |
| --- | --- | --- | --- |
| User wallet and orders | Own only | Own only | Any user through admin operations |
| Subscription and redeem history | Own only | Own only | Global administration |
| Projects | Memberships only | Owned/member Projects | All Projects |
| Personal/user API keys | Own keys in selected Project | Project keys allowed by project role | All keys |
| Requests and usage | Own by default | Project-wide when authorized | Global/project-wide |
| Usable models and public prices | Read consumer projection | Read project projection | Configure global catalog and prices |
| Channels and upstream accounts | No secret or scheduler access | No system access | Full administration |
| Project users, roles, shared keys | No default access | Project administration | Full administration |

## Non-Goals

- Do not replace AxonHub Orchestrator, Channel selection, circuit breaker,
  account pools, tracing, request storage, or provider transformations.
- Do not copy new-api's integer quota ledger over the existing wallet, ledger,
  hold, usage billing, and subscription accounting.
- Do not remove Project or move wallet ownership to Project.
- Do not grant normal users system-level administrator scopes.
- Do not expose Channel IDs, credentials, account health internals, or routing
  controls through consumer model/price APIs.
- Do not maintain a second authorization truth in React components.

## Implementation Workstreams

Each workstream uses TDD, changes backend and frontend together when both are in
scope, updates this checklist, and ends in one rollback-friendly Git commit.

### W1: Capability Model And Post-Login Landing

- [x] Extract a pure frontend route-capability evaluator with tests.
- [x] Make project-owner semantics match the backend project-scope rules.
- [x] Mark consumer, project-admin, and system-admin routes explicitly.
- [x] Remove contradictory route declarations, especially Playground being
  declared public in one file but scope-protected in another.
- [x] Redirect ordinary users to a user dashboard instead of directly to
  Playground; preserve an explicit requested redirect after login.
- [x] Add Playwright coverage for a newly registered user reaching an accessible
  first screen without system scopes.

Status: [x] Completed

Implementation notes:

- Route and sidebar authorization now share one project-owner-aware evaluator.
- Navigation groups use stable IDs instead of translated titles, preventing
  localized Admin navigation from bypassing role filtering.
- Ordinary users land on `/home`; system owners retain the operations dashboard.
- Internal post-login redirects are preserved while external and
  protocol-relative redirects are rejected.
- `/home` exposes only user commercial summary and consumer actions; it does not
  query or expose Channel administration.

Acceptance:

- A registered user with `isOwner=true` and `scopes=[]` for the default Project
  does not see Access Denied on consumer routes.
- The same user cannot see or open Channels, upstream accounts, admin billing,
  global users, roles, or system settings.
- A non-owner project member receives only the consumer and project permissions
  actually allowed by the target resource policy.

### W2: User Dashboard And Onboarding State

- [x] Add an authenticated user dashboard route and summary query.
- [x] Show wallet available balance, active Project, API key count, today's
  requests/consumption, active subscription, and usable model count.
- [x] Add action-oriented empty states for no Project, no API key, no available
  model, zero balance, and inactive or approval-required accounts.
- [x] Link directly to API key management, wallet recharge, usage, and
  Playground; show model availability here and defer the full catalog to W5.
- [x] Keep administrator operational metrics on the existing admin dashboard.

Status: [x] Completed

Implementation notes:

- The authenticated summary endpoint accepts an AxonHub Project GUID, verifies
  membership before using an internal privacy bypass, and returns no Channel
  identifiers, credentials, or scheduling internals.
- API key and request counts are restricted to the current user and selected
  Project. Active subscriptions and wallet availability are evaluated together.
- Available model count is derived from enabled Channel model entries after
  applying the active Project profile's Channel ID/tag restrictions.
- Onboarding reports one deterministic blocking reason: inactive user, missing
  Project, missing enabled key, no available model, unavailable billing account,
  insufficient balance, or ready.
- The frontend no longer loads the complete commercial-profile detail payload
  for the home page.
- Vite now proxies `/admin/account/` and `/admin/users/`, fixing local browser
  requests that previously returned the frontend HTML shell instead of JSON.
- Browser acceptance covers the no-model state and horizontal overflow at a
  `390x844` viewport.

Acceptance:

- Dashboard data is limited to the current user and selected Project.
- Empty installations explain unavailable service without exposing Channel
  configuration or producing a permission error.

### W3: Workspace Selection And Membership Projection

- [x] Treat Project as a user workspace in consumer copy and navigation while
  retaining Project terminology in administrator configuration.
- [x] Return a dedicated membership projection: Project ID/name/status,
  membership owner flag, consumer capabilities, and administration capabilities.
- [x] Make selected-Project initialization deterministic after registration,
  login, storage loss, Project archive, or membership removal.
- [x] Add a consumer workspace page for listing and switching memberships.
- [x] Decide project creation policy through a commercial setting; do not reuse
  the global `write_projects` scope as the only self-service policy switch.

Implemented:

- `GET /admin/account/workspaces` returns only the authenticated user's active
  memberships. Project IDs use AxonHub GUIDs and the response excludes Channels,
  upstream accounts, credentials, circuit-breaker state, and scheduling details.
- Consumer capabilities (`consumeAI`, `manageOwnAPIKeys`, and `viewOwnUsage`) are
  separated from Project administration capabilities. Administration capability
  calculation merges Project ownership, direct membership scopes, and assigned
  Project-role scopes without adding any system scope to an ordinary user.
- `POST /admin/account/workspaces` uses `ProjectService.CreateProject`, preserving
  the existing Admin/Developer/Viewer roles and owner membership. The operation
  runs transactionally, trims and validates names, enforces the per-user active
  workspace limit, and invalidates the authenticated user cache after creation.
- The independent `workspace_settings` system value defaults to self-service
  creation disabled and one active workspace per user. System owners can manage
  it under the registration settings UI through `/admin/system/workspaces`.
- The consumer `/workspaces` page lists, switches, and conditionally creates
  workspaces. The header switcher consumes the same safe REST projection instead
  of the administrator-oriented Project GraphQL query.
- Selection recovery is a unit-tested pure function: retain a valid active
  selection, otherwise use the server default, otherwise the first active
  membership, otherwise clear storage. Creating a workspace refreshes both the
  workspace projection and `me` membership cache before protected routes are used.
- Browser acceptance covers disabled creation (API 403 and disabled UI), owner
  policy UI visibility, stale local-storage recovery, successful self-service
  creation, automatic selection, member cache refresh, and `390x844` overflow.

Acceptance:

- A user can switch among all and only their memberships.
- Stale selected Project IDs recover to an active membership.
- Self-service Project creation, when disabled, is absent in both API and UI.

### W4: User-Owned API Key Lifecycle

- [x] Add explicit self-service API key operations whose ownership checks are
  `current user + selected Project`, independent of global key administration.
- [x] Keep service-account/shared-key management under project administration.
- [x] Support name, status, rotation, expiry, model restrictions, IP allowlist,
  commercial budget, and request/rate limits where the runtime enforces them.
- [x] Display the full secret only at creation/rotation and keep stored/listed
  values masked.
- [x] Add usable-model and public-price selectors without Channel secrets.
- [x] Preserve AxonHub API Key Profile mapping and restrictions.

Implemented:

- Consumer operations use `/admin/account/api-keys` and require both Project and
  API Key GUIDs. Every read and mutation first verifies the authenticated user is
  an active Project member, then queries with `user_id + project_id + api_key_id`.
  A user cannot infer or mutate another member's key even inside a shared Project.
- Newly registered default keys and self-service keys use the existing `personal`
  type. Legacy user-owned `user` keys remain visible through the same strictly
  owner-scoped projection for compatibility.
- Create and rotate responses return the full secret once. List/update responses
  expose only a prefix/suffix mask. Rotation invalidates old and new cache keys;
  archive acts as consumer deletion while retaining request and billing history.
- API Key schema now stores optional `expires_at` and normalized IP/CIDR
  `ip_allowlist` values. Expiration is checked by `AuthenticateAPIKey`; every HTTP
  API-key middleware also validates `ClientIP` before installing the principal.
- Allowed model IDs and request limits are projected into the active AxonHub API
  Key Profile. Model mappings, Channel filters, load-balancing strategy, token
  quota, and cost quota fields that are not edited by the consumer are preserved.
  Request windows map to the existing all-time, rolling minute/hour, and calendar
  day quota periods already enforced by the Orchestrator quota middleware.
- Total, daily, monthly, and single-request commercial budgets reuse
  `APIKeyCommercialLimits`; admission and billing continue charging the user's
  wallet and reject over-budget requests in the real request path.
- `/admin/account/api-key-models` returns only available model IDs and the
  effective Project/global sell-price rule. It does not return Channel IDs,
  provider credentials, upstream cost, account-pool state, or circuit-breaker
  diagnostics.
- `/project/api-keys` is now the personal-key workspace. Existing project/shared
  key management remains available to authorized owners at
  `/project/api-keys/shared`, and system administration remains unchanged.
- Browser acceptance registers two ordinary users, verifies cross-user mutation
  returns 403, creates a restricted key, proves an IP mismatch returns 401 on a
  real `/v1/chat/completions` request, and covers edit, disable, rotate, archive,
  one-time secret display, console errors, and `390x844` overflow.

Acceptance:

- Users cannot list, read, update, rotate, or delete another user's personal
  keys, including inside a shared Project.
- A key cannot target a Project outside the user's membership.
- Key limits are enforced in the real request path, not only displayed in UI.

### W5: Playground And Model Catalog

- [x] Make Playground a project-consumer capability using selected Project and a
  user-owned key or an equivalent server-side user principal.
- [x] Add a consumer-safe model catalog containing model ID, modality,
  availability, public price, and applicable project multiplier/rule summary.
- [x] Do not query global Channel administration from Playground.
- [x] Add explicit no-model, insufficient-balance, key-disabled, rate-limited,
  and upstream-unavailable states.
- [x] Preserve Orchestrator retry, circuit breaker, account-pool switching, and
  cross-Channel behavior.

Status: [x] Completed

Implementation notes:

- Consumer Playground uses `/admin/account/playground` and
  `/admin/account/playground/chat`; the existing administrator Playground route
  remains available for operational use.
- The state endpoint verifies Project membership, lists only the authenticated
  user's personal/user keys in that Project, evaluates expiry and IP policy,
  applies key model restrictions, and checks commercial admission without
  exposing Channel or upstream-account data.
- The model catalog returns model ID, display name, modality, availability,
  public price, currency, and only the effective global/project rule scope and
  pattern. Internal price references and upstream costs are omitted.
- A constrained authz delegation converts an authenticated user principal to a
  key principal only when the selected key belongs to that same user. The
  resulting request therefore reuses API-key quotas, request attribution,
  wallet billing, commercial key limits, and session scoping.
- Consumer chat removes all client-supplied Channel selection and invokes the
  existing `ChatCompletionOrchestrator`. Retry, circuit breaker, account-pool
  switching, cross-Channel selection, tracing, holds, and usage billing remain
  in the original request path.
- The frontend no longer queries global Channels or Models. It selects a usable
  personal key and chat model from the consumer state, shows deterministic
  blocking states, and remains usable without horizontal overflow at 390px.
- Browser acceptance creates an enabled `openai_fake` Channel, sends a streaming
  response through the real Orchestrator, verifies the no-model state before
  provisioning, and checks that Playground issues no Channel/Model admin query.

Acceptance:

- A new user can send a request when the platform has an available model and the
  user's commercial account permits it.
- An installation with no available model shows a deterministic empty state,
  not Access Denied.

### W6: User Requests, Usage, And Billing Projection

- [x] Define separate `my` and project-administration queries for request and
  usage data.
- [x] Default ordinary users to their own requests, usage records, API keys,
  model totals, spend, latency, and errors.
- [x] Allow project-wide visibility only through explicit project-admin
  capability.
- [x] Keep trace internals, raw upstream payloads, Channel/account identifiers,
  and sensitive errors out of the consumer projection.
- [x] Reuse hourly/daily aggregates for charts and detail records for audit.
- [x] Link every displayed charge to the user wallet ledger and Project price
  snapshot without changing wallet ownership.

Status: [x] Completed

Implementation notes:

- Consumer endpoints are split into `/admin/account/requests`,
  `/admin/account/usage`, detail/export variants, and explicit
  `/admin/account/project-*` administration endpoints. Every request requires
  an active Project membership; Project-wide access additionally requires
  system ownership, `read_requests`, Project ownership, a direct Project scope,
  or a Project role scope.
- `mine` request queries require an API key owned by the authenticated user.
  List, detail, aggregate, and CSV paths use the same ownership boundary, and
  encoded object GUIDs are supported safely in path parameters.
- The consumer projection includes the user's final request/response body,
  personal key name, tokens, latency, settled charge, billing record, wallet
  ledger transaction, and Project price snapshot. It omits request headers,
  client IP, Trace and execution internals, Channel and upstream-account IDs,
  raw upstream payloads/errors, upstream cost, and profit.
- Request details reuse `RequestService` so database and external-storage
  request/response bodies behave consistently. Charts use hourly aggregates for
  ranges up to 48 hours and daily aggregates for longer ranges; model totals and
  user charges come from the same aggregate rows.
- The Requests and Usage routes are consumer capabilities for every Project
  member. Only an explicitly authorized Project administrator sees the
  `Mine`/`Project` switch, while the backend remains the final authorization
  authority.
- Real streaming browser acceptance exposed that the streaming persistence path
  created `UsageLog` rows without invoking usage billing. Streaming and
  non-streaming persistence now share the same billing trigger. Computed usage
  charges are settled at micro-currency precision with positive sub-micro
  amounts rounded up; manually entered commercial money values retain strict
  six-decimal validation.
- E2E billing defaults to `warn` through `AXONHUB_E2E_BILLING_MODE`, leaving the
  production default unchanged while allowing browser tests to verify wallet,
  ledger, billing-record, price-snapshot, and aggregate reconciliation.

Acceptance:

- Cross-user and cross-Project isolation tests cover list, detail, aggregate,
  export, and guessed-ID access.
- Displayed user totals reconcile with usage billing and ledger records.

### W7: Navigation And Commercial Self-Service Closure

- [x] Reorganize ordinary-user navigation into Home, Workspaces, API Keys,
  Playground, Usage, Models & Prices, Wallet & Billing, and Profile.
- [x] Keep Project administration in a separate conditional group.
- [x] Keep system administration completely absent for normal users.
- [x] Split the current oversized billing page into scannable wallet, recharge,
  orders, subscriptions, redeem, affiliate, notifications, ledger, and usage
  views while retaining one accounting source of truth.
- [x] Verify desktop and mobile navigation, overflow, loading, empty, error, and
  disabled states.

Status: [x] Completed

Implementation notes:

- The primary Workspace navigation now has exactly eight consumer entries:
  Home, Workspaces, personal API Keys, Playground, Usage, Models & Prices,
  Wallet & Billing, and Profile. Project administration is rendered as a
  separate permission-aware group, while system administration remains a
  system-scope-only group and is absent for ordinary users.
- Project request administration moved to `/project/request-admin`; the
  consumer `/project/requests` projection remains user-scoped. Existing Project
  owners and explicitly authorized members retain the original request,
  Trace, Thread, member, role, prompt, and shared-key administration surfaces.
- `/project/models` reuses the personal-key model catalog endpoint and exposes
  only models available in the selected Project plus customer-facing price
  items and currency. Channel, upstream-account, routing, cost, and profit data
  remain outside the consumer projection.
- `/billing` is now a URL-driven shell with `wallet`, `recharge`, `orders`,
  `subscriptions`, `redeem`, `affiliate`, `notifications`, `ledger`, and
  `usage` views. All views continue to read the same `MyBillingOverview` query
  and use the existing billing mutations, so wallet balance, ledger, orders,
  subscriptions, notifications, and usage charges keep one accounting source
  of truth. The old `/billing` URL remains valid and defaults to `wallet`.
- Browser acceptance covers registration, personal-key creation, model and
  price discovery, a streaming request through an `openai_fake` Channel,
  consumer usage, settled billing records, wallet ledger visibility, all nine
  billing views, subscription and recharge entry points, Project/System admin
  compatibility, loading, empty, disabled, and forced error states.
- Desktop and 390x844 mobile checks cover billing navigation overflow and page
  width. Existing commercial lifecycle tests continue through the simulated
  ePay checkout, promo, redeem, subscription, affiliate, and paid-order flows.

Acceptance:

- A normal user can complete registration -> login -> create key -> inspect
  models/prices -> call AI -> inspect usage/charge -> recharge or subscribe.
- An administrator can still configure and operate all existing AxonHub
  commercial and routing modules.

### W8: Migration, Compatibility, And Release Gate

- [ ] Backfill or derive consumer capabilities for existing memberships without
  granting system scopes.
- [ ] Preserve existing API keys, Project IDs, pricing rules, wallets, ledgers,
  orders, subscriptions, and request history.
- [ ] Keep old routes as temporary redirects where bookmarks or external docs
  depend on them.
- [ ] Add PostgreSQL upgrade acceptance from the pre-refactor schema and current
  commercial data set.
- [ ] Add a browser release gate for owner, existing user, newly registered
  user, project member, suspended user, and no-channel installation.
- [ ] Update user documentation and administrator policy documentation.

Acceptance:

- Upgrade, restart, rollback-compatible backup, backend tests, frontend tests,
  production build, and Playwright gates pass.
- Existing AxonHub routing and account-pool acceptance remains green.

## TDD And Commit Protocol

For each workstream:

1. Add a failing backend unit/integration test or pure frontend unit test for
   the intended authorization and ownership behavior.
2. Implement the smallest domain change that makes the focused test pass.
3. Add or update the React route, state, and UI behavior.
4. Run TypeScript checking and production build.
5. Rebuild `scripts/e2e/axonhub-e2e` before backend-dependent Playwright tests.
6. Run the focused Playwright specification in desktop and mobile viewports
   where the work changes navigation or layout.
7. Run `git diff --check`, update the workstream checkbox, and commit only that
   workstream.

Required final release gate:

```text
go test ./internal/server/biz ./internal/server/gql ./internal/server/api ./internal/scopes -count=1
pnpm --dir frontend test:unit
pnpm --dir frontend exec tsc --noEmit
pnpm --dir frontend build
./scripts/e2e/e2e-test.sh commercial-user-workspace-smoke.spec.ts
./scripts/e2e/e2e-test.sh commercial-smoke.spec.ts
./scripts/e2e/e2e-test.sh upstream-accounts-smoke.spec.ts
```

## Immediate Refactor Order

The implementation order is W1 -> W2 -> W3 -> W4 -> W5 -> W6 -> W7 -> W8.
W1 is intentionally first because every later page depends on a single,
testable capability model and a valid post-login landing. W4 precedes W5 so the
Playground uses a proven user-owned credential boundary. W6 follows the real
request path so usage visibility is tested against actual ownership and billing
records rather than synthetic UI data.

## Workstream Completion Log

| Workstream | Commit | Date | Notes |
| --- | --- | --- | --- |
| W1 | this commit | 2026-07-10 | Unified route authorization, fixed project-owner and localized Admin navigation behavior, added the user home landing and safe redirect handling, and added unit/browser regression coverage. |
| W2 | this commit | 2026-07-10 | Added the tenant-scoped workspace summary, wallet/key/model/subscription/usage metrics, deterministic onboarding states, desktop/mobile browser coverage, GUID parsing, and missing Vite API proxies. |
| W6 | this commit | 2026-07-11 | Added isolated user/project request and usage REST projections, consumer Requests/Usage/detail/export UI, wallet-ledger and Project-price reconciliation, streaming billing completion, low-token micro settlement, and cross-user/browser regression coverage. |
