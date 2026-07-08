---
alwaysApply: false
globs: "**/*.go"
---

# Go General Rules

## Development Workflow

1. The development server is managed by `air`; do not restart it manually.
2. Run `go test` for affected packages before finishing backend changes. Broaden to neighboring packages or `go test ./...` when shared behavior, generated code, middleware, or cross-module contracts changed.
3. Run `make generate` after Ent or GraphQL schema changes, then test the affected generated-code consumers.
4. Run `go build`, `make build-backend`, or `make build` when the change affects startup wiring, release packaging, embedded assets, build tags, generated code, or command entrypoints.
5. `golangci-lint run` is allowed when useful, but prefer targeted tests first. If full lint fails because of unrelated existing issues, report that distinction instead of treating the current task as failed.
6. Use the owning Go module for commands. In particular, run Go commands for `llm/` from the `llm/` directory.

## Coding Conventions

1. Prefer `github.com/samber/lo` for collection, slice, map, and pointer helpers.
2. Do not add handwritten pointer helper functions such as `stringPtr`; use `lo.ToPtr(...)` or `new(T)` when appropriate.
3. It is acceptable to use `lo.ToPtr(...)` for constants or literals that cannot be addressed directly.
4. Follow the existing FX dependency injection patterns.
5. Use structured logging with zap.
6. Propagate `context.Context` correctly through request and service boundaries.
7. Handle errors with the unified helpers in `internal/pkg/xerrors` and wrap them with useful context.
8. Any manually started goroutine (`go func(...) { ... }(...)` or equivalent) must install a top-level `defer recover()` guard that logs the panic before the goroutine exits.
