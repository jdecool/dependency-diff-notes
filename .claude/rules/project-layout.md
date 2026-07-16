# Go project structure

Based on [`golang-standards/project-layout`](https://github.com/golang-standards/project-layout), "a set of common historical and emerging project layout patterns" — this is not an official Go standard, but a convention widely adopted by the community.
We apply it with one guiding principle: **start simple and only grow the structure when there's a real need**.
Never create an empty directory "just in case."

## Directories used in this project

- **`/cmd`** — executable entry points, one subdirectory per binary (`cmd/{{BINARY}}/main.go`).
  Code in `main` must stay minimal: flag parsing, orchestration, exit code.
  All business logic lives in `internal/`.
- **`/internal`** — private code, whose import is **enforced by the Go compiler** outside the module.
  This is where the business packages live.

## Directories documented but not created (to adopt if the need arises)

- **`/pkg`** — public code meant to be reused by other projects.
  Only introduce it if part of the code genuinely needs to be importable from outside.
- **`/api`** — OpenAPI/Swagger specs or protocol definitions (if the project ever exposes an API).
- **`/configs`** — configuration templates or default values.
- **`/build`** — packaging and CI/CD configuration (currently `.goreleaser.yaml` and `.github/workflows/` at the root are enough).
- **`/deployments`** — infrastructure manifests (Docker, Kubernetes, Terraform…).
- **`/test`** — external test applications and large shared test data across several packages (today, `*_test.go` next to the code and local `testdata/` are enough, see `testing.md`).
- **`/docs`** — design and user documentation beyond `README.md`.
- **`/scripts`** — build/install/analysis scripts that don't belong in the Makefile.
- **`/tools`** — support tools that import `pkg/` or `internal/`.
- **`/examples`** — usage examples for the application or library.

## Other principles from the reference repo

- Go Modules for dependency management (already in place via `go.mod`).
- Follow official Go naming conventions and always run `gofmt` (see `go-style.md`).
- Avoid the Java-inherited `/src` style — Go doesn't use it.
