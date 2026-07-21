package report_test

import (
	"errors"
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
			name: "production only, every change type, one removed without SourceURL",
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
								Direction:   dependencydiff.Upgrade,
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
				"\n<details open>\n<summary><strong>3 changes</strong></summary>\n" +
				"\n<details open>\n<summary>Production dependencies (3)</summary>\n\n" +
				"| Package | Change | Version |\n" +
				"|---|---|---|\n" +
				"| [acme/foo](https://example.com/foo) | ➕ Added | 1.2.3 |\n" +
				"| [acme/bar](https://example.com/bar) | ⬆️ Upgraded | 1.0.0 → 1.1.0 |\n" +
				"| acme/baz | ➖ Removed | 0.9.0 |\n" +
				"\n</details>\n" +
				"\n</details>\n",
		},
		{
			name: "development group is collapsed while production stays open",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem: lockfile.Composer,
						Production: []dependencydiff.Change{
							{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.0.0"},
						},
						Development: []dependencydiff.Change{
							{Name: "acme/dev-tool", Type: dependencydiff.Added, ToVersion: "2.0.0"},
						},
					},
				},
			},
			want: "<!-- dependency-diff-notes -->\n" +
				"## Dependency changes\n" +
				"\n### Composer\n" +
				"\n<details open>\n<summary><strong>2 changes</strong></summary>\n" +
				"\n<details open>\n<summary>Production dependencies (1)</summary>\n\n" +
				"| Package | Change | Version |\n" +
				"|---|---|---|\n" +
				"| acme/foo | ➕ Added | 1.0.0 |\n" +
				"\n</details>\n" +
				"\n<details>\n<summary>Development dependencies (1)</summary>\n\n" +
				"| Package | Change | Version |\n" +
				"|---|---|---|\n" +
				"| acme/dev-tool | ➕ Added | 2.0.0 |\n" +
				"\n</details>\n" +
				"\n</details>\n",
		},
		{
			name: "single change uses the singular noun",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem: lockfile.Composer,
						Production: []dependencydiff.Change{
							{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.0.0"},
						},
					},
				},
			},
			want: "<!-- dependency-diff-notes -->\n" +
				"## Dependency changes\n" +
				"\n### Composer\n" +
				"\n<details open>\n<summary><strong>1 change</strong></summary>\n" +
				"\n<details open>\n<summary>Production dependencies (1)</summary>\n\n" +
				"| Package | Change | Version |\n" +
				"|---|---|---|\n" +
				"| acme/foo | ➕ Added | 1.0.0 |\n" +
				"\n</details>\n" +
				"\n</details>\n",
		},
		{
			name: "reference change without a version move is reported as a plain change",
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
				"\n<details open>\n<summary><strong>1 change</strong></summary>\n" +
				"\n<details open>\n<summary>Production dependencies (1)</summary>\n\n" +
				"| Package | Change | Version |\n" +
				"|---|---|---|\n" +
				"| [acme/branch-lib](https://example.com/branch-lib) | 🔄 Changed | dev-main (aaaaaaa) → dev-main (bbbbbbb) |\n" +
				"\n</details>\n" +
				"\n</details>\n",
		},
		{
			name: "combined section renders a single open Dependencies group",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem: lockfile.Pnpm,
						Combined: []dependencydiff.Change{
							{Name: "lodash", Type: dependencydiff.Added, ToVersion: "4.17.21"},
						},
					},
				},
			},
			want: "<!-- dependency-diff-notes -->\n" +
				"## Dependency changes\n" +
				"\n### pnpm\n" +
				"\n<details open>\n<summary><strong>1 change</strong></summary>\n" +
				"\n<details open>\n<summary>Dependencies (1)</summary>\n\n" +
				"| Package | Change | Version |\n" +
				"|---|---|---|\n" +
				"| lodash | ➕ Added | 4.17.21 |\n" +
				"\n</details>\n" +
				"\n</details>\n",
		},
		{
			name: "two ecosystems get a linked summary line and one section each",
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
							{Name: "lodash", Type: dependencydiff.Removed, FromVersion: "4.17.21"},
						},
					},
				},
			},
			want: "<!-- dependency-diff-notes -->\n" +
				"## Dependency changes\n" +
				"\n[Composer](#composer) 1 · [npm](#npm) 1\n" +
				"\n### Composer\n" +
				"\n<details open>\n<summary><strong>1 change</strong></summary>\n" +
				"\n<details open>\n<summary>Production dependencies (1)</summary>\n\n" +
				"| Package | Change | Version |\n" +
				"|---|---|---|\n" +
				"| acme/foo | ➕ Added | 1.0.0 |\n" +
				"\n</details>\n" +
				"\n</details>\n" +
				"\n### npm\n" +
				"\n<details open>\n<summary><strong>1 change</strong></summary>\n" +
				"\n<details open>\n<summary>Production dependencies (1)</summary>\n\n" +
				"| Package | Change | Version |\n" +
				"|---|---|---|\n" +
				"| lodash | ➖ Removed | 4.17.21 |\n" +
				"\n</details>\n" +
				"\n</details>\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Every body closes on EndMarker, so that one Render serves
			// both Report Destinations: appending it here asserts that
			// invariant once instead of repeating it in each want.
			want := tt.want + report.EndMarker + "\n"

			got := report.Render(tt.in)
			if got != want {
				t.Errorf("Render() =\n%q\nwant\n%q", got, want)
			}
		})
	}
}

