// Package pnpmlock parses the YAML content of a pnpm-lock.yaml file, across
// its three structurally different eras (see docs/pnpm-lockfile-schema.md
// and ADR 0004), into the generic lockfile.Lock domain type.
//
// Only the top-level "packages" map is read: it enumerates every resolved
// package (direct and transitive) across the whole lockfile, including
// every workspace member in a pnpm workspace, so no separate handling of
// "importers" or "snapshots" is needed to cover multi-package workspaces —
// see docs/pnpm-lockfile-schema.md's "Does a non-workspace project get
// wrapped in importers?" section for why those two are not the packages'
// system of record.
package pnpmlock

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
	"gopkg.in/yaml.v3"
)

// era identifies which of pnpm-lock.yaml's incompatible shapes a document
// uses (see docs/pnpm-lockfile-schema.md).
type era int

const (
	// eraLegacy is lockfileVersion 5.x: "packages" keys are slash
	// separated, and each entry carries a "dev" flag.
	eraLegacy era = iota
	// eraCurrentSplit is lockfileVersion 6.0: "packages" keys are "@"
	// separated with a leading slash, and each entry still carries a "dev"
	// flag.
	eraCurrentSplit
	// eraCurrentCombined is lockfileVersion 9.0: same key shape as
	// eraCurrentSplit minus the leading slash, but the "dev" flag was
	// removed from the format entirely — see docs/pnpm-lockfile-schema.md's
	// "key finding". A Lock parsed from this era is always Combined.
	eraCurrentCombined
)

// header is decoded first, on its own, to determine which era the rest of
// the document must be parsed as.
type header struct {
	// LockfileVersion is decoded as a raw YAML scalar (rather than a string
	// or float) because the value is written inconsistently across
	// versions — a bare float in 5.x/6.0 (e.g. 6.0) but a quoted string in
	// 9.0 (e.g. '9.0') — and yaml.Node.Value gives the literal scalar text
	// either way.
	LockfileVersion yaml.Node `yaml:"lockfileVersion"`
}

// rawLock holds the one section every era shares: the flat "packages" map.
type rawLock struct {
	Packages map[string]rawPackage `yaml:"packages"`
}

// rawPackage mirrors the one field of a "packages" entry we care about.
// "dev" only has meaning in eraLegacy and eraCurrentSplit (see era); it is
// simply absent, and so decodes to its zero value, in eraCurrentCombined
// documents.
type rawPackage struct {
	Dev bool `yaml:"dev"`
}

// legacyKey matches an eraLegacy "packages" map key: slash separated, e.g.
// "/lodash/4.17.21" or "/@babel/core/7.20.0", with an optional "_"-joined
// peer-dependency suffix on the version segment (e.g.
// "/foo/1.0.0_bar@2.0.0") that this pattern strips.
var legacyKey = regexp.MustCompile(`^/(?:(@[^/]+)/)?([^/]+)/([^/_]+)(?:_.*)?$`)

// currentKey matches an eraCurrentSplit or eraCurrentCombined "packages" map
// key: "@" separated, with an optional leading slash (eraCurrentSplit only)
// and an optional parenthesized peer-dependency suffix (e.g.
// "ts-node@10.9.1(@types/node@14.18.36)") that this pattern strips.
var currentKey = regexp.MustCompile(`^/?(?:(@[^/]+)/)?([^@/]+)@([^(]+)(?:\(.*\))?$`)

// Parse parses the raw YAML content of a pnpm-lock.yaml file.
func Parse(data []byte) (lockfile.Lock, error) {
	var h header
	if err := yaml.Unmarshal(data, &h); err != nil {
		return lockfile.Lock{}, fmt.Errorf("parse pnpm-lock.yaml: %w", err)
	}

	e, err := detectEra(h.LockfileVersion.Value)
	if err != nil {
		return lockfile.Lock{}, err
	}

	var raw rawLock
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return lockfile.Lock{}, fmt.Errorf("parse pnpm-lock.yaml: %w", err)
	}

	keyPattern := currentKey
	if e == eraLegacy {
		keyPattern = legacyKey
	}

	lock := lockfile.Lock{Combined: e == eraCurrentCombined}

	for key, p := range raw.Packages {
		m := keyPattern.FindStringSubmatch(key)
		if m == nil {
			continue
		}

		name := m[2]
		if m[1] != "" {
			name = m[1] + "/" + m[2]
		}

		pkg := lockfile.Package{Name: name, Version: m[3]}

		switch {
		case lock.Combined:
			lock.Packages = append(lock.Packages, pkg)
		case p.Dev:
			lock.PackagesDev = append(lock.PackagesDev, pkg)
		default:
			lock.Packages = append(lock.Packages, pkg)
		}
	}

	sortByName(lock.Packages)
	sortByName(lock.PackagesDev)

	return lock, nil
}

// detectEra maps a raw lockfileVersion scalar (e.g. "5.4", "6.0", "9.0") to
// the era whose "packages" key shape and dev-flag semantics apply — see
// docs/pnpm-lockfile-schema.md, which documents that only 5.x, 6.0, and 9.0
// exist (there is no 7.0 or 8.0 lockfileVersion, despite pnpm CLI majors 7
// and 8).
func detectEra(version string) (era, error) {
	switch {
	case strings.HasPrefix(version, "5"):
		return eraLegacy, nil
	case strings.HasPrefix(version, "6"):
		return eraCurrentSplit, nil
	case strings.HasPrefix(version, "9"):
		return eraCurrentCombined, nil
	default:
		return 0, fmt.Errorf("parse pnpm-lock.yaml: unsupported lockfileVersion %q", version)
	}
}

// sortByName sorts packages alphabetically by Name, in place — YAML map
// iteration order is not stable, so this keeps Parse's output deterministic.
func sortByName(packages []lockfile.Package) {
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})
}
