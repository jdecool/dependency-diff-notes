// Package yarnlock parses the content of a yarn.lock file, across Yarn's
// two structurally incompatible formats — Classic (v1, a bespoke,
// YAML-like-but-not-YAML format) and Berry (v2+, real YAML) — into the
// generic lockfile.Lock domain type.
//
// Neither format records a production-vs-development distinction per
// package (see CONTEXT.md: Production dependencies / Development
// dependencies) — that information lives only in package.json, which this
// package does not read — so a Lock returned by Parse is always Combined.
package yarnlock

import (
	"bytes"
	"sort"
	"strings"

	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
)

// classicHeader is the exact comment line Yarn writes at the top of every
// Classic (v1) lockfile it generates (see docs/yarn-lockfile-schema.md).
// Its presence within the first few lines is used to distinguish Classic
// from Berry, since both are otherwise superficially similar text formats.
const classicHeader = "# yarn lockfile v1"

// classicHeaderScanLines caps how many leading lines are checked for
// classicHeader, so a large Berry lockfile is never scanned in full just to
// rule out the (fixed-position) Classic header.
const classicHeaderScanLines = 5

// Parse parses the raw content of a yarn.lock file, detecting whether it is
// Classic or Berry from its header and dispatching accordingly.
func Parse(data []byte) (lockfile.Lock, error) {
	if isClassic(data) {
		return parseClassic(data), nil
	}

	return parseBerry(data)
}

// isClassic reports whether data looks like a Classic (v1) yarn.lock, by
// checking for classicHeader within its first few lines.
func isClassic(data []byte) bool {
	lines := bytes.SplitN(data, []byte("\n"), classicHeaderScanLines+1)
	for i := 0; i < len(lines) && i < classicHeaderScanLines; i++ {
		if bytes.Contains(lines[i], []byte(classicHeader)) {
			return true
		}
	}

	return false
}

// packageName extracts a package's bare name from a single spec key (e.g.
// "lodash@^4.17.21" -> "lodash", "@babel/core@^7.20.0" -> "@babel/core"),
// stripping surrounding quotes first. A scoped name's own leading "@"
// doesn't separate the name from the range — only the next "@" does.
func packageName(specKey string) string {
	specKey = unquote(strings.TrimSpace(specKey))

	start := 0
	if strings.HasPrefix(specKey, "@") {
		start = 1
	}

	if i := strings.Index(specKey[start:], "@"); i != -1 {
		return specKey[:start+i]
	}

	return specKey
}

// unquote strips a single pair of surrounding double quotes, if present.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}

	return s
}

// sortByName sorts packages by Name, then by Version as a tie-breaker, in
// place. Both formats can legitimately resolve the same package name to two
// different versions in one lockfile (a version-range conflict between
// consumers) — dependencydiff only keeps one entry per name, so without a
// deterministic tie-break, which version "wins" could vary between
// otherwise-identical runs. This doesn't pick the objectively best entry
// (it's a lexicographic, not semver, comparison), only a stable one.
func sortByName(packages []lockfile.Package) {
	sort.SliceStable(packages, func(i, j int) bool {
		if packages[i].Name != packages[j].Name {
			return packages[i].Name < packages[j].Name
		}
		return packages[i].Version < packages[j].Version
	})
}
