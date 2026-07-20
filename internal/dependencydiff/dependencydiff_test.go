package dependencydiff

import (
	"testing"

	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		base lockfile.Lock
		head lockfile.Lock
		want Section
	}{
		{
			name: "pure addition",
			base: lockfile.Lock{},
			head: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "1.0.0", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			want: Section{
				Ecosystem: lockfile.Composer,
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
			base: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "1.0.0", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: lockfile.Lock{},
			want: Section{
				Ecosystem: lockfile.Composer,
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
			base: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "1.0.0", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "1.1.0", Reference: "def456", SourceURL: "https://example.com/acme/foo"},
				},
			},
			want: Section{
				Ecosystem: lockfile.Composer,
				Production: []Change{
					{
						Name:          "acme/foo",
						Type:          Updated,
						Direction:     Upgrade,
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
			base: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "dev-main", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "dev-main", Reference: "def456", SourceURL: "https://example.com/acme/foo"},
				},
			},
			want: Section{
				Ecosystem: lockfile.Composer,
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
			base: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "1.0.0", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "1.0.0", Reference: "abc123", SourceURL: "https://example.com/acme/foo"},
				},
			},
			want: Section{Ecosystem: lockfile.Composer},
		},
		{
			name: "ordering: alphabetical across every change type, out-of-order input",
			base: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "zzz/removed-one", Version: "1.0.0"},
					{Name: "bbb/updated-two", Version: "1.0.0"},
					{Name: "aaa/removed-two", Version: "1.0.0"},
					{Name: "yyy/updated-one", Version: "1.0.0"},
				},
			},
			head: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "yyy/updated-one", Version: "2.0.0"},
					{Name: "mmm/added-two", Version: "1.0.0"},
					{Name: "bbb/updated-two", Version: "2.0.0"},
					{Name: "ccc/added-one", Version: "1.0.0"},
				},
			},
			want: Section{
				Ecosystem: lockfile.Composer,
				Production: []Change{
					{Name: "aaa/removed-two", Type: Removed, FromVersion: "1.0.0"},
					{Name: "bbb/updated-two", Type: Updated, Direction: Upgrade, FromVersion: "1.0.0", ToVersion: "2.0.0"},
					{Name: "ccc/added-one", Type: Added, ToVersion: "1.0.0"},
					{Name: "mmm/added-two", Type: Added, ToVersion: "1.0.0"},
					{Name: "yyy/updated-one", Type: Updated, Direction: Upgrade, FromVersion: "1.0.0", ToVersion: "2.0.0"},
					{Name: "zzz/removed-one", Type: Removed, FromVersion: "1.0.0"},
				},
			},
		},
		{
			name: "removed change keeps base's SourceURL",
			base: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "1.0.0", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: lockfile.Lock{},
			want: Section{
				Ecosystem: lockfile.Composer,
				Production: []Change{
					{Name: "acme/foo", Type: Removed, FromVersion: "1.0.0", SourceURL: "https://example.com/acme/foo"},
				},
			},
		},
		{
			name: "updated change falls back to base's SourceURL when head's is empty",
			base: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "1.0.0", SourceURL: "https://example.com/acme/foo"},
				},
			},
			head: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "1.1.0", SourceURL: ""},
				},
			},
			want: Section{
				Ecosystem: lockfile.Composer,
				Production: []Change{
					{Name: "acme/foo", Type: Updated, Direction: Upgrade, FromVersion: "1.0.0", ToVersion: "1.1.0", SourceURL: "https://example.com/acme/foo"},
				},
			},
		},
		{
			name: "production and development sections are computed independently",
			base: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "1.0.0"},
				},
				PackagesDev: []lockfile.Package{
					{Name: "acme/dev-tool", Version: "1.0.0"},
				},
			},
			head: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "acme/foo", Version: "2.0.0"},
				},
				PackagesDev: []lockfile.Package{
					{Name: "acme/dev-tool", Version: "1.0.0"},
					{Name: "acme/new-dev-tool", Version: "1.0.0"},
				},
			},
			want: Section{
				Ecosystem: lockfile.Composer,
				Production: []Change{
					{Name: "acme/foo", Type: Updated, Direction: Upgrade, FromVersion: "1.0.0", ToVersion: "2.0.0"},
				},
				Development: []Change{
					{Name: "acme/new-dev-tool", Type: Added, ToVersion: "1.0.0"},
				},
			},
		},
		{
			name: "combined mode when head is Combined: no Production/Development split",
			base: lockfile.Lock{},
			head: lockfile.Lock{
				Combined: true,
				Packages: []lockfile.Package{
					{Name: "lodash", Version: "4.17.21"},
				},
			},
			want: Section{
				Ecosystem: lockfile.Pnpm,
				Combined: []Change{
					{Name: "lodash", Type: Added, ToVersion: "4.17.21"},
				},
			},
		},
		{
			name: "combined mode normalizes a split base instead of double-reporting: a lockfileVersion upgrade mid-Change-Request",
			base: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "lodash", Version: "4.17.21"},
				},
				PackagesDev: []lockfile.Package{
					{Name: "eslint", Version: "8.29.0"},
				},
			},
			head: lockfile.Lock{
				Combined: true,
				Packages: []lockfile.Package{
					{Name: "lodash", Version: "4.17.21"},
					{Name: "eslint", Version: "8.29.0"},
				},
			},
			// Both packages are unchanged across the upgrade (same name and
			// version on both sides), so despite base being split and head
			// being Combined, nothing should show up as removed or added.
			want: Section{Ecosystem: lockfile.Pnpm},
		},
		{
			name: "the ecosystem passed to Diff is echoed onto the Section, regardless of its packages",
			base: lockfile.Lock{},
			head: lockfile.Lock{
				Packages: []lockfile.Package{
					{Name: "lodash", Version: "4.17.21"},
				},
			},
			want: Section{
				Ecosystem: lockfile.NPM,
				Production: []Change{
					{Name: "lodash", Type: Added, ToVersion: "4.17.21"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Diff(tt.want.Ecosystem, tt.base, tt.head)

			if !sectionsEqual(got, tt.want) {
				t.Errorf("Diff() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSectionIsEmpty(t *testing.T) {
	tests := []struct {
		name    string
		section Section
		want    bool
	}{
		{
			name:    "empty section",
			section: Section{},
			want:    true,
		},
		{
			name: "non-empty section",
			section: Section{
				Production: []Change{
					{Name: "acme/foo", Type: Added, ToVersion: "1.0.0"},
				},
			},
			want: false,
		},
		{
			name: "non-empty Combined section",
			section: Section{
				Combined: []Change{
					{Name: "lodash", Type: Added, ToVersion: "4.17.21"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.section.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
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
			name:   "no sections at all",
			report: Report{},
			want:   true,
		},
		{
			name: "every section empty",
			report: Report{
				Sections: []Section{
					{Ecosystem: lockfile.Composer},
					{Ecosystem: lockfile.NPM},
				},
			},
			want: true,
		},
		{
			name: "one non-empty section among empty ones",
			report: Report{
				Sections: []Section{
					{Ecosystem: lockfile.Composer},
					{
						Ecosystem: lockfile.NPM,
						Production: []Change{
							{Name: "lodash", Type: Added, ToVersion: "4.17.21"},
						},
					},
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

// sectionsEqual compares two Sections, treating nil and empty slices as equal.
func sectionsEqual(a, b Section) bool {
	return a.Ecosystem == b.Ecosystem &&
		changesEqual(a.Production, b.Production) &&
		changesEqual(a.Development, b.Development) &&
		changesEqual(a.Combined, b.Combined)
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

// TestDiff_Direction covers the direction reported on Updated changes, which
// is what lets the report distinguish an upgrade from a downgrade instead of
// showing every update alike.
func TestDiff_Direction(t *testing.T) {
	tests := []struct {
		name        string
		fromVersion string
		toVersion   string
		want        Direction
	}{
		{
			name:        "higher version is an upgrade",
			fromVersion: "1.0.0",
			toVersion:   "1.1.0",
			want:        Upgrade,
		},
		{
			name:        "lower version is a downgrade",
			fromVersion: "2.1.0",
			toVersion:   "2.0.0",
			want:        Downgrade,
		},
		{
			name:        "major downgrade",
			fromVersion: "3.0.0",
			toVersion:   "2.9.9",
			want:        Downgrade,
		},
		{
			name:        "Composer v prefix does not defeat comparison",
			fromVersion: "v6.4.2",
			toVersion:   "v6.4.3",
			want:        Upgrade,
		},
		{
			name:        "leaving a pre-release for its release is an upgrade",
			fromVersion: "1.0.0-rc.1",
			toVersion:   "1.0.0",
			want:        Upgrade,
		},
		{
			name:        "dev branch alias has no reportable direction",
			fromVersion: "dev-main",
			toVersion:   "dev-main",
			want:        DirectionUnknown,
		},
		{
			name:        "moving off a dev branch alias has no reportable direction",
			fromVersion: "dev-main",
			toVersion:   "1.0.0",
			want:        DirectionUnknown,
		},
		{
			name:        "equal versions differing only by build metadata have no direction",
			fromVersion: "1.0.0+build1",
			toVersion:   "1.0.0+build2",
			want:        DirectionUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := lockfile.Lock{
				Packages: []lockfile.Package{{Name: "acme/foo", Version: tt.fromVersion, Reference: "abc123"}},
			}
			head := lockfile.Lock{
				Packages: []lockfile.Package{{Name: "acme/foo", Version: tt.toVersion, Reference: "def456"}},
			}

			got := Diff(lockfile.Composer, base, head)

			if len(got.Production) != 1 {
				t.Fatalf("Diff() produced %d production changes, want exactly 1", len(got.Production))
			}

			if got.Production[0].Direction != tt.want {
				t.Errorf("Diff() direction for %q -> %q = %v, want %v", tt.fromVersion, tt.toVersion, got.Production[0].Direction, tt.want)
			}
		})
	}
}

// TestDiff_DirectionOnlyOnUpdates guards the invariant documented on Change:
// additions and removals never carry a direction, since there is no pair of
// versions to order.
func TestDiff_DirectionOnlyOnUpdates(t *testing.T) {
	base := lockfile.Lock{
		Packages: []lockfile.Package{{Name: "acme/gone", Version: "1.0.0"}},
	}
	head := lockfile.Lock{
		Packages: []lockfile.Package{{Name: "acme/new", Version: "2.0.0"}},
	}

	for _, c := range Diff(lockfile.Composer, base, head).Production {
		if c.Direction != DirectionUnknown {
			t.Errorf("change %q of type %v carries direction %v, want DirectionUnknown", c.Name, c.Type, c.Direction)
		}
	}
}
