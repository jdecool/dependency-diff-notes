package npmlock

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
			name: "realistic lock file with prod and dev packages",
			file: "testdata/realistic.json",
			want: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "lodash", Version: "4.17.21"},
				},
				PackagesDev: []lockfile.Package{
					{Name: "jest", Version: "29.7.0"},
				},
			},
		},
		{
			name: "nested duplicate resolution is ignored, only the hoisted top-level one is reported",
			file: "testdata/duplicate_versions.json",
			want: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "semver", Version: "7.6.0"},
				},
			},
		},
		{
			name: "git dependency's resolved commit becomes the Reference",
			file: "testdata/git_dependency.json",
			want: lockfile.Lock{
				Packages: []lockfile.Package{
					{
						Name:      "acme-fork",
						Version:   "1.2.3",
						Reference: "abcdef1234567890abcdef1234567890abcdef12",
					},
				},
			},
		},
		{
			name: "symlinked workspace member is not reported as a dependency",
			file: "testdata/workspace_link.json",
			want: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "lodash", Version: "4.17.21"},
				},
			},
		},
		{
			name: "empty lock file with no packages map at all",
			file: "testdata/empty.json",
			want: lockfile.Lock{},
		},
		{
			name:    "malformed JSON",
			file:    "testdata/malformed.json",
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

			assertPackagesEqual(t, "Packages", got.Packages, tt.want.Packages)
			assertPackagesEqual(t, "PackagesDev", got.PackagesDev, tt.want.PackagesDev)
		})
	}
}

func TestGitDependencyReference(t *testing.T) {
	tests := []struct {
		name     string
		resolved string
		want     string
	}{
		{
			name:     "git+ssh URL with a commit fragment",
			resolved: "git+ssh://git@github.com/acme/fork.git#abcdef1234567890abcdef1234567890abcdef12",
			want:     "abcdef1234567890abcdef1234567890abcdef12",
		},
		{
			name:     "git+https URL with a commit fragment",
			resolved: "git+https://github.com/acme/fork.git#abcdef1234567890abcdef1234567890abcdef12",
			want:     "abcdef1234567890abcdef1234567890abcdef12",
		},
		{
			name:     "plain git:// URL with a commit fragment",
			resolved: "git://github.com/acme/fork.git#abcdef1234567890abcdef1234567890abcdef12",
			want:     "abcdef1234567890abcdef1234567890abcdef12",
		},
		{
			name:     "registry tarball URL has no reference",
			resolved: "https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz",
			want:     "",
		},
		{
			name:     "git URL without a fragment has no reference",
			resolved: "git+https://github.com/acme/fork.git",
			want:     "",
		},
		{
			name:     "empty resolved has no reference",
			resolved: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gitDependencyReference(tt.resolved); got != tt.want {
				t.Errorf("gitDependencyReference(%q) = %q, want %q", tt.resolved, got, tt.want)
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
