---
status: accepted
---

# Operator-declared Ecosystem allowlist to resolve JavaScript Lockfile conflicts

When two JavaScript package managers' Lockfiles coexist at the same ref (e.g. both `package-lock.json` and `pnpm-lock.yaml` at HEAD), the bot cannot tell which one is actually in use and fails the run rather than guessing (see CONTEXT.md: Lockfile).
That refusal is correct, but until now there was no way for the operator to break the tie, so an affected project could not be reported at all.

We add an allowlist, the Considered Ecosystems (see CONTEXT.md), declared via `--ecosystems` / `DEPENDENCY_DIFF_NOTES_ECOSYSTEMS` (e.g. `composer,pnpm`): the bot only examines the listed Ecosystems, and the JavaScript conflict simply never arises when the allowlist keeps at most one JavaScript Ecosystem.
Composer, which never competes for the JavaScript role, is unaffected either way.

The decision that needs recording is the **semantics** of that allowlist, because there were two genuine options and the choice is not reversible once CI pipelines depend on it:

- **Permanent restriction (chosen)**: a declared allowlist reduces the set of Ecosystems considered for the *whole run*. An excluded Ecosystem is ignored at every ref, even where no other Lockfile competes with it.
- **Conflict tie-breaker (rejected)**: the value would only be consulted at a ref where two JavaScript Lockfiles actually collide, and ignored otherwise.

We chose the permanent restriction because it is the simpler and more predictable model: the allowlist reduces the set of Ecosystems, and the existing conflict check then falls out naturally on the reduced set with no special case.
It also matches operator intent for the motivating scenario — a project genuinely on pnpm that carries a stray `package-lock.json` wants npm ignored *always*, not only when the two files happen to coexist.

The trade-off is that the two models diverge on a JavaScript package-manager migration (base ref has `package-lock.json`, HEAD has `pnpm-lock.yaml`, which is not a conflict today): under the permanent model an operator who has pinned `--ecosystems=pnpm` loses the npm-removal side of the report, whereas a tie-breaker would have shown both.
We accept this: the allowlist is an explicit, opt-in statement of "these are the Ecosystems I care about," and a run without the allowlist still reports the migration in full.

Unrecognized tokens (including `yarn`, a defined Ecosystem with no parser yet) are rejected at config load rather than silently ignored, so a typo fails fast instead of quietly narrowing the report; this is also what makes the "Considered but absent Lockfile" case safe to treat as silent.