func TestRender_Structural(t *testing.T) {
	composerOnly := dependencydiff.Report{
		Sections: []dependencydiff.Section{
			{
				Ecosystem:  lockfile.Composer,
				Production: []dependencydiff.Change{{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.0.0"}},
			},
		},
	}

	tests := []struct {
		name            string
		in              dependencydiff.Report
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "marker always present",
			in:           composerOnly,
			wantContains: []string{report.Marker},
		},
		{
			name:            "summary line omitted for a single ecosystem",
			in:              composerOnly,
			wantNotContains: []string{"(#composer)"},
		},
		{
			name:            "development group absent when Development is empty",
			in:              composerOnly,
			wantNotContains: []string{"Development dependencies"},
		},
		{
			name: "production group absent when Production is empty",
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
			name: "ecosystem heading stays outside the details, so its anchor survives",
			in:   composerOnly,
			// The heading must precede the <details>, never sit inside the <summary>.
			wantContains:    []string{"### Composer\n\n<details open>"},
			wantNotContains: []string{"<summary><strong>Composer"},
		},
		{
			name:            "package without SourceURL renders as plain text, not a link",
			in:              dependencydiff.Report{Sections: []dependencydiff.Section{{Ecosystem: lockfile.Composer, Production: []dependencydiff.Change{{Name: "acme/foo", Type: dependencydiff.Removed, FromVersion: "1.0.0"}}}}},
			wantContains:    []string{"| acme/foo | ➖ Removed | 1.0.0 |"},
			wantNotContains: []string{"[acme/foo]"},
		},
		{
			name:         "package with SourceURL renders as a Markdown link",
			in:           dependencydiff.Report{Sections: []dependencydiff.Section{{Ecosystem: lockfile.Composer, Production: []dependencydiff.Change{{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.0.0", SourceURL: "https://example.com/foo"}}}}},
			wantContains: []string{"[acme/foo](https://example.com/foo)"},
		},
		{
			name: "Production/Development groups absent when a section uses Combined instead",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{Ecosystem: lockfile.Pnpm, Combined: []dependencydiff.Change{{Name: "lodash", Type: dependencydiff.Added, ToVersion: "4.17.21"}}},
				},
			},
			wantContains:    []string{"<summary>Dependencies (1)</summary>"},
			wantNotContains: []string{"Production dependencies", "Development dependencies"},
		},
		{
			name: "an empty section among non-empty ones is skipped entirely, summary line included",
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
			wantNotContains: []string{"### Composer", "Composer](#composer)"},
		},
		{
			name: "a pipe inside a version does not break the table",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem:  lockfile.NPM,
						Production: []dependencydiff.Change{{Name: "weird", Type: dependencydiff.Added, ToVersion: "git+https://example.com/p.git?a=1|b=2"}},
					},
				},
			},
			wantContains: []string{`| weird | ➕ Added | git+https://example.com/p.git?a=1\|b=2 |`},
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

// TestRender_TableSeparatedFromSummary guards the one formatting rule both
// Forges are unforgiving about: a Markdown table nested in an HTML block is
// only parsed when a blank line separates it from the <summary> above it.
func TestRender_TableSeparatedFromSummary(t *testing.T) {
	got := report.Render(dependencydiff.Report{
		Sections: []dependencydiff.Section{
			{
				Ecosystem:   lockfile.Composer,
				Production:  []dependencydiff.Change{{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.0.0"}},
				Development: []dependencydiff.Change{{Name: "acme/bar", Type: dependencydiff.Added, ToVersion: "1.0.0"}},
			},
		},
	})

	if strings.Contains(got, "</summary>\n|") {
		t.Errorf("Render() puts a table directly after a </summary> without a blank line:\n%s", got)
	}

	if !strings.Contains(got, "</summary>\n\n| Package |") {
		t.Errorf("Render() = %q, want every table preceded by a blank line after its summary", got)
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

func TestRenderText_ExactBody(t *testing.T) {
	tests := []struct {
		name string
		in   dependencydiff.Report
		want string
	}{
		{
			name: "fully empty report",
			in:   dependencydiff.Report{},
			want: "Dependency changes\n" +
				"\n" +
				"No dependency changes detected.\n",
		},
		{
			name: "every marker, including both update directions",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{
						Ecosystem: lockfile.Composer,
						Production: []dependencydiff.Change{
							{Name: "acme/foo", Type: dependencydiff.Added, ToVersion: "1.2.3"},
							{Name: "acme/bar", Type: dependencydiff.Updated, Direction: dependencydiff.Upgrade, FromVersion: "1.0.0", ToVersion: "1.1.0"},
							{Name: "acme/old", Type: dependencydiff.Updated, Direction: dependencydiff.Downgrade, FromVersion: "2.1.0", ToVersion: "2.0.0"},
							{Name: "acme/dev", Type: dependencydiff.Updated, FromVersion: "dev-main", ToVersion: "dev-main", FromReference: "0000000aaaa", ToReference: "1111111bbbb"},
							{Name: "acme/baz", Type: dependencydiff.Removed, FromVersion: "0.9.0"},
						},
					},
				},
			},
			want: "Dependency changes\n" +
				"\nComposer\n" +
				"  Production dependencies\n" +
				"    + acme/foo  1.2.3\n" +
				"    ↑ acme/bar  1.0.0 -> 1.1.0\n" +
				"    ↓ acme/old  2.1.0 -> 2.0.0\n" +
				"    ~ acme/dev  dev-main (0000000) -> dev-main (1111111)\n" +
				"    - acme/baz  0.9.0\n",
		},
		{
			name: "combined section with an empty section skipped",
			in: dependencydiff.Report{
				Sections: []dependencydiff.Section{
					{Ecosystem: lockfile.Composer}, // empty: must be skipped
					{
						Ecosystem: lockfile.Yarn,
						Combined: []dependencydiff.Change{
							{Name: "lodash", Type: dependencydiff.Updated, Direction: dependencydiff.Upgrade, FromVersion: "4.17.20", ToVersion: "4.17.21"},
						},
					},
				},
			},
			want: "Dependency changes\n" +
				"\nYarn\n" +
				"  Dependencies\n" +
				"    ↑ lodash  4.17.20 -> 4.17.21\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := report.RenderText(tt.in)
			if got != tt.want {
				t.Errorf("RenderText() mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// regionBody is a stand-in for a Render output: what matters to the splicing
// tests is only that it is delimited by the two markers and ends on a newline.
var regionBody = report.Marker + "\n## Dependency changes\n" + report.EndMarker + "\n"

func TestSpliceRegion(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        string
	}{
		{
			name:        "empty description holds the region alone",
			description: "",
			want:        regionBody,
		},
		{
			name:        "description without a trailing newline gets one blank line",
			description: "Fixes the flaky checkout.",
			want:        "Fixes the flaky checkout.\n\n" + regionBody,
		},
		{
			name:        "description ending on one newline gets one more",
			description: "Fixes the flaky checkout.\n",
			want:        "Fixes the flaky checkout.\n\n" + regionBody,
		},
		{
			name:        "description already ending on a blank line gets none added",
			description: "Fixes the flaky checkout.\n\n",
			want:        "Fixes the flaky checkout.\n\n" + regionBody,
		},
		{
			name:        "an existing region at the end is replaced",
			description: "Intro.\n\n" + report.Marker + "\nstale\n" + report.EndMarker + "\n",
			want:        "Intro.\n\n" + regionBody,
		},
		{
			name:        "a region the author moved is updated where it stands",
			description: "Intro.\n\n" + report.Marker + "\nstale\n" + report.EndMarker + "\n\nCloses #12.\n",
			want:        "Intro.\n\n" + regionBody + "\nCloses #12.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := report.SpliceRegion(tt.description, regionBody)
			if err != nil {
				t.Fatalf("SpliceRegion() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("SpliceRegion() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestSpliceRegionIsIdempotent(t *testing.T) {
	// Publishing the same report twice must produce a byte-identical
	// description: that equality is exactly what tells the orchestrator no
	// API write is warranted (see docs/adr/0008-report-destination.md).
	once, err := report.SpliceRegion("Intro.\n", regionBody)
	if err != nil {
		t.Fatalf("SpliceRegion() unexpected error: %v", err)
	}

	twice, err := report.SpliceRegion(once, regionBody)
	if err != nil {
		t.Fatalf("SpliceRegion() unexpected error on second pass: %v", err)
	}

	if twice != once {
		t.Errorf("SpliceRegion() is not idempotent:\n first: %q\nsecond: %q", once, twice)
	}
}

func TestRemoveRegion(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        string
	}{
		{
			name:        "a description with no region is returned untouched",
			description: "Fixes the flaky checkout.\n",
			want:        "Fixes the flaky checkout.\n",
		},
		{
			name:        "an empty description is returned untouched",
			description: "",
			want:        "",
		},
		{
			name:        "a trailing region is removed, leaving the author's text",
			description: "Intro.\n\n" + regionBody,
			want:        "Intro.\n\n",
		},
		{
			name:        "text below the region survives",
			description: "Intro.\n\n" + report.Marker + "\nstale\n" + report.EndMarker + "\n\nCloses #12.\n",
			want:        "Intro.\n\n\nCloses #12.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := report.RemoveRegion(tt.description)
			if err != nil {
				t.Fatalf("RemoveRegion() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("RemoveRegion() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// TestSpliceRemoveCycleDoesNotDrift covers the decision that an operator can
// switch the Report Destination back and forth without the description slowly
// filling with blank lines: the region owns the newline that closes it, so
// removing and re-inserting returns to the same bytes.
func TestSpliceRemoveCycleDoesNotDrift(t *testing.T) {
	const original = "Intro.\n"

	withRegion, err := report.SpliceRegion(original, regionBody)
	if err != nil {
		t.Fatalf("SpliceRegion() unexpected error: %v", err)
	}

	stripped, err := report.RemoveRegion(withRegion)
	if err != nil {
		t.Fatalf("RemoveRegion() unexpected error: %v", err)
	}

	again, err := report.SpliceRegion(stripped, regionBody)
	if err != nil {
		t.Fatalf("SpliceRegion() unexpected error on re-insertion: %v", err)
	}

	if again != withRegion {
		t.Errorf("splice/remove/splice drifted:\n first: %q\n after: %q", withRegion, again)
	}
}

// TestUnterminatedRegionIsRefused covers the decision that the bot never
// guesses where its region ends: assuming it runs to the end of the document
// would delete whatever a human wrote below the missing closing marker.
func TestUnterminatedRegionIsRefused(t *testing.T) {
	description := "Intro.\n\n" + report.Marker + "\nstale\n\nCloses #12.\n"

	if _, err := report.SpliceRegion(description, regionBody); !errors.Is(err, report.ErrUnterminatedRegion) {
		t.Errorf("SpliceRegion() error = %v, want ErrUnterminatedRegion", err)
	}

	if _, err := report.RemoveRegion(description); !errors.Is(err, report.ErrUnterminatedRegion) {
		t.Errorf("RemoveRegion() error = %v, want ErrUnterminatedRegion", err)
	}
}

// TestEndMarkerDoesNotContainMarker guards the property that lets a region be
// located by searching for Marker: if EndMarker ever contained it, the search
// could match the closing delimiter and mis-delimit the region.
func TestEndMarkerDoesNotContainMarker(t *testing.T) {
	if strings.Contains(report.EndMarker, report.Marker) {
		t.Errorf("EndMarker %q contains Marker %q, which would break region location", report.EndMarker, report.Marker)
	}
}
