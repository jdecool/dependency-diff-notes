// Package npmlock parses the JSON content of a package-lock.json file
// (lockfileVersion 2 or 3 — the flat "packages" map produced by npm 7+)
// into the generic lockfile.Lock domain type.
package npmlock

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
)

// rawLock mirrors the subset of package-lock.json's JSON shape we care
// about. Every other top-level field (name, version, lockfileVersion,
// requires, ...) is ignored by omission.
type rawLock struct {
	Packages map[string]rawPackage `json:"packages"`
}

// rawPackage mirrors one entry of the "packages" map. Fields such as
// license, engines, or bin are ignored by omission.
type rawPackage struct {
	Version     string `json:"version"`
	Resolved    string `json:"resolved"`
	Dev         bool   `json:"dev"`
	DevOptional bool   `json:"devOptional"`
	Link        bool   `json:"link"` // a symlinked workspace member, not an installed dependency
}

// topLevelPackagePath matches a "packages" map key for a direct (hoisted)
// node_modules entry, e.g. "node_modules/lodash" or "node_modules/@scope/pkg".
// It does not match the root entry (key "", the project itself) or a nested
// duplicate resolution (e.g. "node_modules/foo/node_modules/lodash",
// present when npm couldn't hoist every version of a package to the top
// level) — only the hoisted top-level resolution is reported, mirroring
// Composer's one-resolved-version-per-package model.
var topLevelPackagePath = regexp.MustCompile(`^node_modules/(@[^/]+/[^/]+|[^/]+)$`)

// Parse parses the raw JSON content of a package-lock.json file.
func Parse(data []byte) (lockfile.Lock, error) {
	var raw rawLock
	if err := json.Unmarshal(data, &raw); err != nil {
		return lockfile.Lock{}, fmt.Errorf("parse package-lock.json: %w", err)
	}

	var lock lockfile.Lock

	for path, p := range raw.Packages {
		if p.Link {
			continue
		}

		m := topLevelPackagePath.FindStringSubmatch(path)
		if m == nil {
			continue
		}

		pkg := lockfile.Package{
			Name:      m[1],
			Version:   p.Version,
			Reference: gitDependencyReference(p.Resolved),
		}

		if p.Dev || p.DevOptional {
			lock.PackagesDev = append(lock.PackagesDev, pkg)
		} else {
			lock.Packages = append(lock.Packages, pkg)
		}
	}

	sortByName(lock.Packages)
	sortByName(lock.PackagesDev)

	return lock, nil
}

// gitDependencyReference extracts the resolved commit from a git
// dependency's "resolved" URL (e.g.
// "git+ssh://git@github.com/owner/repo.git#abcdef1234567890..."), so a
// Reference Change (see CONTEXT.md) is still visible when a git dependency
// is pinned to a branch whose version label doesn't change. A registry
// dependency's "resolved" tarball URL carries no such fragment, so this
// returns "" for the common case.
func gitDependencyReference(resolved string) string {
	if !strings.HasPrefix(resolved, "git+") && !strings.HasPrefix(resolved, "git://") {
		return ""
	}

	i := strings.LastIndex(resolved, "#")
	if i == -1 {
		return ""
	}

	return resolved[i+1:]
}

// sortByName sorts packages alphabetically by Name, in place — the
// "packages" map has no stable iteration order, so this keeps Parse's
// output deterministic.
func sortByName(packages []lockfile.Package) {
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})
}
