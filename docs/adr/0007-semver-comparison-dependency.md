---
status: accepted
---

# External semver dependency for upgrade/downgrade detection

Reporting a Dependency Change of type Updated as an upgrade or a downgrade (see CONTEXT.md) requires ordering two version labels, which the standard library cannot do.
`dependencydiff.Diff` classifies a package as Updated whenever its version *or* its resolved reference differs, without ever comparing the two versions, so the direction of the change is information the bot did not previously have.

We add `golang.org/x/mod` and wrap `golang.org/x/mod/semver` in `internal/semver`, rather than hand-rolling a comparator.

This follows the line already drawn by the earlier dependency decisions.
ADR 0001 and 0003 hand-rolled the GitLab and GitHub clients because the bot fully controls the tiny API surface it talks to, and a hand-rolled client can simply be correct for that surface.
ADR 0004 took `gopkg.in/yaml.v3` because YAML is a general-purpose format the bot does not control the shape of, where subtly wrong parsing is a correctness risk not worth taking on.
Semantic Versioning falls on the ADR 0004 side: it is an external specification, and its genuinely fiddly part — pre-release ordering, where numeric identifiers compare numerically, alphanumeric ones lexically, a shorter identifier list sorts first, and a pre-release always precedes its release — is exactly where hand-rolled implementations go wrong.

`golang.org/x/mod` is maintained by the Go team under the same release discipline as the toolchain, which makes it a weaker commitment than a typical third-party dependency.
It also already implements the tolerant rules the report needs, and it needs no configuration to do so: it accepts one, two, or three numeric components, treating the missing ones as zero, so `5.4` and `5.4.0` compare equal; it ignores build metadata per the specification; and it orders pre-releases correctly.

`internal/semver` exists as an adaptation layer rather than the bot calling `x/mod/semver` directly, because no Lockfile parser in this project normalizes versions.
Versions are stored exactly as the Lockfile spells them, so the same Ecosystem yields both `v6.4.3` (Composer's common form) and `1.2.3`, while `x/mod/semver` requires the leading `v`.
The layer prepends it when absent and, more importantly, converts "not a version at all" into an explicit signal: `Compare` returns a boolean that is false for `dev-main`, `1.0.x-dev`, git URLs, and pnpm's `workspace:*`.

That boolean is what keeps the feature honest.
A label the bot cannot order is reported as an undifferentiated change rather than being guessed at, which also covers the Reference Change case (see CONTEXT.md), where the version label is identical on both sides and only the resolved commit moved — there is no direction to report there, and none is invented.
