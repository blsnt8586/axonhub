# AxonHub Commercial Billing Roadmap

This document tracks the staged plan for turning AxonHub into a production-grade commercial API gateway. Each stage should be delivered with backend and frontend changes together, verified with tests, and committed independently so the project can roll back by stage.

## Execution Rules

- Use TDD for every stage: write focused backend tests first, then implement service/API logic, then frontend data hooks and UI.
- Keep one business module per stage. Do not mix unrelated billing, payment, subscription, or affiliate work in the same commit.
- Every completed stage must update this checklist from `[ ]` to `[x]` in the same commit or in an immediate follow-up docs commit.
- User wallet is the commercial billing subject by default. Project rules affect pricing, but user wallet pays the charge.
- Wallet balance must not be edited directly. Balance changes must go through ledger transactions.
- Payment return pages are read-only. Only verified async notifications or explicit admin make-up operations can credit a wallet.
- Keep AxonHub's existing routing, scheduling, and circuit-breaker path intact unless a stage explicitly requires a narrowly scoped integration point.

## Current Baseline

- [x] User billing account model exists.
- [x] Ledger transaction and ledger entry model exists.
- [x] Payment order, payment event, and payment provider instance model exists.
- [x] ePay-compatible checkout and simulated ePay provider exist.
- [x] User wallet recharge checkout exists.
- [x] Usage billing can charge user wallets from usage logs.
- [x] Sell price rules support global and project scopes.
- [x] User wallet frontend exists.
- [x] Admin billing console exists.
- [x] Admin can adjust user wallet balance through audited ledger transactions.
- [x] Admin can freeze, close, reopen, and set credit limit for user billing accounts.

## Stage 0: Harden Existing Commercial Modules

Status: [x] Completed

Goal: make the current wallet, ledger, payment, and usage billing implementation stable enough to become the foundation for later modules.

Backend scope:

- [x] Refine billing admission errors into stable machine-readable codes: missing account, frozen account, closed account, insufficient balance, insufficient credit.
- [x] Ensure failed usage billing records can be retried safely only when no ledger transaction was posted.
- [x] Add owner-only admin queries for ledger transactions, usage billing records, payment orders, and payment events.
- [x] Add filtering by user, project, API key, model, status, direction, provider, and time range where applicable.
- [x] Confirm all wallet balance changes are ledger-backed and idempotent.

Frontend scope:

- [x] Split admin billing into tabs: wallet accounts, ledger, usage charges, payment orders, payment events, sell price rules, payment providers.
- [x] Add user billing filters and clear empty/error/loading states.
- [x] Normalize currency and micros formatting in billing pages.
- [x] Surface stable billing failure reasons in user-facing and admin-facing views.

Verification:

- [x] Backend tests cover frozen/closed/insufficient-balance paths.
- [x] Backend tests cover usage billing retry safety.
- [x] Frontend TypeScript check passes for changed frontend files. Lint/build were not run because `AGENTS.md` forbids them unless explicitly requested.
- [x] Commit completed stage.

## Stage 1: Wallet Holds And Pre-Authorization

Status: [x] Completed

Goal: prevent concurrent calls from overspending a wallet by reserving estimated cost before upstream execution and settling after usage is known.

Backend scope:

- [x] Add `BillingHold` or `BillingAuthorization` entity.
- [x] Hold fields include billing account, usage log/request reference, amount, currency, status, expires_at, idempotency key, and captured ledger transaction.
- [x] Request admission creates a hold for estimated cost.
- [x] Successful request captures actual usage cost and releases any remainder.
- [x] Failed request releases the hold.
- [x] Timeout cleanup releases or marks stale holds according to deterministic rules.
- [x] Capture/release operations are idempotent.

Frontend scope:

- [x] User wallet page shows balance, held amount, credit limit, and available amount.
- [x] Admin billing page shows hold list and hold status.
- [x] Admin can manually release abnormal holds with an audit reason.

Verification:

- [x] Concurrent request tests prove wallet cannot overspend.
- [x] Failure path releases hold.
- [x] Duplicate capture/release is safe.
- [x] Cleanup job is repeatable.
- [x] Commit completed stage.

