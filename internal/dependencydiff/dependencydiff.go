// Package dependencydiff computes the Dependency Report (see CONTEXT.md) for
// a run: the Dependency Changes for every active Ecosystem, computed between
// the merge-base of the Change Request's target branch and the Change
// Request's current commit.
package dependencydiff

import (
	"sort"

	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
)

// ChangeType classifies how a package changed between the base and head Lock.
type ChangeType int

const (
	Added ChangeType = iota
	Updated
	Removed
)

// Change describes one package's change between the base and head Lock.
type Change struct {
	Name          string
	Type          ChangeType
	FromVersion   string // empty when Type == Added
	ToVersion     string // empty when Type == Removed
	FromReference string // empty when Type == Added, or when the package has no source.reference
	ToReference   string // empty when Type == Removed, or when the package has no source.reference
	SourceURL     string // best-effort browsable link to the package's repository; may be empty
}

// Section is one Ecosystem's slice of a Dependency Report (see CONTEXT.md):
// its Production and Development Dependency Changes.
type Section struct {
	Ecosystem   lockfile.Ecosystem
	Production  []Change
	Development []Change
}

// IsEmpty reports whether the Section contains no changes at all.
func (s Section) IsEmpty() bool {
	return len(s.Production) == 0 && len(s.Development) == 0
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
func Diff(ecosystem lockfile.Ecosystem, base, head lockfile.Lock) Section {
	return Section{
		Ecosystem:   ecosystem,
		Production:  diffPackages(base.Packages, head.Packages),
		Development: diffPackages(base.PackagesDev, head.PackagesDev),
	}
}

// diffPackages computes the Dependency Changes for a single section (either
// production or development packages), independently of the other section.
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

	sortByName(added)
	sortByName(updated)
	sortByName(removed)

	if len(added) == 0 && len(updated) == 0 && len(removed) == 0 {
		return nil
	}

	changes := make([]Change, 0, len(added)+len(updated)+len(removed))
	changes = append(changes, added...)
	changes = append(changes, updated...)
	changes = append(changes, removed...)

	return changes
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
