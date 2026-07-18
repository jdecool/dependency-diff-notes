package yarnlock

import (
	"strings"

	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
)

// classicIndentStep is the indentation width one level of a Classic
// yarn.lock entry uses (see docs/yarn-lockfile-schema.md's grammar
// citation: INDENT_STEP = 2). A block header sits at indent 0, its direct
// fields (version, resolved, integrity, dependencies:) at indent 2; deeper
// indentation (a "dependencies:" field's own nested list) is content this
// package doesn't need and skips.
const classicIndentStep = 2

// parseClassic parses a Classic (v1) yarn.lock with a small hand-rolled
// line scanner: the format is YAML-like but not YAML (see the package doc
// comment and docs/yarn-lockfile-schema.md), so gopkg.in/yaml.v3 cannot
// read it. It never fails — content it doesn't recognize (a field it
// doesn't track, a malformed line) is simply skipped, matching this
// package's minimal, best-effort scope (see ADR 0004's discussion of
// hand-rolled vs. library parsing).
func parseClassic(data []byte) lockfile.Lock {
	lock := lockfile.Lock{Combined: true}

	var (
		inBlock  bool
		name     string
		version  string
		resolved string
	)

	flush := func() {
		if !inBlock || name == "" {
			return
		}

		lock.Packages = append(lock.Packages, lockfile.Package{
			Name:      name,
			Version:   version,
			Reference: gitReference(resolved),
		})
	}

	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(rawLine, "\r")

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))

		switch indent {
		case 0:
			flush()

			inBlock = true
			name = packageName(firstSpecKey(trimmed))
			version = ""
			resolved = ""

		case classicIndentStep:
			if !inBlock {
				continue
			}

			switch {
			case strings.HasPrefix(trimmed, "version "):
				version = unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "version ")))
			case strings.HasPrefix(trimmed, "resolved "):
				resolved = unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "resolved ")))
			}
		}
	}

	flush()

	sortByName(lock.Packages)

	return lock
}

// firstSpecKey returns the first comma-separated spec key from a block
// header line (e.g. "foo@^1.0.0, foo@^1.2.0:" -> "foo@^1.0.0"), which is
// enough to name the package: every comma-joined key in one block resolves
// to the same package.
func firstSpecKey(header string) string {
	header = strings.TrimSuffix(header, ":")
	return strings.SplitN(header, ", ", 2)[0]
}

// gitReference extracts the resolved commit from a git dependency's
// "resolved" field, so a Reference Change (see CONTEXT.md) is still visible
// when a git dependency is pinned to a branch or tag whose version label
// doesn't change. Two forms exist (see docs/yarn-lockfile-schema.md):
//   - explicit "git+..." / "git://..." protocol: "...#<commit>" suffix.
//   - GitHub shorthand ("user/repo#ref"): resolves to a codeload tarball
//     URL of the form ".../tar.gz/<commit>", with no "#" fragment at all.
//
// A registry tarball's "resolved" URL also carries a "#<hash>" fragment
// (a checksum, historically redundant with "integrity"), which must NOT be
// mistaken for a git reference — it never changes independently of the
// package's published version, so surfacing it would only add noise. Since
// registry tarballs are always ".tgz" (not ".tar.gz"), checking for
// "git+"/"git://" prefixes and the "/tar.gz/" codeload path distinguishes
// the two without needing to inspect the fragment's shape.
func gitReference(resolved string) string {
	if strings.HasPrefix(resolved, "git+") || strings.HasPrefix(resolved, "git://") {
		if i := strings.LastIndex(resolved, "#"); i != -1 {
			return resolved[i+1:]
		}
		return ""
	}

	const codeloadMarker = "/tar.gz/"
	if i := strings.LastIndex(resolved, codeloadMarker); i != -1 {
		return resolved[i+len(codeloadMarker):]
	}

	return ""
}
