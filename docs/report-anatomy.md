# What the report looks like

Each Ecosystem gets its own heading, followed by a collapsible section holding one table per dependency group.
By default, Ecosystems and their production dependencies are expanded; development dependencies are collapsed, since they are usually the larger and less scrutinized half of a Change Request.
On a repository whose reports run long you can fold one level further out with `--report-fold` (see [Report options](report-options.md)).
The summary line at the top appears only when more than one Ecosystem changed, and links to each section.

````markdown
## Dependency changes

[Composer](#composer) 3 · [npm](#npm) 1

### Composer

<details open>
<summary><strong>3 changes</strong></summary>

<details open>
<summary>Production dependencies (2)</summary>

| Package | Change | Version |
|---|---|---|
| [symfony/console](https://github.com/symfony/console) | ⬆️ Upgraded | v6.4.2 → v6.4.3 |
| [acme/legacy](https://github.com/acme/legacy) | ⬇️ Downgraded | 2.1.0 → 2.0.0 |

</details>

<details>
<summary>Development dependencies (1)</summary>

| Package | Change | Version |
|---|---|---|
| [phpunit/phpunit](https://github.com/sebastianbergmann/phpunit) | ➕ Added | 10.5.9 |

</details>

</details>
````

The `Change` column reports the direction of an update whenever the two versions can be ordered as Semantic Versioning.
When they cannot — a Composer `dev-*` alias, a git dependency, pnpm's `workspace:*` — or when only the resolved commit moved while the version label stayed put, the change is reported as `🔄 Changed` rather than being guessed at as an upgrade.
Packages are sorted alphabetically by name, across every kind of change: the `Change` column already says what happened, so the ordering is there to help you find a package.
