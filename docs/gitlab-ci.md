# GitLab CI

Run as a job in a consumer project's merge request pipeline, the bot compares each active Ecosystem's Lockfile (`composer.lock`, `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`) between the merge-base of the target branch and the current commit, and creates or updates a single note on the merge request reporting the dependency changes found.

See [`examples/gitlab-ci.yml`](../examples/gitlab-ci.yml) for a copy-pasteable job.

Outside of a merge request pipeline (`--request-iid`/`CI_MERGE_REQUEST_IID` empty), where `CI_MERGE_REQUEST_TARGET_BRANCH_NAME` is also empty, the bot prints a message and exits `0` without doing anything - it's safe to leave the job unrestricted by `rules` if you'd rather not gate it on `$CI_PIPELINE_SOURCE`.
(Passing a `--target-branch` explicitly in that situation runs a [local comparison](local-comparison.md) instead.)

## Configuring the token

The bot needs a GitLab API token, `DEPENDENCY_DIFF_NOTES_TOKEN`, to read and write merge request notes.

### Why a token is required

When the bot runs in a `merge_request_event` pipeline, GitLab sets `CI_MERGE_REQUEST_IID` and the bot detects that it is running in a merge request context.
In that context it must authenticate to GitLab to create or update its note, so an API token becomes required.

If the token is missing, the job fails at startup with:

```
load config: resolve config: missing required change request settings: token (or DEPENDENCY_DIFF_NOTES_TOKEN)
```

GitLab provides most of the bot's other settings automatically through predefined CI/CD variables (`CI_SERVER_URL`, `CI_PROJECT_ID`, `CI_MERGE_REQUEST_IID`, `CI_MERGE_REQUEST_TARGET_BRANCH_NAME`).
The token is the one setting GitLab does not provide for you, so you must configure it yourself.

### Why `CI_JOB_TOKEN` cannot be used

GitLab injects a built-in `CI_JOB_TOKEN` into every job, but it cannot be used here.
Its API surface deliberately excludes the Notes API, so it cannot create or update merge request comments.
Falling back to it would only turn the clear "missing token" startup error into a more confusing authentication failure at runtime.

An explicit token with the `api` scope is therefore required.

### Choosing a token type

Any of the following works, as long as it has the `api` scope and a role of `Developer` or higher (writing merge request notes requires at least `Developer`).

- **Project Access Token** (recommended): scoped to the single project, not tied to a personal account.
  This is the simplest and most contained option.
- **Group Access Token**: use this when you want to reuse one token across many projects in the same group.
- **Personal Access Token**: a fallback for GitLab tiers or instances where project and group access tokens are not available.
  It is tied to your user account and acts on your behalf.

### Step 1: Create the token

These steps describe a Project Access Token.
Group and personal tokens follow the same idea from their own settings pages.

1. Open your project in GitLab.
2. Go to **Settings > Access tokens**.
3. Click **Add new token**.
4. Give it a name, for example `dependency-diff-notes`.
5. Set an expiration date according to your security policy.
6. Select the role **Developer** (or higher).
7. Select the scope **`api`**.
8. Click **Create project access token**.
9. Copy the generated token value now.
   GitLab shows it only once.

### Step 2: Add the CI/CD variable

1. In the same project, go to **Settings > CI/CD**.
2. Expand the **Variables** section and click **Add variable**.
3. Set **Key** to `DEPENDENCY_DIFF_NOTES_TOKEN`.
4. Set **Value** to the token you copied in Step 1.
5. Set **Type** to `Variable`.
6. Enable **Masked** so the token is hidden in job logs.
7. Leave **Protected** disabled, unless all your merge request source branches are protected (see the gotcha below).
8. Save the variable.

## Common gotcha: protected variables on unprotected branches

A **Protected** CI/CD variable is only exposed to pipelines running on protected branches or tags.
Most merge requests come from unprotected feature branches, so a protected `DEPENDENCY_DIFF_NOTES_TOKEN` resolves to an empty value in those pipelines.
That reproduces the exact "missing required change request settings: token" error described above, even though the variable exists.

Recommendation:

- Leave **Protected** disabled and keep **Masked** enabled.
  This is enough for most setups.
- If your security policy requires the token to stay protected, protect the merge request source branches as well, so the token is exposed to their pipelines.

## Verify

1. Re-run the merge request pipeline, or push a commit that changes `composer.lock`.
2. On success, the `dependency-diff-notes` job passes and a single note reporting the dependency changes appears on the merge request.
3. Re-running the pipeline updates that same note instead of adding a new one.

For the full job definition, see [`examples/gitlab-ci.yml`](../examples/gitlab-ci.yml).
For the complete configuration surface (all flags and environment variables), see [Configuration](configuration.md).
