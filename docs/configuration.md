# Configuration

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
| `--report-destination` | Where to publish the report: `comment` or `description` | `DEPENDENCY_DIFF_NOTES_REPORT_DESTINATION` | `DEPENDENCY_DIFF_NOTES_REPORT_DESTINATION` | `comment` |
| `--report-fold`        | Outermost report level that starts collapsed: `development`, `ecosystem` or `none` | `DEPENDENCY_DIFF_NOTES_REPORT_FOLD` | `DEPENDENCY_DIFF_NOTES_REPORT_FOLD` | `development` |
| `--repo-dir`           | Path to the repository checkout   | none                                      | none                           | `.` |

`--server-url` (GitLab only), `--project-id`, `--target-branch`, and `--token` are only required once the bot detects it's running in a Change Request context (i.e. `--request-iid` resolves to a non-empty value, from the flag, `CI_MERGE_REQUEST_IID`, or `GITHUB_REF`).
For a [local comparison](local-comparison.md) (no Change Request context) only `--target-branch` is needed; `--source` is optional, and no Forge, token, or project ID applies.
`--composer-lock-path`, `--npm-lock-path`, `--pnpm-lock-path`, `--yarn-lock-path`, and `--repo-dir` always have safe defaults and can be left unset in most setups — each Ecosystem's Lockfile is read at its default path and simply contributes no changes if the file doesn't exist, so a project using only one Ecosystem doesn't need to configure anything about the others.

`--report-destination` and `--report-fold` control how the report is published and presented; see [Report options](report-options.md).

## Restricting the Ecosystems considered

`--ecosystems` is an allowlist restricting the Ecosystems the bot considers for the run (tokens `composer`, `npm`, `pnpm`, `yarn`, case-insensitive; unknown tokens fail fast).
Left unset, every Ecosystem is considered — the historical behavior.
Its main use is resolving a JavaScript Lockfile conflict: on a project that carries both `package-lock.json` and `pnpm-lock.yaml`, set `--ecosystems=composer,pnpm` (or just `pnpm`) to tell the bot which JavaScript package manager is real, and the stray Lockfile is ignored for the whole run.
The restriction is permanent, not consulted only on conflict, so an excluded Ecosystem is dropped at every ref (see [`docs/adr/0005-ecosystem-allowlist.md`](adr/0005-ecosystem-allowlist.md)).

A project using more than one Ecosystem at once (e.g. a Composer backend and an npm frontend in the same repository) gets a single Bot Comment with one section per Ecosystem.
But a project with more than one JavaScript package manager's Lockfile present at once (e.g. both `package-lock.json` and `pnpm-lock.yaml`) is treated as a conflict and fails the run, since a project uses at most one at a time — unless you tell the bot which one is real with `--ecosystems`.
