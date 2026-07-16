---
status: accepted
---

# Minimal internal GitLab client instead of an SDK

The bot only needs three GitLab API calls: list notes on a merge request, create a note, and update a note.
Pulling in an official SDK (e.g. `xanzy/go-gitlab`) would cover far more surface than needed and would require an ADR exception to the project's "no external dependency without an ADR, prefer the standard library" rule (see `CLAUDE.md`).

We instead write a small internal client (`internal/gitlab`) on top of `net/http` that implements only the endpoints the bot uses.
The maintenance cost is low given the tiny surface, and it keeps the project dependency-free as already established for the rest of the codebase.
