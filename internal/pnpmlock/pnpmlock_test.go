package pnpmlock

import (
	"os"
	"testing"

	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		want    lockfile.Lock
		wantErr bool
	}{
		{
			name: "legacy lockfileVersion 5.x: split via the dev flag, scoped name, peer suffix stripped",
			file: "testdata/legacy_5x.yaml",
			want: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "@babel/core", Version: "7.20.0"},
					{Name: "lodash", Version: "4.17.21"},
				},
				PackagesDev: []lockfile.Package{
					{Name: "eslint", Version: "8.29.0"},
					{Name: "ts-node", Version: "10.9.1"},
				},
			},
		},
		{
			name: "current lockfileVersion 6.0: split via the dev flag, leading slash, parenthesized peer suffix stripped",
			file: "testdata/current_6_split.yaml",
			want: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "@scope/pkg", Version: "1.0.0"},
					{Name: "lodash", Version: "4.17.21"},
				},
				PackagesDev: []lockfile.Package{
					{Name: "eslint", Version: "8.29.0"},
					{Name: "ts-node", Version: "10.9.1"},
				},
			},
		},
		{
			name: "current lockfileVersion 9.0: no dev flag at all, Combined instead of a split",
			file: "testdata/current_9_combined.yaml",
			want: lockfile.Lock{
				Combined: true,
				Packages: []lockfile.Package{
					{Name: "@scope/pkg", Version: "1.0.0"},
					{Name: "eslint", Version: "8.29.0"},
					{Name: "lodash", Version: "4.17.21"},
				},
			},
		},
		{
			name: "empty lock file with no packages map at all",
			file: "testdata/empty.yaml",
			want: lockfile.Lock{Combined: true},
		},
		{
			name:    "malformed YAML",
			file:    "testdata/malformed.yaml",
			wantErr: true,
		},
		{
			name:    "unsupported lockfileVersion",
			file:    "testdata/unsupported_version.yaml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.file)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			got, err := Parse(data)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() unexpected error: %v", err)
			}

			if got.Combined != tt.want.Combined {
				t.Errorf("Combined = %v, want %v", got.Combined, tt.want.Combined)
			}
			assertPackagesEqual(t, "Packages", got.Packages, tt.want.Packages)
			assertPackagesEqual(t, "PackagesDev", got.PackagesDev, tt.want.PackagesDev)
		})
	}
}

func TestDetectEra(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    era
		wantErr bool
	}{
		{name: "5.4 is legacy", version: "5.4", want: eraLegacy},
		{name: "5.0 is legacy", version: "5.0", want: eraLegacy},
		{name: "6.0 is current split", version: "6.0", want: eraCurrentSplit},
		{name: "9.0 is current combined", version: "9.0", want: eraCurrentCombined},
		{name: "unknown major version is an error", version: "3.0", wantErr: true},
		{name: "empty version is an error", version: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detectEra(tt.version)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("detectEra(%q) error = nil, want an error", tt.version)
				}
				return
			}
			if err != nil {
				t.Fatalf("detectEra(%q) unexpected error: %v", tt.version, err)
			}
			if got != tt.want {
				t.Errorf("detectEra(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestPackageKeyPatterns(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string // "legacy" or "current"
		key         string
		wantMatch   bool
		wantScope   string
		wantName    string
		wantVersion string
	}{
		{name: "legacy unscoped", pattern: "legacy", key: "/lodash/4.17.21", wantMatch: true, wantName: "lodash", wantVersion: "4.17.21"},
		{name: "legacy scoped", pattern: "legacy", key: "/@babel/core/7.20.0", wantMatch: true, wantScope: "@babel", wantName: "core", wantVersion: "7.20.0"},
		{name: "legacy unscoped with peer suffix stripped", pattern: "legacy", key: "/foo/1.0.0_bar@2.0.0", wantMatch: true, wantName: "foo", wantVersion: "1.0.0"},
		{name: "legacy key with no leading slash does not match", pattern: "legacy", key: "lodash/4.17.21", wantMatch: false},

		{name: "current unscoped with leading slash (6.0)", pattern: "current", key: "/lodash@4.17.21", wantMatch: true, wantName: "lodash", wantVersion: "4.17.21"},
		{name: "current unscoped without leading slash (9.0)", pattern: "current", key: "lodash@4.17.21", wantMatch: true, wantName: "lodash", wantVersion: "4.17.21"},
		{name: "current scoped with leading slash (6.0)", pattern: "current", key: "/@scope/name@1.2.3", wantMatch: true, wantScope: "@scope", wantName: "name", wantVersion: "1.2.3"},
		{name: "current scoped without leading slash (9.0)", pattern: "current", key: "@scope/name@1.2.3", wantMatch: true, wantScope: "@scope", wantName: "name", wantVersion: "1.2.3"},
		{name: "current unscoped with parenthesized peer suffix stripped", pattern: "current", key: "ts-node@10.9.1(@types/node@14.18.36)", wantMatch: true, wantName: "ts-node", wantVersion: "10.9.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := currentKey
			if tt.pattern == "legacy" {
				pattern = legacyKey
			}

			m := pattern.FindStringSubmatch(tt.key)

			if !tt.wantMatch {
				if m != nil {
					t.Fatalf("FindStringSubmatch(%q) = %v, want no match", tt.key, m)
				}
				return
			}
			if m == nil {
				t.Fatalf("FindStringSubmatch(%q) = nil, want a match", tt.key)
			}

			if m[1] != tt.wantScope {
				t.Errorf("scope = %q, want %q", m[1], tt.wantScope)
			}
			if m[2] != tt.wantName {
				t.Errorf("name = %q, want %q", m[2], tt.wantName)
			}
			if m[3] != tt.wantVersion {
				t.Errorf("version = %q, want %q", m[3], tt.wantVersion)
			}
		})
	}
}

func assertPackagesEqual(t *testing.T, field string, got, want []lockfile.Package) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s length = %d, want %d (got %#v)", field, len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %#v, want %#v", field, i, got[i], want[i])
		}
	}
}
