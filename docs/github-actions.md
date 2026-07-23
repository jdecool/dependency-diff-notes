# GitHub Actions

Run as a step in a consumer project's pull request workflow, the bot compares each active Ecosystem's Lockfile (`composer.lock`, `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`) between the merge-base of the base branch and the current commit, and creates or updates a single comment on the pull request reporting the dependency changes found.

See [`examples/github-actions.yml`](../examples/github-actions.yml) for a copy-pasteable workflow.
Note:

- The pull request number is parsed from `GITHUB_REF` (`refs/pull/<number>/merge`), so no extra configuration is needed beyond triggering on `pull_request`.
- `actions/checkout` must be configured to fetch the base branch's history (e.g. `fetch-depth: 0`), the same way GitLab CI jobs need `GIT_DEPTH: 0` — the bot never runs `git fetch` itself.
- GitHub Enterprise Server is not supported; the bot always talks to `https://api.github.com`.
- The step needs `permissions: pull-requests: write` (or a token with equivalent scope) to create/update comments.

For the complete configuration surface (all flags and environment variables), see [Configuration](configuration.md).
