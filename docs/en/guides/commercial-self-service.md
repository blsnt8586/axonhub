# Commercial Self-Service

AxonHub commercial accounts use a user-owned wallet. Projects organize API
keys, requests, models, and price rules, but a Project does not own a separate
wallet.

## Account Lifecycle

1. Register from `/sign-up` when public registration is enabled.
2. Sign in and select a workspace. A newly registered account normally receives
   a default Project and personal API key according to the registration policy.
3. Open **Models & Prices** to confirm that the selected Project has an
   available model and a public sales price.
4. Create or restrict a personal API key under **API Keys**. The secret is shown
   only when the key is created or rotated.
5. Use **Playground** or an OpenAI/Anthropic-compatible client to send requests.
6. Review personal requests and aggregates under **Usage**. Consumer views do
   not expose Channels, upstream accounts, routing traces, upstream cost, or
   platform profit.
7. Use **Wallet & Billing** to inspect balance, recharge, orders,
   subscriptions, redeem codes, affiliate rebates, notifications, ledger
   movements, and usage charges.

## Billing Rules

- Recharge, redeem credits, subscriptions, refunds, and usage charges all post
  to the same user wallet ledger.
- Project and global price rules determine the customer charge. They never
  change wallet ownership.
- Subscription quota is consumed before wallet fallback when the plan covers
  the Project and model.
- A successful request can take a short time to appear in aggregate charts, but
  its request detail, usage billing record, and ledger transaction use the same
  settled charge.

## Access And Suspension

Every active Project member receives consumer capabilities to call AI, manage
their own personal keys, and view their own usage. These capabilities are
derived from active membership and do not require system administration
scopes.

Project administration appears only for a Project owner or a member with the
required Project scope. System administration is never granted by Project
ownership.

When an administrator deactivates a user, password login, existing JWT
sessions, and the user's `personal` or legacy `user` API keys are rejected.
Project `service_account` keys remain Project-owned operational credentials and
must be disabled separately when required.

## Empty Installation

If no enabled Channel exposes a model, Home and Playground show a deterministic
no-model state. Create or enable a Channel and configure a public price before
expecting users to call AI.
