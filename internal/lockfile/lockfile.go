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
)

// String returns the human-readable Ecosystem name, used as the section
// heading in the Bot Comment.
func (e Ecosystem) String() string {
	switch e {
	case NPM:
		return "npm"
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
	Packages    []Package // production dependencies
	PackagesDev []Package // development dependencies
}
