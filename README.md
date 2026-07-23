Dependency Diff Notes Bot
=========================

A bot that comments on a Change Request with updated dependencies.

It inspects a project's dependency state, determines what changed, and posts a single comment on the corresponding GitLab merge request or GitHub pull request reporting them.

**Supported ecosystems:** Composer (PHP), reading from `composer.lock`; npm (JavaScript), reading from `package-lock.json`; pnpm (JavaScript), reading from `pnpm-lock.yaml`; and Yarn (JavaScript), reading from `yarn.lock`.
A project using more than one at once (e.g. a Composer backend and an npm frontend) gets a single comment with one section per ecosystem — see [Ecosystem notes](docs/ecosystems.md).

## Installation

```bash
go install github.com/jdecool/dependency-diff-notes/cmd/dependency-diff-notes@latest
```

Or download a prebuilt binary from the [releases page](https://github.com/jdecool/dependency-diff-notes/releases).

A Docker image is also published to [`ghcr.io/jdecool/dependency-diff-notes`](https://github.com/jdecool/dependency-diff-notes/pkgs/container/dependency-diff-notes) on every release (`:<version>` and `:latest`, `linux/amd64` and `linux/arm64`).

## Quick start

In a CI pipeline the bot posts its report as a comment on the Change Request; run from a terminal it prints the same report to stdout.
The active Forge is detected automatically from the CI environment — GitHub Actions always sets `GITHUB_ACTIONS=true`; anything else is treated as GitLab — so no flag is needed to select one.

- **GitLab CI** — run it as a job in a merge request pipeline. See [GitLab CI](docs/gitlab-ci.md) and [`examples/gitlab-ci.yml`](examples/gitlab-ci.yml).
- **GitHub Actions** — run it as a step in a pull request workflow. See [GitHub Actions](docs/github-actions.md) and [`examples/github-actions.yml`](examples/github-actions.yml).
- **Local comparison** — preview the report from your terminal, no Forge or token needed: `dependency-diff-notes --target-branch main`. See [Local comparison](docs/local-comparison.md).

## What the comment looks like

Each ecosystem gets its own heading and a collapsible section holding one table per dependency group, with added, removed, upgraded, downgraded, and changed packages listed alphabetically:

```markdown
## Dependency changes

### Composer

| Package | Change | Version |
|---|---|---|
| symfony/console | ⬆️ Upgraded | v6.4.2 → v6.4.3 |
| phpunit/phpunit | ➕ Added | 10.5.9 |
```

See [Understanding the report](docs/report-anatomy.md) for the full anatomy and how the `Change` column is decided.

## Documentation

Full documentation lives in [`docs/index.md`](docs/index.md): setup and usage guides, the complete configuration reference, report options, ecosystem notes, and architecture decisions.

## Development

```bash
make build      # build the binary into dist/
make run        # run without building
make test       # run tests
make coverage   # run tests with coverage (HTML + JUnit reports)
```
