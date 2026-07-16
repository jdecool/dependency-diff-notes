# Go code style

- **Formatting**: always run `gofmt -w .` before committing.
  `go vet ./...` must stay clean.
- **Naming**: `PascalCase` for exported identifiers, `camelCase` for locals.
  Package names short, lowercase, no underscore.
- **Errors**: propagate them with context via `fmt.Errorf("action: %w", err)` (never a concatenated `err.Error()`).
  No `panic` in the normal flow — reserved for unrecoverable programming errors.
- **Packages**: cohesive and single-responsibility (see `project-layout.md` for the `cmd/` / `internal/` organization).
  Unexported code not meant to be reused goes in `internal/`.
- **Dependencies**: don't add an external dependency without a real need — prefer the standard library.
  Any new runtime dependency must be justified by an ADR in `docs/adr/`.
- **Pure functions when possible**: business logic should avoid side effects (I/O, exec) to stay easily testable in a table-driven style — see `testing.md`.
