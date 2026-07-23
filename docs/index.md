# Documentation

Documentation home for the Dependency Diff Notes Bot.
For a quick overview and installation, start from the [README](../README.md).

## Setup & Usage

- [GitLab CI](gitlab-ci.md) — run the bot in a merge request pipeline, including token setup.
- [GitHub Actions](github-actions.md) — run the bot in a pull request workflow.
- [Local comparison](local-comparison.md) — preview dependency changes from your terminal, no Forge needed.
- [Understanding the report](report-anatomy.md) — what the posted comment looks like, and how the `Change` column is decided.

## Configuration

- [Configuration reference](configuration.md) — every flag and environment variable, plus restricting the Ecosystems considered.
- [Report options](report-options.md) — how much of the report starts collapsed (`--report-fold`) and where it is published (`--report-destination`).
- [Ecosystem notes](ecosystems.md) — pnpm lockfileVersion and Yarn format specifics.

## Reference & Internals

- [pnpm lockfile schema](pnpm-lockfile-schema.md) — wire-format research notes for the `pnpm-lock.yaml` parser.
- [yarn lockfile schema](yarn-lockfile-schema.md) — wire-format research notes for the `yarn.lock` parser.
- [Architecture decisions](adr/) — ADRs recording the project's significant design choices.
