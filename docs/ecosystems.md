# Ecosystem notes

The bot reads dependency state per Ecosystem from that Ecosystem's Lockfile: Composer (PHP) from `composer.lock`, npm (JavaScript) from `package-lock.json`, pnpm (JavaScript) from `pnpm-lock.yaml`, and Yarn (JavaScript) from `yarn.lock`.
Most of that reading is uniform, but pnpm and Yarn have format-specific behavior worth calling out.

## pnpm lockfileVersion support

The bot reads `pnpm-lock.yaml` across pnpm's three lockfile formats: `5.x`, `6.0`, and `9.0` (there is no `7.0`/`8.0` lockfileVersion, despite pnpm CLI majors 7 and 8 existing).
lockfileVersion 9.0 (pnpm ≥9, the current major version) dropped the per-package `dev` flag entirely — see [pnpm lockfile schema](pnpm-lockfile-schema.md) — so a pnpm section only gets the Production/Development split on `5.x`/`6.0` Lockfiles; a `9.0` Lockfile reports a single undifferentiated Dependencies group instead, unlike Composer's `composer.lock` and npm's `package-lock.json`, which always split.

## Yarn support

The bot reads `yarn.lock` across both of Yarn's incompatible formats: Classic (v1) and Berry (v2+) — see [yarn lockfile schema](yarn-lockfile-schema.md).
Neither format records a production-vs-development distinction per package (that information lives only in `package.json`, which the bot does not read), so a Yarn section always reports a single undifferentiated Dependencies group, regardless of format.
