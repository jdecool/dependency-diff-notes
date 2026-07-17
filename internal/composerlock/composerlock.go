// Package composerlock parses the JSON content of a composer.lock file into
// the generic lockfile.Lock domain type, keeping only the fields relevant to
// detecting dependency changes (name, version, and source reference/URL).
package composerlock

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
)

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
func Parse(data []byte) (lockfile.Lock, error) {
	var raw rawLock
	if err := json.Unmarshal(data, &raw); err != nil {
		return lockfile.Lock{}, fmt.Errorf("parse composer.lock: %w", err)
	}

	return lockfile.Lock{
		Packages:    convertPackages(raw.Packages),
		PackagesDev: convertPackages(raw.PackagesDev),
	}, nil
}

// convertPackages maps rawPackage entries to the generic lockfile.Package shape.
func convertPackages(raw []rawPackage) []lockfile.Package {
	if len(raw) == 0 {
		return nil
	}

	packages := make([]lockfile.Package, 0, len(raw))
	for _, p := range raw {
		pkg := lockfile.Package{
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
