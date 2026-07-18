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

**Dependency Change**:
An addition, removal, or update of a package between the merge-base of the Change Request's target branch and the Change Request's current commit, computed from one Ecosystem's Lockfile.
_Avoid_: diff, delta

**Reference Change**:
A Dependency Change where a package's version label is unchanged (typical of Composer `dev-*` branch aliases, or a JavaScript git dependency pinned to a branch) but the resolved commit/identifier behind it differs (Composer's `source.reference`; a JavaScript git dependency's resolved commit).
Treated as an update even though the version label itself didn't change.
The concept applies uniformly across every Ecosystem, but is only actually implemented for Composer, npm, and Yarn so far — pnpm's git-dependency resolution format isn't handled yet, so a pnpm git dependency's Reference is always empty.
_Avoid_: commit change

**Dependency Report**:
The structured content the bot posts: one section per active Ecosystem, each broken down into Production / Development dependencies (or a single undifferentiated Dependencies group — see below), each in turn grouped into Added / Updated / Removed, sorted alphabetically within each group.

**Production dependencies** / **Development dependencies**:
Within an Ecosystem's section of a Dependency Report, the two groups most Ecosystems' Lockfiles distinguish (Composer's `packages`/`packages-dev`; npm's and pnpm's per-package `dev` flag).
Both include direct and transitive packages — the report is not filtered to direct requirements only.
Two cases report a single undifferentiated **Dependencies** group instead of this split:
Yarn's Lockfile (`yarn.lock`, both Classic and Berry) carries no such distinction at all — that information lives only in `package.json`, which the bot does not cross-reference.
pnpm's Lockfile (`pnpm-lock.yaml`) distinguishes them in lockfileVersion 5.x and 6.0 (a per-package `dev` flag, same as npm), but that flag was dropped entirely in lockfileVersion 9.0 — the distinction there lives only per-workspace-importer, which the bot does not walk — so which grouping a pnpm section uses depends on the lockfileVersion of the Lockfile that was actually read, not on the Ecosystem alone.

**Bot Comment**:
The single comment the bot maintains on a Change Request, identified by a hidden marker and updated in place on every pipeline run instead of being duplicated.
Implemented as a plain GitLab Note (never a resolvable Discussion) or a GitHub issue comment, depending on the Forge.
_Avoid_: Bot Note (superseded now that the bot supports more than one Forge), note, discussion (as a synonym for this concept)
