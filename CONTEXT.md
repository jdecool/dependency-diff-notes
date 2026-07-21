# dependency-diff-notes

A bot that reports dependency changes on a Change Request, so authors and reviewers can see every package addition, removal, or version change at a glance.

**Supported ecosystems**: Composer (PHP), and npm, pnpm, and Yarn (all three JavaScript).

## Language

**Forge**:
A code-hosting platform that hosts a Change Request and accepts a Bot Comment.
GitLab and GitHub are the two Forges this bot supports.
_Avoid_: provider, platform, SCM

**Change Request**:
A proposal to merge one branch into another on a Forge — a GitLab Merge Request or a GitHub Pull Request.
The bot computes Dependency Changes between the merge-base of the Change Request's target branch and its current commit.
_Avoid_: merge request, pull request, MR, PR (each is the Forge-specific term; use them only when a sentence is specifically about that one Forge)

**Ecosystem**:
A (language, package manager) pairing the bot can read dependency state from, identified by the Lockfile format it produces: Composer (PHP); npm, Yarn, and pnpm (all three JavaScript).
A Change Request may involve more than one Ecosystem at once (e.g. a Composer backend and an npm frontend in the same repository) — each active Ecosystem gets its own section of the same Dependency Report.
_Avoid_: language, package manager (each names one half of the pairing; use them only when a sentence is specifically about that one half)

**Lockfile**:
The file recording a project's exact resolved dependency state for one Ecosystem: `composer.lock` (Composer), `package-lock.json` (npm), `yarn.lock` (Yarn), `pnpm-lock.yaml` (pnpm).
An Ecosystem is active for a given ref when its Lockfile exists at that ref; detected independently at the merge-base and at the Change Request's current commit (see Dependency Change), not as a single combined check — a Change Request that migrates from one Ecosystem to another (e.g. Yarn → pnpm) is not a conflict, since the two Lockfiles never coexist at the same ref.
Two Lockfiles of different JavaScript package managers coexisting at the *same* ref (e.g. both `yarn.lock` and `package-lock.json` present at HEAD) is a genuine conflict the bot refuses to guess about, and fails the run instead of picking one — reachable between any two of npm, Yarn, and pnpm, since all three are implemented; Composer never participates in this conflict, since its Lockfile doesn't compete for the same role.
An operator can pre-empt this conflict by restricting the Considered Ecosystems (see below) to keep at most one JavaScript Ecosystem — the excluded one is then dropped for the whole run and never competes for the role.

**Considered Ecosystems**:
The set of Ecosystems the bot is permitted to examine for a run, as declared by the operator; all Ecosystems by default.
It is an allowlist independent of Lockfile presence: an Ecosystem produces a section of the Dependency Report only when it is both *Considered* (in the allowlist, or all by default) and *active* (its Lockfile present at the ref — see Lockfile).
The restriction is permanent for the run, not a tie-breaker consulted only on conflict: an excluded Ecosystem is ignored at every ref, even where no other Lockfile competes with it (see `docs/adr/0005-ecosystem-allowlist.md`).
A Considered Ecosystem whose Lockfile is absent is not an error — it simply contributes nothing, exactly as when no allowlist is set.
_Avoid_: enabled/selected ecosystems, ecosystem filter (use "Considered" as the canonical adjective)

**Source**:
The new side of a comparison - what is measured against the base (the merge-base of the target branch).
In a Change Request it is the Change Request's current commit.
In a Local Comparison it is, by default, the on-disk working tree (including uncommitted changes), or a chosen git ref instead.
It mirrors the source branch of a GitLab Merge Request (the branch being merged in).
_Avoid_: head, new version

**Local Comparison**:
Running the bot outside a Change Request to compute a Dependency Report between a base branch and the Source and print it to the terminal, instead of posting a Bot Comment on a Forge.
A developer uses it to preview the dependency changes a Change Request would report, before opening one.
The base branch is used literally (a local branch, tag, or SHA), not resolved through a remote as it is in a Change Request context.
_Avoid_: dry run

**Dependency Change**:
An addition, removal, or update of a package between the merge-base of the Change Request's target branch and the Change Request's current commit, computed from one Ecosystem's Lockfile.
_Avoid_: diff, delta

**Reference Change**:
A Dependency Change where a package's version label is unchanged (typical of Composer `dev-*` branch aliases, or a JavaScript git dependency pinned to a branch) but the resolved commit/identifier behind it differs (Composer's `source.reference`; a JavaScript git dependency's resolved commit).
Treated as an update even though the version label itself didn't change.
The concept applies uniformly across every Ecosystem, but is only actually implemented for Composer, npm, and Yarn so far — pnpm's git-dependency resolution format isn't handled yet, so a pnpm git dependency's Reference is always empty.
_Avoid_: commit change

