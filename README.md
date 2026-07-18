Dependency Diff Notes Bot
=========================

A bot that comments on a Change Request with updated dependencies.

It inspects a project's dependency state, determines what changed, and posts a comment on the corresponding GitLab merge request or GitHub pull request reporting them.

> **Supported ecosystems:** Composer (PHP), reading from `composer.lock`; npm (JavaScript), reading from `package-lock.json`; pnpm (JavaScript), reading from `pnpm-lock.yaml`; and Yarn (JavaScript), reading from `yarn.lock`. A project using more than one at once (e.g. a Composer backend and an npm frontend in the same repository) gets a single Bot Comment with one section per Ecosystem — but a project with more than one JavaScript package manager's Lockfile present at once (e.g. both `package-lock.json` and `pnpm-lock.yaml`) is treated as a conflict and fails the run, since a project uses at most one at a time — unless you tell the bot which one is real with `--ecosystems` (see Configuration below).

## Installation

```bash
go install github.com/jdecool/dependency-diff-notes/cmd/dependency-diff-notes@latest
```

Or download a prebuilt binary from the [releases page](https://github.com/jdecool/dependency-diff-notes/releases).

A Docker image is also published to [`ghcr.io/jdecool/dependency-diff-notes`](https://github.com/jdecool/dependency-diff-notes/pkgs/container/dependency-diff-notes) on every release (`:<version>` and `:latest`, `linux/amd64` and `linux/arm64`) — see the GitLab CI section below for the primary use case.

## Usage

```bash
dependency-diff-notes
```

In a CI pipeline it posts its report as a comment on the Change Request; run from a terminal it prints the same report to stdout (see [Local comparison](#local-comparison)).

The bot supports two Forges (see CONTEXT.md): GitLab and GitHub.
Which one is active is detected automatically from the CI environment — GitHub Actions always sets `GITHUB_ACTIONS=true`; anything else is treated as GitLab — so no flag or extra configuration is needed to select one.

### GitLab CI

Run as a job in a consumer project's merge request pipeline, the bot compares each active Ecosystem's Lockfile (`composer.lock`, `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`) between the merge-base of the target branch and the current commit (see the description at the top of this README), and creates or updates a single note on the merge request reporting the dependency changes found.

See [`examples/gitlab-ci.yml`](examples/gitlab-ci.yml) for a copy-pasteable job.

Outside of a merge request pipeline (`--request-iid`/`CI_MERGE_REQUEST_IID` empty), where `CI_MERGE_REQUEST_TARGET_BRANCH_NAME` is also empty, the bot prints a message and exits `0` without doing anything — it's safe to leave the job unrestricted by `rules` if you'd rather not gate it on `$CI_PIPELINE_SOURCE`. (Passing a `--target-branch` explicitly in that situation runs a [local comparison](#local-comparison) instead.)

### GitHub Actions

Run as a step in a consumer project's pull request workflow, the bot compares each active Ecosystem's Lockfile (`composer.lock`, `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`) between the merge-base of the base branch and the current commit, and creates or updates a single comment on the pull request reporting the dependency changes found.

See [`examples/github-actions.yml`](examples/github-actions.yml) for a copy-pasteable workflow.
Note:

- The pull request number is parsed from `GITHUB_REF` (`refs/pull/<number>/merge`), so no extra configuration is needed beyond triggering on `pull_request`.
- `actions/checkout` must be configured to fetch the base branch's history (e.g. `fetch-depth: 0`), the same way GitLab CI jobs need `GIT_DEPTH: 0` — the bot never runs `git fetch` itself.
- GitHub Enterprise Server is not supported; the bot always talks to `https://api.github.com`.
- The step needs `permissions: pull-requests: write` (or a token with equivalent scope) to create/update comments.

### Local comparison

Run from a terminal - outside any Change Request pipeline - the bot previews the dependency changes it would report, printing them to stdout instead of posting a comment.
This mode is auto-detected: when there is no Change Request context (no `--request-iid`/`CI_MERGE_REQUEST_IID`) but a base branch is given via `--target-branch`, the bot runs a local comparison.
No Forge, token, or project ID is needed.

```bash
# Compare your uncommitted working tree against the merge-base with main:
dependency-diff-notes --target-branch main

# Compare the current branch's committed HEAD instead of the working tree:
dependency-diff-notes --target-branch main --source HEAD

# Compare a specific commit or tag against another branch:
dependency-diff-notes --target-branch develop --source v1.4.0
```

The comparison uses the same merge-base semantics as CI, so the output is a faithful preview of the comment the bot would post on a Change Request.
Two things differ from CI: `--target-branch` is used **literally** (a local branch, tag, or SHA - it is not resolved through `origin/`, so no `git fetch` is required), and the `--source` (the new side) defaults to the on-disk working tree, including uncommitted changes.
The command always exits `0` on success whether or not changes were found (see `docs/adr/0006-local-comparison-mode.md`).

### Configuration

Configuration is resolved from CLI flags first, then from the matching environment variable of the detected Forge (predefined CI variables in most cases), then a default if any:

| Flag                   | Purpose                          | GitLab environment variable            | GitHub environment variable | Default          |
|------------------------|-----------------------------------|------------------------------------------|-------------------------------|-------------------|
| `--server-url`         | GitLab server URL (unused on GitHub) | `CI_SERVER_URL`                     | —                              | none (required in Change Request context on GitLab) |
| `--project-id`         | GitLab project ID, or GitHub `owner/repo` | `CI_PROJECT_ID`                | `GITHUB_REPOSITORY`            | none (required in Change Request context) |
| `--request-iid`        | Merge Request IID or Pull Request number | `CI_MERGE_REQUEST_IID`          | parsed from `GITHUB_REF`       | none — empty means "not in a Change Request" |
| `--target-branch`      | Change Request target branch; outside a Change Request, the base branch of a local comparison | `CI_MERGE_REQUEST_TARGET_BRANCH_NAME`    | `GITHUB_BASE_REF`              | none (required in Change Request context; triggers local comparison outside one) |
| `--source`             | Local comparison only: the new side to compare against `--target-branch` | none                                      | none                           | empty — the on-disk working tree |
| `--token`              | Forge API token                   | `DEPENDENCY_DIFF_NOTES_TOKEN`              | `GITHUB_TOKEN`                 | none (required in Change Request context) |
| `--composer-lock-path` | Path to `composer.lock`           | `DEPENDENCY_DIFF_NOTES_COMPOSER_LOCK_PATH` | `DEPENDENCY_DIFF_NOTES_COMPOSER_LOCK_PATH` | `composer.lock` |
| `--npm-lock-path`      | Path to `package-lock.json`       | `DEPENDENCY_DIFF_NOTES_NPM_LOCK_PATH`      | `DEPENDENCY_DIFF_NOTES_NPM_LOCK_PATH` | `package-lock.json` |
| `--pnpm-lock-path`     | Path to `pnpm-lock.yaml`          | `DEPENDENCY_DIFF_NOTES_PNPM_LOCK_PATH`     | `DEPENDENCY_DIFF_NOTES_PNPM_LOCK_PATH` | `pnpm-lock.yaml` |
| `--yarn-lock-path`     | Path to `yarn.lock`               | `DEPENDENCY_DIFF_NOTES_YARN_LOCK_PATH`     | `DEPENDENCY_DIFF_NOTES_YARN_LOCK_PATH` | `yarn.lock` |
| `--ecosystems`         | Comma-separated Ecosystems to consider, e.g. `composer,pnpm` | `DEPENDENCY_DIFF_NOTES_ECOSYSTEMS` | `DEPENDENCY_DIFF_NOTES_ECOSYSTEMS` | all present |
| `--repo-dir`           | Path to the repository checkout   | none                                      | none                           | `.` |

`--server-url` (GitLab only), `--project-id`, `--target-branch`, and `--token` are only required once the bot detects it's running in a Change Request context (i.e. `--request-iid` resolves to a non-empty value, from the flag, `CI_MERGE_REQUEST_IID`, or `GITHUB_REF`).
For a [local comparison](#local-comparison) (no Change Request context) only `--target-branch` is needed; `--source` is optional, and no Forge, token, or project ID applies.
`--composer-lock-path`, `--npm-lock-path`, `--pnpm-lock-path`, `--yarn-lock-path`, and `--repo-dir` always have safe defaults and can be left unset in most setups — each Ecosystem's Lockfile is read at its default path and simply contributes no changes if the file doesn't exist, so a project using only one Ecosystem doesn't need to configure anything about the others.

`--ecosystems` is an allowlist restricting the Ecosystems the bot considers for the run (tokens `composer`, `npm`, `pnpm`, `yarn`, case-insensitive; unknown tokens fail fast).
Left unset, every Ecosystem is considered — the historical behavior.
Its main use is resolving the JavaScript Lockfile conflict above: on a project that carries both `package-lock.json` and `pnpm-lock.yaml`, set `--ecosystems=composer,pnpm` (or just `pnpm`) to tell the bot which JavaScript package manager is real, and the stray Lockfile is ignored for the whole run.
The restriction is permanent, not consulted only on conflict, so an excluded Ecosystem is dropped at every ref (see `docs/adr/0005-ecosystem-allowlist.md`).

### pnpm lockfileVersion support

The bot reads `pnpm-lock.yaml` across pnpm's three lockfile formats: `5.x`, `6.0`, and `9.0` (there is no `7.0`/`8.0` lockfileVersion, despite pnpm CLI majors 7 and 8 existing). lockfileVersion 9.0 (pnpm ≥9, the current major version) dropped the per-package `dev` flag entirely — see `docs/pnpm-lockfile-schema.md` — so a pnpm section only gets the Production/Development split on `5.x`/`6.0` Lockfiles; a `9.0` Lockfile reports a single undifferentiated Dependencies group instead, unlike Composer's `composer.lock` and npm's `package-lock.json`, which always split.

### Yarn support

The bot reads `yarn.lock` across both of Yarn's incompatible formats: Classic (v1) and Berry (v2+) — see `docs/yarn-lockfile-schema.md`. Neither format records a production-vs-development distinction per package (that information lives only in `package.json`, which the bot does not read), so a Yarn section always reports a single undifferentiated Dependencies group, regardless of format.

## Development

```bash
make build      # build the binary into dist/
make run        # run without building
make test       # run tests
make coverage   # run tests with coverage (HTML + JUnit reports)
```
