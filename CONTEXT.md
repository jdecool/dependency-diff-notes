# dependency-diff-notes

A bot that reports dependency changes on a Change Request, so authors and reviewers can see every package addition, removal, or version change at a glance.

**Supported ecosystems**: currently Composer (PHP) and npm (JavaScript).
Yarn and pnpm (also JavaScript) are designed for (see the Ecosystem and Lockfile entries below) but not yet implemented.

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
A (language, package manager) pairing the bot can read dependency state from, identified by the Lockfile format it produces: Composer (PHP); npm, Yarn, and pnpm (all three JavaScript — Yarn and pnpm not yet implemented, see "Supported ecosystems" above).
A Change Request may involve more than one Ecosystem at once (e.g. a Composer backend and an npm frontend in the same repository) — each active Ecosystem gets its own section of the same Dependency Report.
_Avoid_: language, package manager (each names one half of the pairing; use them only when a sentence is specifically about that one half)

**Lockfile**:
The file recording a project's exact resolved dependency state for one Ecosystem: `composer.lock` (Composer), `package-lock.json` (npm), `yarn.lock` (Yarn), `pnpm-lock.yaml` (pnpm).
An Ecosystem is active for a given ref when its Lockfile exists at that ref; detected independently at the merge-base and at the Change Request's current commit (see Dependency Change), not as a single combined check — a Change Request that migrates from one Ecosystem to another (e.g. Yarn → pnpm) is not a conflict, since the two Lockfiles never coexist at the same ref.
Two Lockfiles of different JavaScript package managers coexisting at the *same* ref (e.g. both `yarn.lock` and `package-lock.json` present at HEAD) is a genuine conflict the bot refuses to guess about, and fails the run instead of picking one.
This conflict/migration handling applies once more than one JavaScript Ecosystem is implemented; with only Composer and npm implemented so far, no two Ecosystems can conflict (Composer's Lockfile and npm's never compete for the same role).

**Dependency Change**:
An addition, removal, or update of a package between the merge-base of the Change Request's target branch and the Change Request's current commit, computed from one Ecosystem's Lockfile.
_Avoid_: diff, delta

**Reference Change**:
A Dependency Change where a package's version label is unchanged (typical of Composer `dev-*` branch aliases, or a JavaScript git dependency pinned to a branch) but the resolved commit/identifier behind it differs (Composer's `source.reference`; a JavaScript git dependency's resolved commit).
Treated as an update even though the version label itself didn't change.
Applies uniformly across every Ecosystem.
_Avoid_: commit change

**Dependency Report**:
The structured content the bot posts: one section per active Ecosystem, each broken down into Production / Development dependencies (or a single undifferentiated Dependencies group — see below), each in turn grouped into Added / Updated / Removed, sorted alphabetically within each group.

**Production dependencies** / **Development dependencies**:
Within an Ecosystem's section of a Dependency Report, the two groups most Ecosystems' Lockfiles distinguish (Composer's `packages`/`packages-dev`; npm's and pnpm's per-package `dev` flag).
Both include direct and transitive packages — the report is not filtered to direct requirements only.
Yarn's Lockfile (`yarn.lock`, both Classic and Berry) carries no such distinction at all — that information lives only in `package.json`, which the bot does not cross-reference — so a Yarn section reports a single undifferentiated **Dependencies** group instead of this Production/Development split.

**Bot Comment**:
The single comment the bot maintains on a Change Request, identified by a hidden marker and updated in place on every pipeline run instead of being duplicated.
Implemented as a plain GitLab Note (never a resolvable Discussion) or a GitHub issue comment, depending on the Forge.
_Avoid_: Bot Note (superseded now that the bot supports more than one Forge), note, discussion (as a synonym for this concept)