## Stage 2: Production Payment Order Lifecycle

Status: [x] Completed

Goal: make ePay and future payment providers operable in production, with order expiry, make-up operations, event audit, and clear status handling.

Backend scope:

- [x] Finalize payment statuses: pending, paid, failed, expired, canceled, refunded.
- [x] Add order timeout expiry worker.
- [x] Record all successful and failed payment notifications.
- [x] Keep async notification idempotent.
- [x] Reject invalid signature, provider mismatch, amount mismatch, currency mismatch, and expired order without crediting ledger.
- [x] Add admin make-up payment operation with reason and audit trail.
- [x] Add admin cancel pending order operation.
- [x] Prepare refund fields without implementing full refund flow unless required by this stage.

Frontend scope:

- [x] User payment return/status page shows read-only order result and refresh action.
- [x] Admin payment order list supports filters and details.
- [x] Admin payment event list shows notification payload summary and failure reason.
- [x] Admin can make up or cancel eligible orders through guarded actions.

Verification:

- [x] ePay notification credits once only.
- [x] ePay return never credits ledger.
- [x] Expired/canceled order cannot be credited by late notification.
- [x] Manual make-up is idempotent and audited.
- [x] Commit completed stage.

## Stage 3: Commercial Reports And Audit Console

Status: [x] Completed

Goal: let operators reconcile revenue, usage charges, failed billing, and user-level money movement without querying the database directly.

Backend scope:

- [x] Add daily recharge, consumption, net movement, and failed-payment aggregates.
- [x] Add top models by charge amount.
- [x] Add top projects by charge amount.
- [x] Add top users by recharge and consumption.
- [x] Add CSV export for ledgers, orders, usage charges, and payment events.
- [x] Ensure aggregate totals reconcile with ledger transactions.

Frontend scope:

- [x] Admin dashboard cards for recharge, consumption, net revenue, failed payments, and pending holds.
- [x] Tables for ranked models, projects, and users.
- [x] CSV export buttons on operational tables.
- [x] Date range selector with stable default range.

Verification:

- [x] Aggregate tests compare report totals against seeded ledger rows.
- [x] Permission tests enforce owner-only access.
- [x] Frontend build passes.
- [x] Commit completed stage.

## Stage 4: API Key Budgets And Commercial Limits

Status: [x] Completed

Goal: provide new-api-like API key budget controls while keeping the user wallet as the source of funds.

Backend scope:

- [x] Add API key commercial budget configuration: total, daily, monthly, and single-request max.
- [x] Keep model restrictions on the existing API key profile `modelIDs` path instead of duplicating allow/deny fields.
- [x] Keep project restriction on the existing API key `project_id` path.
- [x] Admission checks wallet availability and API key budget together.
- [x] Usage billing records charged API key spend as the source of truth after charge.
- [x] Daily/monthly budget reset is handled by deterministic usage windows.

Frontend scope:

- [x] API key create/edit form includes budget controls.
- [x] API key list displays current spend and remaining budget.
- [x] User can self-limit own keys; owner can override through API key management controls.

Verification:

- [x] Wallet sufficient but key over budget is denied.
- [x] Key budget sufficient but wallet insufficient is denied.
- [x] Daily/monthly reset is deterministic and repeatable.
- [x] Frontend typecheck, targeted lint, and production build pass.
- [x] Commit completed stage.

## Stage 5: Redeem Codes

Status: [x] Completed

Goal: support balance grants, compensation, and campaign distribution through auditable redeem codes.

Backend scope:

- [x] Add redeem code entity with code, type, amount, status, created_by, used_by, used_at, expires_at, and notes.
- [x] Support balance redeem first; reserve fields for credit/subscription redeem if needed.
- [x] Support batch generation.
- [x] Support admin create-and-redeem for a target user.
- [x] Redeem success credits wallet through ledger transaction.
- [x] Concurrent redeem of the same code can succeed only once.

Frontend scope:

- [x] User redeem form and redeem history.
- [x] Admin redeem code list with filters.
- [x] Admin batch generation, disable, expire, delete, export, and create-and-redeem actions.

