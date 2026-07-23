# Local comparison

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

Output is plain text rather than Markdown - neither tables nor collapsible sections mean anything on a terminal - but it reports the same information, with one marker per change:

```
Dependency changes

Composer
  Production dependencies
    + acme/new  1.2.0
    ↑ symfony/console  v6.4.2 -> v6.4.3
    ↓ acme/legacy  2.1.0 -> 2.0.0
    ~ acme/dev-lib  dev-main (abc1234) -> dev-main (def5678)
    - acme/gone  0.9.0
```

`+` added, `-` removed, `↑` upgraded, `↓` downgraded, and `~` for an update whose direction cannot be determined - the same rule the comment's `🔄 Changed` follows.

The comparison uses the same merge-base semantics as CI, so the output is a faithful preview of the comment the bot would post on a Change Request.
Two things differ from CI: `--target-branch` is used **literally** (a local branch, tag, or SHA - it is not resolved through `origin/`, so no `git fetch` is required), and the `--source` (the new side) defaults to the on-disk working tree, including uncommitted changes.
The command always exits `0` on success whether or not changes were found (see [`docs/adr/0006-local-comparison-mode.md`](adr/0006-local-comparison-mode.md)).
