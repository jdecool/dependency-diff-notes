---
status: accepted
---

# Report Destination: publishing the Dependency Report in the Change Request description

Until now the bot had exactly one way to publish: the Bot Comment, a comment of its own on the Change Request.
Some teams would rather read the dependency changes in the Change Request description, where the reader lands first and where the information stays attached to the proposal itself rather than to a position in a discussion that grows below it.

We add a **Report Destination** (see CONTEXT.md), selected with `--report-destination=comment|description` (env `DEPENDENCY_DIFF_NOTES_REPORT_DESTINATION`), defaulting to `comment` so existing pipelines are unaffected.

The decisions below are recorded because writing into a description is a different problem from writing a comment, and because the marker format ends up in real Change Request descriptions outside this project's control, where changing it later would strand regions the bot can no longer find.

## The destinations are exclusive, not cumulative

One destination is in effect per run; the report is never published to both.

The content is identical either way, so publishing it twice adds no information and costs a reader the work of checking that the two copies agree.
It would also double what has to stay synchronized on every pipeline run, and therefore what has to be tested.

The option is an enum rather than a boolean (`--in-description`) so that a third destination, should one ever appear, is a new value rather than a second boolean with meaningless combinations.
It is named "destination" rather than "target" because `--target-branch` already uses "target" for the branch a Change Request merges into.

## The bot owns a delimited region, never the description

A Bot Comment is entirely the bot's; a description is the author's document with a region lent to the bot.
That asymmetry drives everything else.

The region is delimited by a **pair** of markers: the existing `<!-- dependency-diff-notes -->` as the opening one, and a new closing `<!-- /dependency-diff-notes -->`.
The bot reads the description, replaces what lies between them, and appends the region at the end when it is absent.
Everything outside the markers is untouched.

We rejected the simpler "everything after the marker belongs to the bot", which would have been adequate as long as the region really is the last thing in the document.
That assumption breaks the moment a human writes a line below the region - a note added later, a template footer, another integration - and the next run would delete that text silently.
Silent data loss on human-authored content is not an acceptable failure mode for a convenience feature.

For the same reason, an opening marker found with no closing one fails the run with an explicit error instead of assuming the region runs to the end of the document.
Assuming would resurrect exactly the behaviour rejected above, and would do so precisely in the case where we have evidence the document was hand-edited.
A loud failure is fixed in seconds; a silent deletion is not noticed at all.

Reusing the current `Marker` value as the opening delimiter is what makes this change need no migration: `report.HasMarker` keeps recognizing Bot Comments already published on open Change Requests, so none of them becomes an orphan that the bot duplicates on its next run.

## The report body is identical for both destinations

`report.Render` produces one Markdown body, used verbatim as the Bot Comment and as the content of the Description Region.

A second rendering would mean two things to maintain for content that is the same, rendered by the same Markdown engine.
It would also weaken a property the project already relies on: a Local Comparison is documented as a faithful preview of what the bot would publish, which only holds while there is one body to preview.

The accepted trade-off is that a long report is more intrusive in a description - the most visible part of the Change Request page, and the author's own text - than in a discussion, where length is expected.
Two things soften it: the region sits at the end, so it pushes nothing down, and development dependencies (the larger half of a typical report) are already collapsed by the existing rendering.
If this proves too intrusive, the cheap fix is to wrap the region in one more `<details>`, not to fork the renderer.

Nothing decorative is written around the region either - no horizontal rule to set it off.
A rule outside the markers would be content the bot writes but does not own, and would survive as an orphan when the region is removed; a rule inside them would also appear at the top of the Bot Comment, where it separates nothing.
The region already opens on an `## Dependency changes` heading, which both Forges render with enough separation.

## The other destination is cleaned up on every run

In `description` mode the bot deletes an existing Bot Comment; in `comment` mode it strips its region from the description.

Without this, switching the option mid-review leaves a report that will never be updated again, frozen next to an up-to-date one that says something else, with nothing telling the reader which is authoritative.
A stale report is worse than no report: the whole value of the bot is being trustworthy at a glance.

We preferred deletion to rewriting the stale artifact into a "this moved" pointer, which would have been a third state to model, render and test for what is a rare operator config change.
Cleanup only ever touches what the bot wrote itself: its own marked comment, or its own delimited region.

The known cost is that on GitLab, deleting the Bot Comment takes any human replies threaded under it with it.
This is unlikely - the bot posts a plain Note, never a resolvable Discussion - but it is real, and it is the reason this decision is written down.

It also means each mode makes one API call it would not otherwise need (listing comments in `description` mode, reading the description in `comment` mode).
That is marginal next to the cost of a contradictory Change Request.

## The bot writes only when the content actually differs

Rendered content identical to what is already published produces no API write, for either destination.

GitLab records every description edit in the Merge Request activity feed and keeps a version history.
Writing unconditionally on every pipeline run would add a "changed the description" entry per run even when no dependency moved, making the bot the main source of noise on a page it exists to clarify.

The rule is applied to the Bot Comment too, rather than as a description-only exception: the comparison is free (both bodies are already in memory when the decision is made), both Forges mark a comment as edited on every update, and one rule stated once is easier to hold in the head than the same rule plus a carve-out.

A secondary benefit on the description is fewer chances to lose an author's concurrent edit.
Neither Forge offers a conditional write on this field, so the read-modify-write is not atomic and an author editing the description while a pipeline runs can be overwritten.
This cannot be eliminated, only made rare, by not writing when there is nothing to say.

## What is unchanged

The Report Destination has no effect on a Local Comparison, which prints to the terminal and publishes nothing.
The option is ignored there rather than rejected, consistent with `Source` being ignored in a Change Request context: the destination is typically set once in a pipeline's environment, and a developer running a local preview should not have to unset an option they never chose.
An unparseable value remains a hard error in every mode - an irrelevant option is tolerated, an incomprehensible one is not.

Writing the description reaches for a permission posting a comment did not need: on GitLab the token must be allowed to modify the Merge Request itself, not only to post notes on it.
On GitHub Actions the workflow permission is unchanged in practice, since `pull-requests: write` (already what the README documents) covers both the pull request body and its comments.
