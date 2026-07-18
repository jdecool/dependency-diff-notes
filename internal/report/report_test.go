package report_test

import (
	"strings"
	"testing"

	"github.com/jdecool/dependency-diff-notes/internal/dependencydiff"
	"github.com/jdecool/dependency-diff-notes/internal/lockfile"
	"github.com/jdecool/dependency-diff-notes/internal/report"
)

func TestRender_ExactBody(t *testing.T) {
	tests := []struct {
		name string
		in   dependencydiff.Report
		want string
	}{
		{
			name: "fully empty report",
			in:   dependencydiff.Report{},
			want: "<!-- dependency-diff-notes -->\n" +
				"## Dependency changes\n" +
				"\n" +
				"No dependency changes detected.\n",
		},
		{
			name: "production only, all three change types, one removed without SourceURL",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem: lockfile.Composer,
						Production: []dependencydiff.Change{
							{
								Name:      "acme/foo",
								Type:      dependencydiff.Added,
								ToVersion: "1.2.3",
								SourceURL: "https://example.com/foo",
							},
							{
								Name:        "acme/bar",
								Type:        dependencydiff.Updated,
								FromVersion: "1.0.0",
								ToVersion:   "1.1.0",
								SourceURL:   "https://example.com/bar",
							},
							{
								Name:        "acme/baz",
								Type:        dependencydiff.Removed,
								FromVersion: "0.9.0",
								// no SourceURL: must render as plain text
							},
						},
					},
				},
			},
			want: "<!-- dependency-diff-notes -->\n" +
				"## Dependency changes\n" +
				"\n### Composer\n" +
				"\n#### Production dependencies\n" +
				"\n**Added**\n\n" +
				"- [acme/foo](https://example.com/foo): added 1.2.3\n" +
				"\n**Updated**\n\n" +
				"- [acme/bar](https://example.com/bar): 1.0.0 → 1.1.0\n" +
				"\n**Removed**\n\n" +
				"- acme/baz: removed 0.9.0\n",
		},
		{
			name: "development only, added dev branch package with reference",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem: lockfile.Composer,
						Development: []dependencydiff.Change{
							{
								Name:        "acme/dev-lib",
								Type:        dependencydiff.Added,
								ToVersion:   "dev-main",
								ToReference: "a1b2c3d4e5f6",
								SourceURL:   "https://example.com/dev-lib",
							},
						},
					},
				},
			},
			want: "<!-- dependency-diff-notes -->\n" +
				"## Dependency changes\n" +
				"\n### Composer\n" +
				"\n#### Development dependencies\n" +
				"\n**Added**\n\n" +
				"- [acme/dev-lib](https://example.com/dev-lib): added dev-main (a1b2c3d)\n",
		},
		{
			name: "updated entry with only a reference change (version label unchanged)",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem: lockfile.Composer,
						Production: []dependencydiff.Change{
							{
								Name:          "acme/branch-lib",
								Type:          dependencydiff.Updated,
								FromVersion:   "dev-main",
								ToVersion:     "dev-main",
								FromReference: "aaaaaaa1111",
								ToReference:   "bbbbbbb2222",
								SourceURL:     "https://example.com/branch-lib",
							},
						},
					},
				},
			},
			want: "<!-- dependency-diff-notes -->\n" +
				"## Dependency changes\n" +
				"\n### Composer\n" +
				"\n#### Production dependencies\n" +
				"\n**Updated**\n\n" +
				"- [acme/branch-lib](https://example.com/branch-lib): dev-main (aaaaaaa) → dev-main (bbbbbbb)\n",
		},
		{
			name: "two ecosystems in the same report, each their own section",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem: lockfile.Composer,
						Production: []dependencydiff.Change{
							{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.0.0"},
						},
					},
					{
						Ecosystem: lockfile.NPM,
						Production: []dependencydiff.Change{
							{Name: "lodash", Type: dependencydiff.Added, ToVersion: "4.17.21"},
						},
					},
				},
			},
			want: "<!-- dependency-diff-notes -->\n" +
				"## Dependency changes\n" +
				"\n### Composer\n" +
				"\n#### Production dependencies\n" +
				"\n**Added**\n\n" +
				"- acme/foo: added 1.0.0\n" +
				"\n### npm\n" +
				"\n#### Production dependencies\n" +
				"\n**Added**\n\n" +
				"- lodash: added 4.17.21\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := report.Render(tt.in)
			if got != tt.want {
				t.Errorf("Render() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestRender_Structural(t *testing.T) {
	tests := []struct {
		name            string
		in              dependencydiff.Report
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "marker always present",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem:  lockfile.Composer,
						Production: []dependencydiff.Change{{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.0.0"}},
					},
				},
			},
			wantContains: []string{report.Marker},
		},
		{
			name: "development section absent when Development is empty",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem:  lockfile.Composer,
						Production: []dependencydiff.Change{{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.0.0"}},
					},
				},
			},
			wantNotContains: []string{"Development dependencies"},
		},
		{
			name: "production section absent when Production is empty",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem:   lockfile.Composer,
						Development: []dependencydiff.Change{{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.0.0"}},
					},
				},
			},
			wantNotContains: []string{"Production dependencies"},
		},
		{
			name: "group label absent when a group has no entries",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem:  lockfile.Composer,
						Production: []dependencydiff.Change{{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.0.0"}},
					},
				},
			},
			wantNotContains: []string{"**Updated**", "**Removed**"},
		},
		{
			name: "package without SourceURL renders as plain text, not a link",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem:  lockfile.Composer,
						Production: []dependencydiff.Change{{Name: "acme/foo", Type: dependencydiff.Removed, FromVersion: "1.0.0"}},
					},
				},
			},
			wantContains:    []string{"- acme/foo: removed 1.0.0"},
			wantNotContains: []string{"[acme/foo]"},
		},
		{
			name: "package with SourceURL renders as a Markdown link",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem:  lockfile.Composer,
						Production: []dependencydiff.Change{{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.0.0", SourceURL: "https://example.com/foo"}},
					},
				},
			},
			wantContains: []string{"[acme/foo](https://example.com/foo)"},
		},
		{
			name: "an empty section among non-empty ones is skipped entirely",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{Ecosystem: lockfile.Composer},
					{
						Ecosystem:  lockfile.NPM,
						Production: []dependencydiff.Change{{Name: "lodash", Type: dependencydiff.Added, ToVersion: "4.17.21"}},
					},
				},
			},
			wantContains:    []string{"### npm"},
			wantNotContains: []string{"### Composer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := report.Render(tt.in)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("Render() = %q, want it to contain %q", got, want)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("Render() = %q, want it NOT to contain %q", got, notWant)
				}
			}
		})
	}
}

func TestHasMarker(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "body rendered by Render",
			body: report.Render(dependencydiff.Report{}),
			want: true,
		},
		{
			name: "body containing the marker among other content",
			body: "some preamble\n" + report.Marker + "\nmore content",
			want: true,
		},
		{
			name: "unrelated note body",
			body: "Thanks for the merge request!",
			want: false,
		},
		{
			name: "empty body",
			body: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := report.HasMarker(tt.body)
			if got != tt.want {
				t.Errorf("HasMarker(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}
