// Package lockfile defines the Ecosystem-agnostic domain model that a
// format-specific parser (composerlock, npmlock, ...) produces and that
// dependencydiff consumes, so the diff logic never needs to know which
// Lockfile format (see CONTEXT.md) a Lock was read from.
package lockfile

import (
	"fmt"
	"strings"
)

// Ecosystem identifies which (language, package manager) pairing a Lock was
// parsed from (see CONTEXT.md).
type Ecosystem int

const (
	Composer Ecosystem = iota
	NPM
	Pnpm
	Yarn
)

// knownEcosystems lists every Ecosystem the bot can actually read, in a
// stable order. It backs both token validation (ParseEcosystem) and the error
// message listing the accepted tokens.
var knownEcosystems = []Ecosystem{Composer, NPM, Pnpm, Yarn}

// String returns the human-readable Ecosystem name, used as the section
// heading in the Bot Comment and as the canonical command-line token (matched
// case-insensitively by ParseEcosystem).
func (e Ecosystem) String() string {
	switch e {
	case NPM:
		return "npm"
	case Pnpm:
		return "pnpm"
	case Yarn:
		return "Yarn"
	default:
		return "Composer"
	}
}

// ParseEcosystem resolves a case-insensitive token (e.g. "composer", "PNPM")
// to its Ecosystem, or returns an error naming the accepted tokens. Only
// Ecosystems the bot can actually read are accepted (see knownEcosystems).
func ParseEcosystem(s string) (Ecosystem, error) {
	for _, e := range knownEcosystems {
		if strings.EqualFold(s, e.String()) {
			return e, nil
		}
	}
	return 0, fmt.Errorf("unknown ecosystem %q (known: %s)", s, strings.Join(ecosystemTokens(), ", "))
}

// ecosystemTokens returns the accepted command-line tokens (lowercased Ecosystem
// names), in knownEcosystems order, for use in validation error messages.
func ecosystemTokens() []string {
	tokens := make([]string, len(knownEcosystems))
	for i, e := range knownEcosystems {
		tokens[i] = strings.ToLower(e.String())
	}
	return tokens
}

// EcosystemSet is a set of Ecosystems, used to represent the Considered
// Ecosystems (see CONTEXT.md) an operator restricts a run to. It is a bitmask
// so it stays a comparable value (Config embeds it and is compared with ==).
// The zero value is the empty set.
type EcosystemSet uint

// With returns s with e added.
func (s EcosystemSet) With(e Ecosystem) EcosystemSet {
	return s | 1<<uint(e)
}

// Contains reports whether e is a member of s.
func (s EcosystemSet) Contains(e Ecosystem) bool {
	return s&(1<<uint(e)) != 0
}

// IsEmpty reports whether s has no members.
func (s EcosystemSet) IsEmpty() bool {
	return s == 0
}

// Package is a single dependency entry from a Lock.
type Package struct {
	Name      string
	Version   string
	Reference string // resolved commit/identifier behind Version, when it can differ independently of Version (see CONTEXT.md: Reference Change); empty when not applicable
	SourceURL string // best-effort browsable link to the package's repository; empty when not available
}

// Lock is the parsed content of one Ecosystem's Lockfile.
type Lock struct {
	// Combined is true when the Lockfile doesn't distinguish production
	// from development dependencies at all (see CONTEXT.md: Production
	// dependencies / Development dependencies) — e.g. pnpm lockfileVersion
	// 9.0, which dropped the per-package "dev" flag lockfileVersion 5.x and
	// 6.0 had. When true, Packages holds every dependency and PackagesDev
	// is unused.
	Combined    bool
	Packages    []Package // production dependencies, or every dependency when Combined is true
	PackagesDev []Package // development dependencies; always empty when Combined is true
}
