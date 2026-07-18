---
status: accepted
---

# External YAML dependency for pnpm and Yarn Berry lockfiles

Extending the bot beyond Composer to the JavaScript Ecosystems (see CONTEXT.md) requires reading `pnpm-lock.yaml` and Yarn Berry's `yarn.lock` (lockfileVersion 2+), both genuine YAML.
Go's standard library has no YAML support at all, unlike JSON (`encoding/json`, already used for `composer.lock` and `package-lock.json`), so this is a real gap the project's "no external dependency without an ADR, prefer the standard library" rule (see `CLAUDE.md`) must be weighed against.

We add `gopkg.in/yaml.v3` rather than hand-rolling a parser for the YAML subset these two formats happen to use today.
Unlike the GitLab/GitHub REST clients (ADR 0001, 0003), where the bot fully controls the tiny surface it talks to, YAML is a general-purpose format the bot does not control the shape of — a hand-rolled parser would need to track pnpm's and Yarn's actual output rather than a fixed API contract, and subtly wrong YAML parsing (quoting, anchors, multi-line scalars) is a correctness risk not worth taking on for a format a mature, widely-used library already parses correctly.

This does not extend to Yarn Classic's `yarn.lock` (lockfileVersion 1): despite looking YAML-like, it is a distinct, non-standard format that `gopkg.in/yaml.v3` cannot parse, and needs its own hand-rolled reader.
