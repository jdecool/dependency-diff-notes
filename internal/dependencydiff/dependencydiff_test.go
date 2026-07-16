package dependencydiff

import (
	"testing"

	"github.com/jdecool/dependency-diff-notes/internal/composerlock"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		base composerlock.Lock
		head composerlock.Lock
		want Report
	}{
		{
			name: "pure addition",
			base: composerlock.Lock{},
			head: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "1.0.0", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			want: Report{
				Production: []Change{
					{
						Name:        "acme/foo",
						Type:        Added,
						ToVersion:   "1.0.0",
						ToReference: "abc123",
						SourceURL:   "https://example.com/acme/foo",
					},
				},
			},
		},
		{
			name: "pure removal",
			base: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "1.0.0", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: composerlock.Lock{},
			want: Report{
				Production: []Change{
					{
						Name:          "acme/foo",
						Type:          Removed,
						FromVersion:   "1.0.0",
						FromReference: "abc123",
						SourceURL:     "https://example.com/acme/foo",
					},
				},
			},
		},
		{
			name: "version-only update",
			base: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "1.0.0", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "1.1.0", Reference: "def456", SourceURL: "https://example.com/acme/foo"},
				},
			},
			want: Report{
				Production: []Change{
					{
						Name:          "acme/foo",
						Type:          Updated,
						FromVersion:   "1.0.0",
						ToVersion:     "1.1.0",
						FromReference: "abc123",
						ToReference:   "def456",
						SourceURL:     "https://example.com/acme/foo",
					},
				},
			},
		},
		{
			name: "reference-only update with identical version (dev-main case)",
			base: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "dev-main", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "dev-main", Reference: "def456", SourceURL: "https://example.com/acme/foo"},
				},
			},
			want: Report{
				Production: []Change{
					{
						Name:          "acme/foo",
						Type:          Updated,
						FromVersion:   "dev-main",
						ToVersion:     "dev-main",
						FromReference: "abc123",
						ToReference:   "def456",
						SourceURL:     "https://example.com/acme/foo",
					},
				},
			},
		},
		{
			name: "package unchanged is excluded",
			base: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "1.0.0", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "1.0.0", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			want: Report{},
		},
		{
			name: "ordering: added, updated, removed each alphabetical, out-of-order input",
			base: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "zzz/removed-one", Version: "1.0.0"},
					{Name: "bbb/updated-two", Version: "1.0.0"},
					{Name: "aaa/removed-two", Version: "1.0.0"},
					{Name: "yyy/updated-one", Version: "1.0.0"},
				},
			},
			head: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "yyy/updated-one", Version: "2.0.0"},
					{Name: "mmm/added-two", Version: "1.0.0"},
					{Name: "bbb/updated-two", Version: "2.0.0"},
					{Name: "ccc/added-one", Version: "1.0.0"},
				},
			},
			want: Report{
				Production: []Change{
					{Name: "ccc/added-one", Type: Added, ToVersion: "1.0.0"},
					{Name: "mmm/added-two", Type: Added, ToVersion: "1.0.0"},
					{Name: "bbb/updated-two", Type: Updated, FromVersion: "1.0.0", ToVersion: "2.0.0"},
					{Name: "yyy/updated-one", Type: Updated, FromVersion: "1.0.0", ToVersion: "2.0.0"},
					{Name: "aaa/removed-two", Type: Removed, FromVersion: "1.0.0"},
					{Name: "zzz/removed-one", Type: Removed, FromVersion: "1.0.0"},
				},
			},
		},
		{
			name: "removed change keeps base's SourceURL",
			base: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "1.0.0", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: composerlock.Lock{},
			want: Report{
				Production: []Change{
					{Name: "acme/foo", Type: Removed, FromVersion: "1.0.0", SourceURL: "https://example.com/acme/foo"},
				},
			},
		},
		{
			name: "updated change falls back to base's SourceURL when head's is empty",
			base: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "1.0.0", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "1.1.0", SourceURL: ""},
				},
			},
			want: Report{
				Production: []Change{
					{Name: "acme/foo", Type: Updated, FromVersion: "1.0.0", ToVersion: "1.1.0", SourceURL: "https://example.com/acme/foo"},
				},
			},
		},
		{
			name: "production and development sections are computed independently",
			base: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "1.0.0"},
				},
				PackagesDev: []composerlock.Package{
					{Name: "acme/dev-tool", Version: "1.0.0"},
				},
			},
			head: composerlock.Lock{
				Packages: []composerlock.Package{
					{Name: "acme/foo", Version: "2.0.0"},
				},
				PackagesDev: []composerlock.Package{
					{Name: "acme/dev-tool", Version: "1.0.0"},
					{Name: "acme/new-dev-tool", Version: "1.0.0"},
				},
			},
			want: Report{
				Production: []Change{
					{Name: "acme/foo", Type: Updated, FromVersion: "1.0.0", ToVersion: "2.0.0"},
				},
				Development: []Change{
					{Name: "acme/new-dev-tool", Type: Added, ToVersion: "1.0.0"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Diff(tt.base, tt.head)

			if !reportsEqual(got, tt.want) {
				t.Errorf("Diff() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReportIsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		report Report
		want   bool
	}{
		{
			name:   "empty report",
			report: Report{},
			want:   true,
		},
		{
			name: "non-empty report",
			report: Report{
				Production: []Change{
					{Name: "acme/foo", Type: Added, ToVersion: "1.0.0"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.report.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

// reportsEqual compares two Reports, treating nil and empty slices as equal.
func reportsEqual(a, b Report) bool {
	return changesEqual(a.Production, b.Production) && changesEqual(a.Development, b.Development)
}

func changesEqual(a, b []Change) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
