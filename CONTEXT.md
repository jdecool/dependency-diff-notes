# dependency-diff-notes

A bot that reports dependency changes on a Change Request, so authors and reviewers can see every package addition, removal, or version change at a glance.

**Supported ecosystem**: currently Composer (PHP), read from `composer.lock`.
The name is deliberately generic so other ecosystems can follow later; today the implementation is Composer-only.

## Language

**Forge**:
A code-hosting platform that hosts a Change Request and accepts a Bot Comment.
GitLab and GitHub are the two Forges this bot supports.
_Avoid_: provider, platform, SCM

**Change Request**:
A proposal to merge one branch into another on a Forge — a GitLab Merge Request or a GitHub Pull Request.
The bot computes Dependency Changes between the merge-base of the Change Request's target branch and its current commit.
_Avoid_: merge request, pull request, MR, PR (each is the Forge-specific term; use them only when a sentence is specifically about that one Forge)

**Dependency Change**:
An addition, removal, or update of a package between the merge-base of the Change Request's target branch and the Change Request's current commit, computed from the project's lockfile (currently `composer.lock`).
_Avoid_: diff, delta

**Reference Change**:
A Dependency Change where a package's `version` field is unchanged (typical of `dev-*` branch aliases) but its `source.reference` commit hash differs.
Treated as an update even though the version label itself didn't change.
_Avoid_: commit change

**Dependency Report**:
The structured content the bot posts: production and development dependencies, each grouped into Added / Updated / Removed, sorted alphabetically within each group.

**Production dependencies** / **Development dependencies**:
The two sections of a Dependency Report.
Currently sourced from `composer.lock`'s `packages` (production) and `packages-dev` (development) arrays.
Both include direct and transitive packages — the report is not filtered to direct requirements only.

**Bot Comment**:
The single comment the bot maintains on a Change Request, identified by a hidden marker and updated in place on every pipeline run instead of being duplicated.
Implemented as a plain GitLab Note (never a resolvable Discussion) or a GitHub issue comment, depending on the Forge.
_Avoid_: Bot Note (superseded now that the bot supports more than one Forge), note, discussion (as a synonym for this concept)
