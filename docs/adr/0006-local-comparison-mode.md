---
status: accepted
---

# Local Comparison mode for previewing a Dependency Report from the terminal

Until now the bot only ran inside a Change Request pipeline: it required a Forge, a token, and a Change Request IID, computed the Dependency Report between the merge-base of `origin/<target-branch>` and `HEAD`, and posted it as a Bot Comment.
A developer wanting to see what the bot would report before opening a Change Request had no way to run it.

We add a **Local Comparison** (see CONTEXT.md): running the bot outside a Change Request to print the same Dependency Report to the terminal instead of posting a Bot Comment.
Several design points are worth recording because they are behavioral choices a future reader will question, and CI pipelines and developer habits will start to depend on them.

## Auto-detection rather than a subcommand or a `--local` flag

The mode is selected by what is available, not by an explicit switch:

- In a Change Request context (a Change Request IID resolved) → CI mode, posts the Bot Comment. Unchanged.
- Otherwise, if `--target-branch` resolves to a non-empty value → Local Comparison, prints to stdout.
- Otherwise → nothing to do.

We chose auto-detection because the two modes are already distinguished by their environment: CI runners provide the Change Request context, a laptop does not.
A subcommand or a `--local` flag would be redundant with that signal and would be one more thing to get wrong in CI.

The trade-off is that the trigger is implicit: a stray `CI_MERGE_REQUEST_TARGET_BRANCH_NAME` (or `--target-branch`) in an environment with no Change Request context now runs a Local Comparison instead of printing "nothing to do".
We accept this; the previous behavior in that case (silently doing nothing) was not valuable to preserve.

## The base branch is used literally, unlike CI

In a Change Request context the target branch is resolved as `origin/<branch>`, because CI checkouts have the remote-tracking ref but not necessarily a local branch.
In a Local Comparison the value is used **literally**: `main` means the local `main`, and a tag or SHA works too.

This divergence is deliberate: locally the developer means their working copy of the branch, which may legitimately differ from `origin/` (not yet pushed, or `origin` not fetched).
Forcing an `origin/` prefix would require a fresh fetch and would forbid comparing against a local-only branch, a tag, or a commit.

## The Source defaults to the working tree, not a commit

The new side of the comparison - the **Source** (see CONTEXT.md) - defaults to the on-disk working tree, read directly from disk including uncommitted changes, and can be overridden to any git ref with `--source` (e.g. `--source HEAD`).

Reading the working tree is a new code path: every other read goes through `git show <ref>:<path>`.
We chose the working tree as the default because the primary use is "what will my in-progress change report", which is exactly the uncommitted state; a developer who wants a committed comparison asks for it explicitly with `--source`.
The merge-base is still computed against `HEAD` when the Source is the working tree, since the working tree conceptually sits on top of `HEAD`.

## What is kept identical to CI

Merge-base semantics, the JavaScript Lockfile conflict check (see CONTEXT.md: Lockfile), the Considered Ecosystems allowlist, and the per-Ecosystem lock-path flags all behave exactly as in a Change Request context, so a Local Comparison is a faithful preview of what the bot would compute in CI.
Only the base-ref resolution (literal vs `origin/`), the Source (working tree vs `HEAD`), and the output (terminal text vs Bot Comment) differ.

The output is a separate plain-text rendering (no Markdown, no hidden marker), because a terminal is not a Forge comment; it always exits 0 on success regardless of whether changes were found, so it reads as a report rather than a gate.