Verification:

- [x] Used, disabled, and expired codes cannot be redeemed.
- [x] Concurrent redeem credits only once.
- [x] Ledger reference links redeem code to wallet credit.
- [x] Frontend typecheck and production build pass.
- [x] Commit completed stage.

## Stage 6: Subscription Plans

Status: [x] Completed

Goal: sell plans and memberships in addition to pay-as-you-go wallet billing.

Backend scope:

- [x] Add subscription plan entity with name, price, period, included quota, supported models/projects/groups, and enabled status.
- [x] Add user subscription entity with user, plan snapshot, status, starts_at, expires_at, usage counters, and reset window.
- [x] API admission checks active subscription before wallet fallback according to configured strategy.
- [x] Support balance purchase of subscription plans.
- [x] Support admin assign, extend, revoke, restore, and reset usage.
- [x] Add expiry and periodic reset workers.

Frontend scope:

- [x] User plan list, purchase flow, active subscriptions, and usage progress.
- [x] Admin plan management.
- [x] Admin user subscription management.

Verification:

- [x] Active subscription can cover matching usage.
- [x] Expired subscription is rejected or falls back to wallet according to policy.
- [x] Usage reset works for supported periods.
- [x] Balance purchase posts ledger debit.
- [x] Commit completed stage.

## Stage 7: Promo Codes

Status: [x] Completed

Goal: support marketing discounts for recharge and subscription purchase flows.

Backend scope:

- [x] Add promo code entity with amount/percentage discount, scope, max uses, per-user limit, status, and expiry.
- [x] Add promo usage records.
- [x] Apply promo code during checkout creation.
- [x] Verify payment amount against discounted order amount.
- [x] Prevent discount from producing negative payable amount.

Frontend scope:

- [x] User recharge and subscription checkout forms accept promo code.
- [x] Show original amount, discount, and payable amount.
- [x] Admin promo code management and usage records.

Verification:

- [x] Disabled, expired, maxed, or reused promo codes are rejected.
- [x] Discounted order payment verification uses payable amount.
- [x] Commit completed stage.

## Stage 8: Affiliate And Rebates

Status: [x] Completed

Goal: support referral growth and partner-style rebate settlement.

Backend scope:

- [x] Add affiliate profile and invitation binding.
- [x] Add rebate records tied to payment orders or subscription purchases.
- [x] Support rebate freeze period.
- [x] Support transferring thawed rebate to wallet through ledger credit.
- [x] Support per-user rebate rate override.
- [x] Prevent self-invite and circular inviter relationships.

Frontend scope:

- [x] User affiliate page with invite code, invitees, frozen rebate, available rebate, and transfer action.
- [x] Admin affiliate settings.
- [x] Admin invite records, rebate records, transfer records, and per-user rate overrides.

Verification:

- [x] Duplicate payment notification does not create duplicate rebate.
- [x] Frozen rebate cannot be transferred early.
- [x] Rebate transfer posts ledger transaction exactly once.
- [x] Commit completed stage.

## Stage 9: Billing Notifications

Status: [x] Completed

Goal: notify users and operators about balance, payment, subscription, and abnormal billing events.

Backend scope:

- [x] Low wallet balance notifications.
- [x] Payment success/failure notifications.
- [x] Subscription expiry and expired notifications.
- [x] Large consumption notification.
- [x] Operator alerts for repeated payment callback failures and failed usage billing.
- [x] Deduplicate notifications with event keys.

Frontend scope:

- [x] User billing notification preferences.
- [x] Admin commercial notification settings.
- [x] Notification status visibility on relevant records where useful.

Verification:

- [x] Notifications trigger at thresholds.
- [x] Disabled preferences suppress notification.
- [x] Repeated worker runs do not duplicate notifications.
- [x] Commit completed stage.

## Stage 10: Production Operations And Security Closure

Status: [x] Completed

Goal: close the operational and security gaps before production rollout.

Backend scope:

