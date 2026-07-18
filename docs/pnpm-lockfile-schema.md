# pnpm-lock.yaml wire format reference

Research notes for the pnpm lockfile parser (see ADR 0004).
Primary source: [pnpm/spec](https://github.com/pnpm/spec) (`lockfile/*.md`), cross-checked against real `pnpm-lock.yaml` files.
The spec repo only documents versions `3.x`, `4`, `5.x`, `6.0`, `9.0` - there is no `7.0` or `8.0` lockfile version, per the [version table](https://github.com/pnpm/spec/blob/master/lockfile/README.md) (`5.4` = pnpm `>=7 <8`, `6.0` = pnpm `>=8 <9`, `9.0` = pnpm `>=9`).

## Legacy era: lockfileVersion 5.x

Source: [`lockfile/5.md`](https://github.com/pnpm/spec/blob/master/lockfile/5.md), [`5.2.md`](https://github.com/pnpm/spec/blob/master/lockfile/5.2.md) (identical `dev` wording), [`dependency-path.md`](https://github.com/pnpm/spec/blob/master/dependency-path.md).
Real example: [unjs/defu @ `796275a`](https://github.com/unjs/defu/blob/796275a8e08839ee0bba9ed93c188b217f49b3dc/pnpm-lock.yaml), non-workspace, `lockfileVersion: 5.4`.

- Non-workspace ("dedicated shrinkwrap") shape: `specifiers`, `dependencies`, `devDependencies` sit directly at file root.
  `dependencies`/`devDependencies` map name -> a **bare exact-version string** (`eslint: 8.29.0`), not an object.
- `packages` key: leading slash, slash-separated: `/name/version`; scoped: `/@scope/name/version` (slash, not `@`).
  Peer suffix (if any) is `_`-joined: `/foo/1.0.0_bar@2.0.0`.
- `packages[key].resolution.integrity` holds the hash.
  `.name`/`.version` are only set "when not in the dependencyPath" - for ordinary registry packages both are absent and must be parsed from the key.
- `packages[key].dev` is a tri-state: `true` = dev-only; `false` = reachable as both prod and dev (mixed); **absent** = pure prod-only.
  Confirmed real: defu's dev-only entries carry `dev: true`, prod-only entries omit the key; `dev: false` not observed in that sample.

## Current era: lockfileVersion 6.0 and 9.0

Source: [`lockfile/6.0.md`](https://github.com/pnpm/spec/blob/master/lockfile/6.0.md), [`9.0.md`](https://github.com/pnpm/spec/blob/master/lockfile/9.0.md).

### Where the dev/prod signal lives moved between 6.0 and 9.0 - the key finding

**6.0 still carries a package-level `dev` flag**, worded identically to 5.x.
Confirmed real: [pnpm/pnpm @ `v8.15.0`](https://github.com/pnpm/pnpm/blob/v8.15.0/pnpm-lock.yaml) and [unjs/defu @ `bbc3c70`](https://github.com/unjs/defu/blob/bbc3c7086bc23514fed56f6bd29b40522da5dafc/pnpm-lock.yaml) both show `dev: true` in `packages`.
Since this flag is still global (not per-importer), a package that is prod for one workspace importer and dev for another cannot be expressed correctly in 6.0 - same limitation as 5.x.

**9.0 drops `dev` entirely.**
[`9.0.md`](https://github.com/pnpm/spec/blob/master/lockfile/9.0.md) lists the full `packages[dependencyId]` field set - `peerDependencies`, `peerDependenciesMeta`, `engines`, `os`, `cpu`, `libc`, `deprecated`, `bundledDependencies`, `resolution`, `hasBin` - no `dev`.
Confirmed empirically: zero `dev:` keys in [pnpm/pnpm's own v9 lockfile](https://github.com/pnpm/pnpm/blob/main/pnpm-lock.yaml), [sveltejs/kit](https://github.com/sveltejs/kit/blob/main/pnpm-lock.yaml), or [defu @ `23cc432`](https://github.com/unjs/defu/blob/23cc432b40509c952c39c4eba0b7def3f57fdb41/pnpm-lock.yaml).
**Dev/prod status now lives exclusively per-importer**: which of `importers[path].dependencies`/`.devDependencies`/`.optionalDependencies` an entry is listed under.
There is no global fallback even for a single-importer project - the parser must read the importer's map, always.

### Does a non-workspace project get wrapped in `importers`?

- **6.0, no `pnpm-workspace.yaml`**: flat `dependencies`/`devDependencies` at file root, **no `importers` wrapper**.
  Confirmed: defu @ `bbc3c70` (no workspace file in that repo).
- **9.0, no `pnpm-workspace.yaml`**: `importers: { ".": {...} } }` **is present regardless**.
  Confirmed: same defu repo, still no workspace file, wraps under `importers["."]` once on `9.0` (commit `23cc432b`); 9.0.md documents only the `importers` shape.

**Parser implication**: read flat root keys for `5.x`/`6.0`, always read `importers["."]` for `9.0`.

### Other shape facts (6.0 and 9.0)

- `importers[path][type][name]` value is `{specifier, version}`, not a bare string.
- `packages` key separator changed `/` -> `@`; 6.0 keeps the leading slash (`/@scope/name@version`), **9.0 drops it** (`@scope/name@version`, quoted because a plain YAML scalar can't start with `@`; unscoped keys like `zod@3.22.3` need no quoting).
- Peer suffix in both: parenthesized, e.g. `ts-node@10.9.1(@types/node@14.18.36)`, replacing 5.x's `_`-joined form ([PR #5810](https://github.com/pnpm/pnpm/pull/5810)).
- `resolution: {integrity: sha512-...}` is unchanged between 6.0 and 9.0; 6.0's changelog notes the `registry` sub-field (present in <=5.x for non-default registries) was removed.
- **9.0 adds a top-level `snapshots` map**; 6.0 has none.
  6.0's `packages[key]` carries its own transitive `dependencies` inline.
  9.0's `packages[dependencyId]` holds only resolution/engine/os/cpu/libc metadata; the transitive graph moved to `snapshots[dependencyPath]`.

## Minimal example: legacy era (lockfileVersion 5.4)

Integrity hashes are illustrative placeholders in pnpm's real format, not fetched from the registry.

```yaml
lockfileVersion: 5.4

specifiers:
  eslint: ^8.29.0
  lodash: ^4.17.21

dependencies:
  lodash: 4.17.21

devDependencies:
  eslint: 8.29.0

packages:

  /lodash/4.17.21:
    resolution: {integrity: sha512-v2kDEe57lecTulaDIuNTPy3Ry4/GHY/23nY/qm9V4wcVYcJ4C6+PkNJqCXHzXfCaHPRVSw9NUwj4Rf/5FLpb0g==}
    dev: false

  /eslint/8.29.0:
    resolution: {integrity: sha512-isRXgqSy4sZ0CxHESuq6NB+7NkP+CVFf/e3jr5EhOhAoOD2E9nIesNjjaFupt3wldbIkbe8B/K5UDb0nu6/xnQ==}
    engines: {node: ^12.22.0 || ^14.17.0 || >=16.0.0}
    hasBin: true
    dev: true
```

## Minimal example: current era (lockfileVersion 9.0)

Same two dependencies, non-workspace (no `pnpm-workspace.yaml`) - `importers["."]` is still present, per the finding above.
`snapshots` entries are empty since both packages are leaves here.

```yaml
lockfileVersion: '9.0'

settings:
  autoInstallPeers: true
  excludeLinksFromLockfile: false

importers:

  .:
    dependencies:
      lodash:
        specifier: ^4.17.21
        version: 4.17.21
    devDependencies:
      eslint:
        specifier: ^8.29.0
        version: 8.29.0

packages:

  lodash@4.17.21:
    resolution: {integrity: sha512-v2kDEe57lecTulaDIuNTPy3Ry4/GHY/23nY/qm9V4wcVYcJ4C6+PkNJqCXHzXfCaHPRVSw9NUwj4Rf/5FLpb0g==}

  eslint@8.29.0:
    resolution: {integrity: sha512-isRXgqSy4sZ0CxHESuq6NB+7NkP+CVFf/e3jr5EhOhAoOD2E9nIesNjjaFupt3wldbIkbe8B/K5UDb0nu6/xnQ==}
    engines: {node: ^12.22.0 || ^14.17.0 || >=16.0.0}
    hasBin: true

snapshots:

  lodash@4.17.21: {}

  eslint@8.29.0: {}
```

## Open ambiguities

- Whether `6.0` ever wraps a *literally single-importer* workspace (one entry, key `.`, but a real `pnpm-workspace.yaml` present) the same way 9.0 always does - only the two extremes (no workspace file, and a large multi-importer workspace) were checked against real lockfiles here.
- Whether `packages[key].dev: false` (vs. absent) actually appears in 5.x/6.0 for the "mixed" case the spec describes - the real sample only exhibited `true` and absent, not `false`.
