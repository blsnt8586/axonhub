# Commercial Access Policy

This document defines the administrator-facing ownership and authorization
contract for the commercial fork.

## Navigation And Authorization Levels

| Level | Purpose | Grant source |
| --- | --- | --- |
| Consumer workspace | Home, workspaces, personal API keys, Playground, personal usage, model catalog, wallet, profile | Active user plus active Project membership |
| Project administration | Shared keys, prompts, project requests, traces, threads, members, roles | Project ownership, direct Project scope, or Project role scope |
| System administration | Projects, Channels, upstream accounts, system models, users, roles, billing administration, system settings | System owner or explicit system scope |

Consumer capabilities are derived at request time. Do not backfill system
scopes into historical memberships merely to let existing members call AI.

## Ownership Contract

- The billing account and wallet belong to the user.
- A Project owns resource isolation and price context, not user funds.
- A personal API key belongs to one user inside one Project.
- A service account belongs to Project operations even though its creator is
  recorded for audit.
- Usage charges must retain the user, Project, API key, price snapshot, billing
  record, and ledger transaction references.

## Suspension Policy

Deactivating a user invalidates password login, JWT authentication, and
user-owned `personal` or legacy `user` API keys. It does not automatically
disable Project `service_account` keys. Administrators must review shared
service accounts separately during offboarding.

Archiving a Project invalidates all API keys attached to that Project. Disabling
or archiving an individual API key affects only that credential.

## Upgrade And Rollback Policy

Before upgrading:

1. Stop writes and create a PostgreSQL dump from the current release.
2. Record the image digest, Git tag, schema baseline, and payment encryption
   key location.
3. Restore the dump into an isolated database and run the commercial release
   role matrix.
4. Upgrade with one migration-capable instance before scaling out.

The release manifest must preserve users, Project IDs, memberships, API key
identity, price rules, wallets, ledgers, orders, subscriptions, requests, usage
logs, usage billing records, and their references. Roll back application code
without restoring the database only when the older binary is schema-compatible;
otherwise restore the verified pre-release dump.

Run `./scripts/e2e/commercial-release-gate.sh --full` for the complete automated
gate.
