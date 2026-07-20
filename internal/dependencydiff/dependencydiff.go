// Package dependencydiff computes the Dependency Report (see CONTEXT.md) for
// a run: the Dependency Changes for every active Ecosystem, computed between
// the merge-base of the Change Request's target branch and the Change
// Request's current commit.
package dependencydiff

import (
	"sort"

	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
	"github.com/jdecool/dependency-diff-notes/internal/semver"
)

// ChangeType classifies how a package changed between the base and head Lock.
type ChangeType int

const (
	Added ChangeType = iota
	Updated
	Removed
)

// Direction refines a Change of type Updated with the direction the version
// moved in, when the two version labels can be ordered at all.
type Direction int

const (
	// DirectionUnknown means no direction is reported: either the Change is
	// not an Updated one, or at least one of its version labels is not a
	// version this project can order (a Composer dev-* alias, a git
	// dependency, pnpm's workspace:*), or it is a Reference Change (see
	// CONTEXT.md) where the version label did not move at all.
	DirectionUnknown Direction = iota
	Upgrade
	Downgrade
)

// Change describes one package's change between the base and head Lock.
type Change struct {
	Name          string
	Type          ChangeType
	Direction     Direction // always DirectionUnknown unless Type == Updated
	FromVersion   string    // empty when Type == Added
	ToVersion     string    // empty when Type == Removed
	FromReference string    // empty when Type == Added, or when the package has no source.reference
	ToReference   string    // empty when Type == Removed, or when the package has no source.reference
	SourceURL     string    // best-effort browsable link to the package's repository; may be empty
}

// Section is one Ecosystem's slice of a Dependency Report (see CONTEXT.md):
// either its Production and Development Dependency Changes, or — when the
// Lockfile doesn't distinguish the two (lockfile.Lock.Combined) — a single
// undifferentiated Combined list instead. Exactly one of (Production,
// Development) or Combined is ever populated for a given Section.
type Section struct {
	Ecosystem   lockfile.Ecosystem
	Production  []Change // empty when Combined is used instead
	Development []Change // empty when Combined is used instead
	Combined    []Change // empty when Production/Development are used instead
}

// IsEmpty reports whether the Section contains no changes at all.
func (s Section) IsEmpty() bool {
	return len(s.Production) == 0 && len(s.Development) == 0 && len(s.Combined) == 0
}

// Report is the full Dependency Report for a run: one Section per Ecosystem
// the bot attempted to read.
type Report struct {
	Sections []Section
}

// IsEmpty reports whether every Section of the Report is empty.
func (r Report) IsEmpty() bool {
	for _, s := range r.Sections {
		if !s.IsEmpty() {
			return false
		}
	}

	return true
}

// Diff computes one Ecosystem's Section: the Dependency Changes between base
// (the merge-base / target branch snapshot) and head (the Change Request's
// current snapshot) of a single Lock.
//
// If either side is Combined (e.g. a Change Request that upgrades pnpm
// across the lockfileVersion 6.0 -> 9.0 boundary, where the dev/prod
// distinction disappears — see lockfile.Lock), both sides are normalized to
// their combined package list before diffing, so the result never
// double-reports a package as both removed-from-Production and
// added-to-Combined.
func Diff(ecosystem lockfile.Ecosystem, base, head lockfile.Lock) Section {
	if base.Combined || head.Combined {
		return Section{
			Ecosystem: ecosystem,
			Combined:  diffPackages(allPackages(base), allPackages(head)),
		}
	}

	return Section{
		Ecosystem:   ecosystem,
		Production:  diffPackages(base.Packages, head.Packages),
		Development: diffPackages(base.PackagesDev, head.PackagesDev),
	}
}

// allPackages returns every package in l, regardless of whether l
// distinguishes Production from Development.
func allPackages(l lockfile.Lock) []lockfile.Package {
	if l.Combined {
		return l.Packages
	}

	all := make([]lockfile.Package, 0, len(l.Packages)+len(l.PackagesDev))
	all = append(all, l.Packages...)
	all = append(all, l.PackagesDev...)

	return all
}

// diffPackages computes the Dependency Changes for a single section (either
// production or development packages), independently of the other section.
//
// The result is sorted alphabetically by package name across every change
// type, not grouped by type: the rendered report carries the type in a column
// of its own, so a reader looks a package up by name rather than by what
// happened to it. No secondary sort key on ChangeType is applied, because it
// could never break a tie — packages are indexed by name on both sides, so one
// package yields at most one Change and no two entries here share a name.
func diffPackages(base, head []lockfile.Package) []Change {
	baseByName := indexByName(base)
	headByName := indexByName(head)

	var added, updated, removed []Change

	for name, headPkg := range headByName {
		basePkg, inBase := baseByName[name]
		if !inBase {
			added = append(added, Change{
				Name:        name,
				Type:        Added,
				ToVersion:   headPkg.Version,
				ToReference: headPkg.Reference,
				SourceURL:   headPkg.SourceURL,
			})
			continue
		}

		if basePkg.Version != headPkg.Version || basePkg.Reference != headPkg.Reference {
			sourceURL := headPkg.SourceURL
			if sourceURL == "" {
				sourceURL = basePkg.SourceURL
			}

			updated = append(updated, Change{
				Name:          name,
				Type:          Updated,
				Direction:     direction(basePkg.Version, headPkg.Version),
				FromVersion:   basePkg.Version,
				ToVersion:     headPkg.Version,
				FromReference: basePkg.Reference,
				ToReference:   headPkg.Reference,
				SourceURL:     sourceURL,
			})
		}
	}

	for name, basePkg := range baseByName {
		if _, inHead := headByName[name]; !inHead {
			removed = append(removed, Change{
				Name:          name,
				Type:          Removed,
				FromVersion:   basePkg.Version,
				FromReference: basePkg.Reference,
				SourceURL:     basePkg.SourceURL,
			})
		}
	}

	if len(added) == 0 && len(updated) == 0 && len(removed) == 0 {
		return nil
	}

	changes := make([]Change, 0, len(added)+len(updated)+len(removed))
	changes = append(changes, added...)
	changes = append(changes, updated...)
	changes = append(changes, removed...)

	sortByName(changes)

	return changes
}

// direction reports which way a package's version moved, for a Change already
// known to be an update. It is DirectionUnknown whenever the two labels cannot
// be ordered, and equally when they compare equal — the latter being the
// Reference Change case (see CONTEXT.md), where only the resolved commit moved
// and there is no direction to report.
func direction(fromVersion, toVersion string) Direction {
	cmp, ok := semver.Compare(fromVersion, toVersion)
	if !ok {
		return DirectionUnknown
	}

	switch {
	case cmp < 0:
		return Upgrade
	case cmp > 0:
		return Downgrade
	default:
		return DirectionUnknown
	}
}

// indexByName builds a lookup of packages by name.
func indexByName(packages []lockfile.Package) map[string]lockfile.Package {
	index := make(map[string]lockfile.Package, len(packages))
	for _, pkg := range packages {
		index[pkg.Name] = pkg
	}

	return index
}

// sortByName sorts changes alphabetically by Name, in place.
func sortByName(changes []Change) {
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Name < changes[j].Name
	})
}
