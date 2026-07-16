// Package dependencydiff computes the Dependency Changes between two
// composer.lock snapshots (see CONTEXT.md): the merge-base of the Change
// Request's target branch, and the Change Request's current commit.
package dependencydiff

import (
	"sort"

	"github.com/jdecool/dependency-diff-notes/internal/composerlock"
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

// Report is the full set of Dependency Changes between two composer.lock snapshots.
type Report struct {
	Production  []Change // from Lock.Packages
	Development []Change // from Lock.PackagesDev
}

// IsEmpty reports whether the Report contains no changes at all.
func (r Report) IsEmpty() bool {
	return len(r.Production) == 0 && len(r.Development) == 0
}

// Diff computes the Dependency Changes between base (the merge-base / target
// branch snapshot) and head (the Change Request's current snapshot).
func Diff(base, head composerlock.Lock) Report {
	return Report{
		Production:  diffPackages(base.Packages, head.Packages),
		Development: diffPackages(base.PackagesDev, head.PackagesDev),
	}
}

// diffPackages computes the Dependency Changes for a single section (either
// production or development packages), independently of the other section.
func diffPackages(base, head []composerlock.Package) []Change {
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
func indexByName(packages []composerlock.Package) map[string]composerlock.Package {
	index := make(map[string]composerlock.Package, len(packages))
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
