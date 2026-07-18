# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

@.claude/rules/project-layout.md
@.claude/rules/go-style.md
@.claude/rules/testing.md
@.claude/rules/git-workflow.md

## Commands

```bash
# Run the bot
make run             # go run ./cmd/dependency-diff-notes
go build -o dist/dependency-diff-notes ./cmd/dependency-diff-notes

# Formatting and vet
gofmt -w .
go vet ./...

# Tests
make test            # go test ./...
make coverage        # HTML + JUnit reports in coverage/
```

## Architecture

A bot that comments on a merge request with updated dependencies: it inspects a project's dependency state (Composer/PHP from `composer.lock`, npm/JavaScript from `package-lock.json`, pnpm/JavaScript from `pnpm-lock.yaml`, Yarn/JavaScript from `yarn.lock`), determines what changed per Ecosystem, and posts a single comment on the corresponding merge request reporting them.

**Layout** (see `.claude/rules/project-layout.md` for details, based on [`golang-standards/project-layout`](https://github.com/golang-standards/project-layout)):

- `cmd/dependency-diff-notes` — CLI entry point.
- `internal/...` — TODO: list packages as they get built (e.g. Composer parsing, dependency update detection, merge request commenting).

## Conventions

- See `.claude/rules/` for details (Go style, tests, structure, git).
- No external dependency without an ADR: prefer the standard library.
