# Report options

Two settings control how the report is presented and where it is published.
Both are presentation choices: neither ever changes which dependency changes the report contains.

## How much of the report starts collapsed

`--report-fold` names the outermost level of the report that starts collapsed.
The three values are positions on a single axis, from the innermost fold outwards:

| `--report-fold` | Ecosystem section | Production / Dependencies | Development dependencies |
|---|---|---|---|
| `development` (default) | expanded | expanded | collapsed |
| `ecosystem`     | collapsed | expanded | expanded |
| `none`          | expanded | expanded | expanded |

```bash
dependency-diff-notes --report-fold ecosystem
```

`ecosystem` is the one to reach for on a monorepo where several Ecosystems change at once and the report crowds out everything else on the page: each section shrinks to its heading and a change count, and one click restores it.
Note that it leaves the groups *inside* a section expanded - since the section around them is shut, their own state is invisible until a reader opens it, and at that point they have asked to see the section, so a second layer of collapsed headers would only charge them another click.

`none` expands everything, including development dependencies.

This is a presentation setting only: it never changes which dependency changes the report contains, it is the same at both report destinations, and it has no effect on a [local comparison](local-comparison.md), whose terminal output has nothing to click and always prints in full.

## Reporting in the Change Request description

By default the bot publishes its report as a comment on the Change Request.
With `--report-destination=description` it publishes into the Change Request's own description instead (a GitLab Merge Request description, a GitHub Pull Request body):

```bash
dependency-diff-notes --report-destination description
```

The two destinations are exclusive: the report is never published to both at once.
Switching the option is safe at any point in a review - on its next run the bot removes the report from the destination no longer in effect, so a Change Request never ends up carrying a stale report next to a live one.

In the description, the bot owns only a delimited region:

```markdown
Whatever the author wrote stays exactly as it is.

<!-- dependency-diff-notes -->
## Dependency changes
...
<!-- /dependency-diff-notes -->
```

- Everything outside the two markers is left untouched, byte for byte.
- The region is appended at the end of the description the first time; after that it is updated wherever it stands, so moving it up in the description is respected rather than undone on the next run.
- Deleting the closing marker by hand fails the run with an explicit error. The bot will not assume its region runs to the end of the document, because that assumption would delete whatever was written below it.
- Nothing is written when the rendered report is identical to what is already published, so a pipeline running on every push does not fill the merge request activity feed with "changed the description".

Writing the description needs more permission than posting a comment: on GitHub Actions the step needs `permissions: pull-requests: write`, and on GitLab the token must be allowed to modify the merge request, not only to post notes on it.

See [`docs/adr/0008-report-destination.md`](adr/0008-report-destination.md) for the reasoning behind these choices.
