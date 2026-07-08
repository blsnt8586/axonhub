---
alwaysApply: false
globs: "frontend/**/*.ts, frontend/**/*.tsx"
---

# Frontend General Rules

1. Do not restart the frontend development server; it is already managed.
2. Use `pnpm` as the package manager.
3. Verify changed frontend code before finishing. Use `pnpm exec tsc --noEmit` for type safety and targeted `pnpm exec eslint <changed files>` for changed TypeScript/TSX files.
4. Run `pnpm build` when changes affect pages, routes, GraphQL data flow, shared UI components, bundling, environment assumptions, or release behavior.
5. Full `pnpm lint` is allowed when useful, but do not hide unrelated existing failures. If full lint fails, run targeted eslint on changed files and report both the global failure and scoped result.
6. Prefer GraphQL input filters over client-side filtering when data should be filtered by the API.
7. When adding fields used by the UI, update the relevant GraphQL query and schema together.
8. Search filters should use debounce to avoid excessive requests.
9. When adding a new feature page, also add the corresponding route and sidebar entry if the feature should be navigable.
10. Use `extractNumberID` from `frontend/src/lib/utils.ts` to extract integer IDs from GUID values.
11. Respect page scoping semantics:
   Project-level pages must explicitly pass project context such as `projectId` or `X-Project-ID`.
   Admin-level pages must not implicitly inherit the current project unless the feature is intentionally project-scoped.
12. The app is client-side only; SSR compatibility is not required unless the code already depends on it.
13. For any field used by a create/edit form, update both write operations and read operations in the same change:
    mutations must send the field, and the queries used for edit echo, backfill, list refresh, or detail refresh must also return it when the UI depends on that value.
