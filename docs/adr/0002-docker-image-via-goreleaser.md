---
status: accepted
---

# Publish a GHCR Docker image via GoReleaser, for use as a GitLab CI job image

Consumers currently install the bot via `go install` or a prebuilt binary from GitHub Releases.
To let a consumer's GitLab CI use a Docker executor job that pulls the bot directly as `image:` (no `go install` step per pipeline run), we publish a container image to `ghcr.io/jdecool/dependency-diff-notes`, built and pushed by GoReleaser's `dockers_v2` config as part of the same `git tag` release that already produces the GitHub Release binaries — binaries and image always share one version and one release event.

The runtime image is `alpine:3` with `git` and `ca-certificates` installed (the bot shells out to `git merge-base`/`git show` in `internal/gitref`, and calls the GitLab API over HTTPS), multi-arch (`linux/amd64` + `linux/arm64`), runs as root, and leaves the entrypoint unset (alpine's default `/bin/sh`) so GitLab's Docker executor can `exec` into it to run `script:` steps with no extra `entrypoint:` override needed in the consumer's `.gitlab-ci.yml`.

## Considered Options

- **`scratch`/`distroless/static` base** — rejected: no shell (breaks GitLab's `script:` execution) and no `git` binary (the bot needs it).
- **`ENTRYPOINT` set to the binary** — rejected: the container would run the bot and exit immediately instead of staying alive for GitLab to `exec` into, forcing every consumer to add `entrypoint: [""]` just to get `script:` working.
- **Non-root user** — rejected for now: risks permission errors against GitLab's mounted checkout, whose ownership depends on the runner host; root matches the current `golang:1`-based example job, which works everywhere.
- **Standalone Dockerfile + separate CI workflow** — rejected: would duplicate the cross-compilation GoReleaser already does and introduce a second version/tagging scheme to keep in sync with the GitHub Releases one.
- **Docker Hub / consumer's own GitLab registry** — rejected: Docker Hub needs a new account/secret for no benefit over GHCR; the consumer's own registry is the wrong model since one image serves many consumer projects.
