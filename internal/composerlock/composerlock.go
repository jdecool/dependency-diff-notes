// Package composerlock parses the JSON content of a composer.lock file
// into a typed structure, keeping only the fields relevant to detecting
// dependency changes (name, version, and source reference/URL).
package composerlock

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Package is a single entry from composer.lock's "packages" or "packages-dev" array.
type Package struct {
	Name      string
	Version   string
	Reference string // from source.reference; empty if absent (e.g. dist-only or path packages)
	SourceURL string // from source.url, with a trailing ".git" suffix stripped for a clean browsable link; empty if absent
}

// Lock is the parsed content of a composer.lock file.
type Lock struct {
	Packages    []Package // production dependencies, from the "packages" array
	PackagesDev []Package // development dependencies, from the "packages-dev" array
}

// rawLock mirrors the subset of composer.lock's JSON shape we care about.
// Every other field present in a real composer.lock (dist, require,
// autoload, license, ...) is ignored by omission.
type rawLock struct {
	Packages    []rawPackage `json:"packages"`
	PackagesDev []rawPackage `json:"packages-dev"`
}

type rawPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  *struct {
		URL       string `json:"url"`
		Reference string `json:"reference"`
	} `json:"source"`
}

// Parse parses the raw JSON content of a composer.lock file.
func Parse(data []byte) (Lock, error) {
	var raw rawLock
	if err := json.Unmarshal(data, &raw); err != nil {
		return Lock{}, fmt.Errorf("parse composer.lock: %w", err)
	}

	return Lock{
		Packages:    convertPackages(raw.Packages),
		PackagesDev: convertPackages(raw.PackagesDev),
	}, nil
}

// convertPackages maps rawPackage entries to the public Package shape.
func convertPackages(raw []rawPackage) []Package {
	if len(raw) == 0 {
		return nil
	}

	packages := make([]Package, 0, len(raw))
	for _, p := range raw {
		pkg := Package{
			Name:    p.Name,
			Version: p.Version,
		}

		if p.Source != nil {
			pkg.Reference = p.Source.Reference
			pkg.SourceURL = strings.TrimSuffix(p.Source.URL, ".git")
		}

		packages = append(packages, pkg)
	}

	return packages
}
