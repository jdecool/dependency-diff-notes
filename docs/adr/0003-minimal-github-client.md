---
status: accepted
---

# Minimal internal GitHub client instead of an SDK

The bot now supports GitHub as a second Forge (see CONTEXT.md), needing the same three operations against GitHub's REST API as it already performs against GitLab's: list, create, and update a Bot Comment on a Change Request (a GitHub pull request's comments, modeled by GitHub as issue comments).

This is the same situation as GitLab's client: a tiny, fixed surface, against a well-documented REST API, with no need for the broader functionality an official SDK (e.g. `google/go-github`) would bring in.
See `0001-minimal-gitlab-client.md` for the full rationale (avoiding an external dependency per this project's "no dependency without an ADR, prefer the standard library" rule) — it applies unchanged here.

We write `internal/github` the same way as `internal/gitlab`: a small client on top of `net/http`, implementing only `GET/POST /repos/{owner}/{repo}/issues/{number}/comments` and `PATCH /repos/{owner}/{repo}/issues/comments/{id}`, authenticating via the `Authorization: Bearer` header.
GitHub Enterprise Server (a different API host per instance) is out of scope for now — the base URL is hardcoded to `https://api.github.com`.
