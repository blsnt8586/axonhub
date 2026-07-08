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

Status: [ ] Not started

Goal: make the current wallet, ledger, payment, and usage billing implementation stable enough to become the foundation for later modules.

Backend scope:

- [ ] Refine billing admission errors into stable machine-readable codes: missing account, frozen account, closed account, insufficient balance, insufficient credit.
- [ ] Ensure failed usage billing records can be retried safely only when no ledger transaction was posted.
- [ ] Add owner-only admin queries for ledger transactions, usage billing records, payment orders, and payment events.
- [ ] Add filtering by user, project, API key, model, status, direction, provider, and time range where applicable.
- [ ] Confirm all wallet balance changes are ledger-backed and idempotent.

Frontend scope:

- [ ] Split admin billing into tabs: wallet accounts, ledger, usage charges, payment orders, payment events, sell price rules, payment providers.
- [ ] Add user billing filters and clear empty/error/loading states.
- [ ] Normalize currency and micros formatting in billing pages.
- [ ] Surface stable billing failure reasons in user-facing and admin-facing views.

Verification:

- [ ] Backend tests cover frozen/closed/insufficient-balance paths.
- [ ] Backend tests cover usage billing retry safety.
- [ ] Frontend lint/build passes for changed frontend files.
- [ ] Commit completed stage.

## Stage 1: Wallet Holds And Pre-Authorization

Status: [ ] Not started

Goal: prevent concurrent calls from overspending a wallet by reserving estimated cost before upstream execution and settling after usage is known.

Backend scope:

- [ ] Add `BillingHold` or `BillingAuthorization` entity.
- [ ] Hold fields include billing account, usage log/request reference, amount, currency, status, expires_at, idempotency key, and captured ledger transaction.
- [ ] Request admission creates a hold for estimated cost.
- [ ] Successful request captures actual usage cost and releases any remainder.
- [ ] Failed request releases the hold.
- [ ] Timeout cleanup releases or marks stale holds according to deterministic rules.
- [ ] Capture/release operations are idempotent.

Frontend scope:

- [ ] User wallet page shows balance, held amount, credit limit, and available amount.
- [ ] Admin billing page shows hold list and hold status.
- [ ] Admin can manually release abnormal holds with an audit reason.

Verification:

- [ ] Concurrent request tests prove wallet cannot overspend.
- [ ] Failure path releases hold.
- [ ] Duplicate capture/release is safe.
- [ ] Cleanup job is repeatable.
- [ ] Commit completed stage.

## Stage 2: Production Payment Order Lifecycle

Status: [ ] Not started

Goal: make ePay and future payment providers operable in production, with order expiry, make-up operations, event audit, and clear status handling.

Backend scope:

- [ ] Finalize payment statuses: pending, paid, failed, expired, canceled, refunded.
- [ ] Add order timeout expiry worker.
- [ ] Record all successful and failed payment notifications.
- [ ] Keep async notification idempotent.
- [ ] Reject invalid signature, provider mismatch, amount mismatch, currency mismatch, and expired order without crediting ledger.
- [ ] Add admin make-up payment operation with reason and audit trail.
- [ ] Add admin cancel pending order operation.
- [ ] Prepare refund fields without implementing full refund flow unless required by this stage.

Frontend scope:

- [ ] User payment return/status page shows read-only order result and refresh action.
- [ ] Admin payment order list supports filters and details.
- [ ] Admin payment event list shows notification payload summary and failure reason.
- [ ] Admin can make up or cancel eligible orders through guarded actions.

Verification:

- [ ] ePay notification credits once only.
- [ ] ePay return never credits ledger.
- [ ] Expired/canceled order cannot be credited by late notification.
- [ ] Manual make-up is idempotent and audited.
- [ ] Commit completed stage.

## Stage 3: Commercial Reports And Audit Console

Status: [ ] Not started

Goal: let operators reconcile revenue, usage charges, failed billing, and user-level money movement without querying the database directly.

Backend scope:

- [ ] Add daily recharge, consumption, net movement, and failed-payment aggregates.
- [ ] Add top models by charge amount.
- [ ] Add top projects by charge amount.
- [ ] Add top users by recharge and consumption.
- [ ] Add CSV export for ledgers, orders, usage charges, and payment events.
- [ ] Ensure aggregate totals reconcile with ledger transactions.