- [x] Add audit records for pricing edits, provider edits, wallet controls, manual adjustments, make-up payments, order cancellation, redeem code operations, subscription operations, and affiliate overrides.
- [x] Encrypt payment provider secrets at rest.
- [x] Never return payment secrets through API responses.
- [x] Add commercial mode configuration: disabled, warn, enforce.
- [x] Add repeatable workers for order expiry, hold expiry, subscription expiry, subscription reset, rebate thawing, and failed billing retry.
- [x] Add migration safety checks and default commercial settings.

Frontend scope:

- [x] System settings page includes commercial billing mode and payment safety settings.
- [x] Sensitive provider secret fields are write-only.
- [x] Dangerous admin actions require confirmation and reason.
- [x] Admin audit table surfaces key commercial actions.

Verification:

- [x] Permission tests cover all commercial admin APIs.
- [x] Secret fields are not returned by API.
- [x] Workers are idempotent.
- [x] Full selected backend tests and frontend build pass.
- [x] Commit completed stage.

## Suggested Stage Order

- [x] Stage 0: Harden Existing Commercial Modules
- [x] Stage 1: Wallet Holds And Pre-Authorization
- [x] Stage 2: Production Payment Order Lifecycle
- [x] Stage 3: Commercial Reports And Audit Console
- [x] Stage 4: API Key Budgets And Commercial Limits
- [x] Stage 5: Redeem Codes
- [x] Stage 6: Subscription Plans
- [x] Stage 7: Promo Codes
- [x] Stage 8: Affiliate And Rebates
- [x] Stage 9: Billing Notifications
- [x] Stage 10: Production Operations And Security Closure
- [ ] Stage 11: Production Deployment And End-to-End Acceptance
- [ ] Stage 12+: Commercial Platform Enhancement Roadmap ([commercial-platform-enhancement-roadmap.md](./commercial-platform-enhancement-roadmap.md))

## Stage 11: Production Deployment And End-to-End Acceptance

Status: [ ] In Progress

Goal: verify the commercial billing build as a deployable product, including configuration defaults, local runtime startup, browser-level owner/user acceptance, Docker readiness, and rollback-friendly evidence.

Tracking checklist:

- [x] Maintain the detailed production checklist in [commercial-billing-production-checklist.md](./commercial-billing-production-checklist.md).
- [x] Lock the default commercial payer to user wallet in configuration.
- [x] Run backend verification for commercial billing and configuration.
- [x] Run frontend TypeScript and production build verification.
- [x] Start backend and frontend locally with an isolated Stage 11 database.
- [x] Complete browser-level admin billing acceptance.
- [x] Complete browser-level user wallet and simulated recharge acceptance.
- [x] Fix acceptance defects found during Stage 11 browser verification.
- [x] Record unresolved production gaps explicitly in the production checklist.
- [ ] Commit completed stage.

## Completion Log

Append one line per completed stage.

| Stage | Commit | Date | Notes |
| --- | --- | --- | --- |
| Baseline | `4ad0913d` and earlier billing commits | 2026-07-08 | Wallet, ledger, ePay, sell price rules, user/admin billing pages, and user wallet controls are in place. |
| Stage 0 | `f96625b4` | 2026-07-08 | Hardened admission codes, usage retry safety, owner-only billing operation queries, admin billing tabs, filters, and failure reason display. |
| Stage 1 | `94106a72` | 2026-07-08 | Added wallet holds, request pre-authorization, hold capture/release/expiry, admin hold operations, and held/available wallet UI. |
| Stage 2 | `bf4368b5` | 2026-07-08 | Hardened payment order lifecycle with expiry, cancel, admin make-up, callback audit, read-only return page, event payload summaries, and refund reserve fields. |
| Stage 3 | `482fad12` | 2026-07-08 | Added owner-only commercial reports, ledger-backed reconciliation totals, ranked model/project/user tables, CSV exports, admin report dashboard, and frontend verification. |
| Stage 9 | current stage commit | 2026-07-09 | Added station billing notifications, user preferences, admin notification settings and logs, event-key deduplication, business triggers, and frontend verification. |
| Stage 10 | current stage commit | 2026-07-09 | Added production commercial settings, billing audit logs, secret encryption checks, manual maintenance controls, idempotent worker closure, and frontend operations UI verification. |