**Dependency Report**:
The structured content the bot posts: one section per active Ecosystem, each broken down into Production / Development dependencies (or a single undifferentiated Dependencies group — see below), each listing its Dependency Changes sorted alphabetically by package name.
The sort is a single alphabetical ordering across every kind of change, not a grouping by kind: the rendered table states what happened to each package in a column of its own, so a reader looks a package up by name rather than by what happened to it.

**Change Direction**:
Which way a package's version moved in an update, reported as an upgrade or a downgrade.
It refines a Dependency Change of type Updated and exists only for presentation — it is not a fourth kind of change alongside addition, removal, and update.
A direction is reported only when both version labels can actually be ordered as Semantic Versioning, which excludes Composer `dev-*` aliases, `1.0.x-dev` suffixes, git dependencies, and pnpm's `workspace:*`; and it is equally absent from a Reference Change, where the version label is identical on both sides and only the resolved commit moved.
In all those cases the bot reports an undifferentiated change rather than guessing a direction (see `docs/adr/0007-semver-comparison-dependency.md`).
_Avoid_: bump, version delta

**Production dependencies** / **Development dependencies**:
Within an Ecosystem's section of a Dependency Report, the two groups most Ecosystems' Lockfiles distinguish (Composer's `packages`/`packages-dev`; npm's and pnpm's per-package `dev` flag).
Both include direct and transitive packages — the report is not filtered to direct requirements only.
Two cases report a single undifferentiated **Dependencies** group instead of this split:
Yarn's Lockfile (`yarn.lock`, both Classic and Berry) carries no such distinction at all — that information lives only in `package.json`, which the bot does not cross-reference.
pnpm's Lockfile (`pnpm-lock.yaml`) distinguishes them in lockfileVersion 5.x and 6.0 (a per-package `dev` flag, same as npm), but that flag was dropped entirely in lockfileVersion 9.0 — the distinction there lives only per-workspace-importer, which the bot does not walk — so which grouping a pnpm section uses depends on the lockfileVersion of the Lockfile that was actually read, not on the Ecosystem alone.

**Report Destination**:
Where the Dependency Report lands on a Change Request: the Bot Comment or the Description Region.
Exactly one is in effect for a run, as declared by the operator; the two are never used at once, since the same report published twice is content a reader has to reconcile with itself.
The bot maintains the report at the destination in effect and removes it from the other one, so a Change Request never carries two reports that can drift apart (see `docs/adr/0008-report-destination.md`).
_Avoid_: report target ("target" already names the branch a Change Request merges into), output mode

**Report Fold**:
The outermost level of a Dependency Report that starts collapsed when a reader first sees it, as declared by the operator.
Three positions on one axis, from innermost outwards: Development dependencies (the default), the Ecosystem section, or no fold at all.
Folding at the Ecosystem level shuts each section but leaves its groups open inside, so one click restores the whole section rather than revealing a second layer of shut headers.
It is presentation only: it never changes which Dependency Changes a report contains, and has no effect on a Local Comparison, whose terminal output has nothing to click and always prints in full.
It does not vary by Report Destination either - the same body is published wherever the report lands, which is what keeps a Local Comparison a faithful preview of it.
_Avoid_: collapse level, expansion mode

**Bot Comment**:
One of the two Report Destinations, and the default: the single comment the bot maintains on a Change Request, identified by a hidden marker and updated in place on every pipeline run instead of being duplicated.
Implemented as a plain GitLab Note (never a resolvable Discussion) or a GitHub issue comment, depending on the Forge.
The bot authored its entire body and is free to rewrite or delete all of it.
_Avoid_: Bot Note (superseded now that the bot supports more than one Forge), note, discussion (as a synonym for this concept)

**Description Region**:
The other Report Destination: a delimited region of a Change Request's description (a GitLab Merge Request description, a GitHub Pull Request body) holding the Dependency Report.
Unlike a Bot Comment it lives inside a document the bot does not own, since the author writes the rest of it, so the bot only ever reads, writes, or removes what lies strictly between its opening and closing markers.
It is placed at the end of the description when first inserted, and from then on updated wherever the author has since moved it: the end is where it starts out, not a position reimposed on every run.
A description carrying an opening marker with no closing one has been hand-edited into a shape the bot cannot delimit, and fails the run rather than being guessed at, since every guess about where the region ends risks deleting text a human wrote.
_Avoid_: description block, managed region (neither says both that it is delimited and where it lives)