Frontend scope:

- [ ] Admin dashboard cards for recharge, consumption, net revenue, failed payments, and pending holds.
- [ ] Tables for ranked models, projects, and users.
- [ ] CSV export buttons on operational tables.
- [ ] Date range selector with stable default range.

Verification:

- [ ] Aggregate tests compare report totals against seeded ledger rows.
- [ ] Permission tests enforce owner-only access.
- [ ] Frontend build passes.
- [ ] Commit completed stage.

## Stage 4: API Key Budgets And Commercial Limits

Status: [ ] Not started

Goal: provide new-api-like API key budget controls while keeping the user wallet as the source of funds.

Backend scope:

- [ ] Add API key commercial budget configuration: total, daily, monthly, and single-request max.
- [ ] Add model allow/deny constraints where they do not duplicate existing profile behavior.
- [ ] Add project restriction support if missing from current API key/profile model.
- [ ] Admission checks wallet availability and API key budget together.
- [ ] Usage billing increments API key spend counters after charge.
- [ ] Periodic budget reset handles daily/monthly windows.

Frontend scope:

- [ ] API key create/edit form includes budget controls.
- [ ] API key list displays current spend and remaining budget.
- [ ] User can self-limit own keys; owner can override through admin controls.

Verification:

- [ ] Wallet sufficient but key over budget is denied.
- [ ] Key budget sufficient but wallet insufficient is denied.
- [ ] Daily/monthly reset is deterministic and repeatable.
- [ ] Commit completed stage.

## Stage 5: Redeem Codes

Status: [ ] Not started

Goal: support balance grants, compensation, and campaign distribution through auditable redeem codes.

Backend scope:

- [ ] Add redeem code entity with code, type, amount, status, created_by, used_by, used_at, expires_at, and notes.
- [ ] Support balance redeem first; reserve fields for credit/subscription redeem if needed.
- [ ] Support batch generation.
- [ ] Support admin create-and-redeem for a target user.
- [ ] Redeem success credits wallet through ledger transaction.
- [ ] Concurrent redeem of the same code can succeed only once.

Frontend scope:

- [ ] User redeem form and redeem history.
- [ ] Admin redeem code list with filters.
- [ ] Admin batch generation, disable, expire, delete, export, and create-and-redeem actions.

Verification:

- [ ] Used, disabled, and expired codes cannot be redeemed.
- [ ] Concurrent redeem credits only once.
- [ ] Ledger reference links redeem code to wallet credit.
- [ ] Commit completed stage.

## Stage 6: Subscription Plans

Status: [ ] Not started

Goal: sell plans and memberships in addition to pay-as-you-go wallet billing.

Backend scope:

- [ ] Add subscription plan entity with name, price, period, included quota, supported models/projects/groups, and enabled status.
- [ ] Add user subscription entity with user, plan snapshot, status, starts_at, expires_at, usage counters, and reset window.
- [ ] API admission checks active subscription before wallet fallback according to configured strategy.
- [ ] Support balance purchase of subscription plans.
- [ ] Support admin assign, extend, revoke, restore, and reset usage.
- [ ] Add expiry and periodic reset workers.

Frontend scope:

- [ ] User plan list, purchase flow, active subscriptions, and usage progress.
- [ ] Admin plan management.
- [ ] Admin user subscription management.

Verification:

- [ ] Active subscription can cover matching usage.
- [ ] Expired subscription is rejected or falls back to wallet according to policy.
- [ ] Usage reset works for supported periods.
- [ ] Balance purchase posts ledger debit.
- [ ] Commit completed stage.

## Stage 7: Promo Codes

Status: [ ] Not started

Goal: support marketing discounts for recharge and subscription purchase flows.

Backend scope:

- [ ] Add promo code entity with amount/percentage discount, scope, max uses, per-user limit, status, and expiry.
- [ ] Add promo usage records.
- [ ] Apply promo code during checkout creation.
- [ ] Verify payment amount against discounted order amount.
- [ ] Prevent discount from producing negative payable amount.

Frontend scope:

- [ ] User recharge and subscription checkout forms accept promo code.
- [ ] Show original amount, discount, and payable amount.
- [ ] Admin promo code management and usage records.

Verification:

- [ ] Disabled, expired, maxed, or reused promo codes are rejected.
- [ ] Discounted order payment verification uses payable amount.
- [ ] Commit completed stage.

## Stage 8: Affiliate And Rebates

Status: [ ] Not started

Goal: support referral growth and partner-style rebate settlement.

Backend scope:

- [ ] Add affiliate profile and invitation binding.
- [ ] Add rebate records tied to payment orders or subscription purchases.
- [ ] Support rebate freeze period.
- [ ] Support transferring thawed rebate to wallet through ledger credit.
- [ ] Support per-user rebate rate override.
- [ ] Prevent self-invite and circular inviter relationships.

Frontend scope:

- [ ] User affiliate page with invite code, invitees, frozen rebate, available rebate, and transfer action.
- [ ] Admin affiliate settings.
- [ ] Admin invite records, rebate records, transfer records, and per-user rate overrides.

Verification:

- [ ] Duplicate payment notification does not create duplicate rebate.
- [ ] Frozen rebate cannot be transferred early.
- [ ] Rebate transfer posts ledger transaction exactly once.
- [ ] Commit completed stage.

## Stage 9: Billing Notifications

Status: [ ] Not started

Goal: notify users and operators about balance, payment, subscription, and abnormal billing events.

Backend scope:

- [ ] Low wallet balance notifications.
- [ ] Payment success/failure notifications.
- [ ] Subscription expiry and expired notifications.
- [ ] Large consumption notification.
- [ ] Operator alerts for repeated payment callback failures and failed usage billing.
- [ ] Deduplicate notifications with event keys.

Frontend scope:

- [ ] User billing notification preferences.
- [ ] Admin commercial notification settings.
- [ ] Notification status visibility on relevant records where useful.

Verification:

- [ ] Notifications trigger at thresholds.
- [ ] Disabled preferences suppress notification.
- [ ] Repeated worker runs do not duplicate notifications.
- [ ] Commit completed stage.

## Stage 10: Production Operations And Security Closure

Status: [ ] Not started

Goal: close the operational and security gaps before production rollout.

Backend scope:

- [ ] Add audit records for pricing edits, provider edits, wallet controls, manual adjustments, make-up payments, order cancellation, redeem code operations, subscription operations, and affiliate overrides.
- [ ] Encrypt payment provider secrets at rest.
- [ ] Never return payment secrets through API responses.
- [ ] Add commercial mode configuration: disabled, warn, enforce.
- [ ] Add repeatable workers for order expiry, hold expiry, subscription expiry, subscription reset, rebate thawing, and failed billing retry.
- [ ] Add migration safety checks and default commercial settings.

Frontend scope:

- [ ] System settings page includes commercial billing mode and payment safety settings.
- [ ] Sensitive provider secret fields are write-only.
- [ ] Dangerous admin actions require confirmation and reason.
- [ ] Admin audit table surfaces key commercial actions.

Verification:

- [ ] Permission tests cover all commercial admin APIs.
- [ ] Secret fields are not returned by API.
- [ ] Workers are idempotent.
- [ ] Full selected backend tests and frontend build pass.
- [ ] Commit completed stage.

## Suggested Stage Order

- [ ] Stage 0: Harden Existing Commercial Modules
- [ ] Stage 1: Wallet Holds And Pre-Authorization
- [ ] Stage 2: Production Payment Order Lifecycle
- [ ] Stage 3: Commercial Reports And Audit Console
- [ ] Stage 4: API Key Budgets And Commercial Limits
- [ ] Stage 5: Redeem Codes
- [ ] Stage 6: Subscription Plans
- [ ] Stage 7: Promo Codes
- [ ] Stage 8: Affiliate And Rebates
- [ ] Stage 9: Billing Notifications
- [ ] Stage 10: Production Operations And Security Closure

## Completion Log

Append one line per completed stage.

| Stage | Commit | Date | Notes |
| --- | --- | --- | --- |
| Baseline | `4ad0913d` and earlier billing commits | 2026-07-08 | Wallet, ledger, ePay, sell price rules, user/admin billing pages, and user wallet controls are in place. |
