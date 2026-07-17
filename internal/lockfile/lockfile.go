// Package lockfile defines the Ecosystem-agnostic domain model that a
// format-specific parser (composerlock, npmlock, ...) produces and that
// dependencydiff consumes, so the diff logic never needs to know which
// Lockfile format (see CONTEXT.md) a Lock was read from.
package lockfile

// Ecosystem identifies which (language, package manager) pairing a Lock was
// parsed from (see CONTEXT.md).
type Ecosystem int

const (
	Composer Ecosystem = iota
	NPM
	Pnpm
)

// String returns the human-readable Ecosystem name, used as the section
// heading in the Bot Comment.
func (e Ecosystem) String() string {
	switch e {
	case NPM:
		return "npm"
	case Pnpm:
		return "pnpm"
	default:
		return "Composer"
	}
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
