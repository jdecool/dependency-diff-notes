// Package semver compares two version labels as they appear verbatim in a
// Lockfile, so a Dependency Change of type Updated can be reported as an
// upgrade or a downgrade rather than an undifferentiated change (see
// CONTEXT.md).
//
// It is a thin adaptation layer over golang.org/x/mod/semver (see
// docs/adr/0007-semver-comparison-dependency.md), whose only job is to bridge
// the gap between what Lockfiles actually store and what that package expects:
// no Lockfile parser in this project normalizes versions, so the same
// Ecosystem yields both "v6.4.3" and "1.2.3", alongside labels that are not
// versions at all ("dev-main", "1.0.x-dev", git URLs, "workspace:*").
package semver

import (
	"strings"

	"golang.org/x/mod/semver"
)

// Compare compares two version labels. It returns -1 if a sorts before b, 0 if
// they are equal, and +1 if a sorts after b.
//
// The boolean reports whether the comparison is meaningful at all: it is false
// when either label is not a version this package can order, in which case the
// int is 0 and must be ignored. Callers are expected to fall back on reporting
// an undifferentiated change rather than inventing a direction.
//
// Comparison follows the Semantic Versioning specification as implemented by
// golang.org/x/mod/semver: a missing minor or patch is treated as zero (so
// "5.4" equals "5.4.0"), build metadata is ignored ("1.0.0+build9" equals
// "1.0.0"), and a pre-release always sorts before the release it precedes
// ("1.0.0-rc.1" before "1.0.0").
func Compare(a, b string) (int, bool) {
	va, vb := normalize(a), normalize(b)
	if !semver.IsValid(va) || !semver.IsValid(vb) {
		return 0, false
	}

	return semver.Compare(va, vb), true
}

// normalize prepends the "v" that golang.org/x/mod/semver requires but that
// most Lockfiles omit ("5.4" -> "v5.4"), leaving labels that already carry it
// untouched ("v6.4.3", the form Composer often records). It makes no attempt
// to repair anything else: a label that is not a version stays invalid, which
// is exactly what Compare reports through its boolean.
func normalize(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}

	return "v" + version
}
