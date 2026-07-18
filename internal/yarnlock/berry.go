package yarnlock

import (
	"fmt"
	"strings"

	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
	"gopkg.in/yaml.v3"
)

// metadataKey is the one top-level key in a Berry yarn.lock that isn't a
// package entry (see docs/yarn-lockfile-schema.md): it carries the
// lockfile's own format version, not a dependency.
const metadataKey = "__metadata"

// softLinkType marks a workspace-local package (no physical install) in a
// Berry lockfile — analogous to npm's "link": true entries — and is
// skipped the same way: it isn't an installed dependency.
const softLinkType = "soft"

// gitCommitMarker precedes the resolved commit in a Berry git dependency's
// "resolution" locator (see docs/yarn-lockfile-schema.md), e.g.
// "underscore@https://github.com/jashkenas/underscore.git#commit=<sha>".
const gitCommitMarker = "#commit="

// rawBerryPackage mirrors the fields of one Berry "packages" entry this
// package cares about. "dependencies" and "checksum" are ignored by
// omission.
type rawBerryPackage struct {
	Version    string `yaml:"version"`
	Resolution string `yaml:"resolution"`
	LinkType   string `yaml:"linkType"`
}

// parseBerry parses a Berry (v2+) yarn.lock: real YAML, a flat map at the
// document root keyed by comma-joined spec strings (see
// docs/yarn-lockfile-schema.md), plus the one non-package metadataKey.
func parseBerry(data []byte) (lockfile.Lock, error) {
	var raw map[string]rawBerryPackage
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return lockfile.Lock{}, fmt.Errorf("parse yarn.lock: %w", err)
	}

	lock := lockfile.Lock{Combined: true}

	for key, p := range raw {
		if key == metadataKey || p.LinkType == softLinkType {
			continue
		}

		name, locator := splitNameAndLocator(p.Resolution)
		if name == "" {
			name = packageName(firstSpecKey(key))
		}

		lock.Packages = append(lock.Packages, lockfile.Package{
			Name:      name,
			Version:   p.Version,
			Reference: gitCommitFromLocator(locator),
		})
	}

	sortByName(lock.Packages)

	return lock, nil
}

// splitNameAndLocator splits a Berry "resolution" field (e.g.
// "@algolia/autocomplete-core@npm:1.17.9") into the package's bare name and
// its locator (the part after the name's own "@", e.g. "npm:1.17.9") — the
// resolved identity of the package, as opposed to the range(s) requested by
// its consumers that the entry's header key carries instead.
func splitNameAndLocator(resolution string) (name, locator string) {
	resolution = unquote(strings.TrimSpace(resolution))

	start := 0
	if strings.HasPrefix(resolution, "@") {
		start = 1
	}

	i := strings.Index(resolution[start:], "@")
	if i == -1 {
		return resolution, ""
	}

	idx := start + i

	return resolution[:idx], resolution[idx+1:]
}

// gitCommitFromLocator extracts the resolved commit from a git
// dependency's locator (see gitCommitMarker), so a Reference Change (see
// CONTEXT.md) is still visible when a git dependency is pinned to a branch
// or tag whose version label doesn't change. A registry locator (e.g.
// "npm:1.17.9") never contains gitCommitMarker, so this returns "" for the
// common case.
func gitCommitFromLocator(locator string) string {
	i := strings.Index(locator, gitCommitMarker)
	if i == -1 {
		return ""
	}

	return locator[i+len(gitCommitMarker):]
}
