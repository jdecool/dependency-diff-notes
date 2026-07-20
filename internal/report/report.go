// Package report renders a dependencydiff.Report for its two audiences: the
// Markdown body of the Bot Comment (Render), and plain terminal text for a
// Local Comparison (RenderText) — see CONTEXT.md. The Bot Comment is the
// single comment the bot maintains on a Change Request, identified by a
// hidden marker and updated in place on every pipeline run; a Local
// Comparison instead prints to the terminal and posts nothing.
package report

import (
	"fmt"
	"strings"

	"github.com/jdecool/dependency-diff-notes/internal/dependencydiff"
)

// Marker is a hidden HTML comment identifying a note as the bot's own,
// so the bot can find and update it instead of creating a duplicate.
const Marker = "<!-- dependency-diff-notes -->"

// shortRefLength is how many leading characters of a reference (a Git commit
// hash) are shown, mirroring the short hash format Git itself uses.
const shortRefLength = 7

// Render renders r into the full Markdown body of the Bot Comment,
// including the leading Marker.
//
// Each Ecosystem gets a Markdown heading followed by a collapsible section
// holding its tables. The heading deliberately sits *outside* the <details>
// rather than inside the <summary>: both Forges auto-generate an anchor for a
// Markdown heading, which is what makes a section shareable by URL, and that
// mechanism is far more reliable than hand-written id attributes (GitHub
// rewrites those with a user-content- prefix, and GitLab's sanitizer may drop
// them). Keeping it outside also leaves the Ecosystem name visible when its
// section is collapsed.
//
// Ecosystems and their production dependencies are expanded by default;
// development dependencies are collapsed, since they are the larger and less
// scrutinized half of a typical Change Request.
func Render(r dependencydiff.Report) string {
	var b strings.Builder

	b.WriteString(Marker)
	b.WriteString("\n## Dependency changes\n")

	if r.IsEmpty() {
		b.WriteString("\nNo dependency changes detected.\n")
		return b.String()
	}

	sections := nonEmptySections(r)

	writeSummaryLine(&b, sections)

	for _, s := range sections {
		fmt.Fprintf(&b, "\n### %s\n", s.Ecosystem)
		fmt.Fprintf(&b, "\n<details open>\n<summary><strong>%s</strong></summary>\n", changeCount(sectionTotal(s)))

		if len(s.Combined) > 0 {
			writeGroup(&b, "Dependencies", s.Combined, true)
		} else {
			if len(s.Production) > 0 {
				writeGroup(&b, "Production dependencies", s.Production, true)
			}

			if len(s.Development) > 0 {
				writeGroup(&b, "Development dependencies", s.Development, false)
			}
		}

		b.WriteString("\n</details>\n")
	}

	return b.String()
}

// nonEmptySections returns the Report's sections that actually carry changes,
// in their original order.
func nonEmptySections(r dependencydiff.Report) []dependencydiff.Section {
	sections := make([]dependencydiff.Section, 0, len(r.Sections))

	for _, s := range r.Sections {
		if !s.IsEmpty() {
			sections = append(sections, s)
		}
	}

	return sections
}

// writeSummaryLine writes the one-line overview linking to each Ecosystem's
// heading anchor, so a reader can jump straight to one section of a long
// comment. It is omitted for a single Ecosystem, where it would only repeat
// the section heading immediately below it.
func writeSummaryLine(b *strings.Builder, sections []dependencydiff.Section) {
	if len(sections) < 2 {
		return
	}

	entries := make([]string, len(sections))
	for i, s := range sections {
		name := s.Ecosystem.String()
		entries[i] = fmt.Sprintf("[%s](#%s) %d", name, anchor(name), sectionTotal(s))
	}

	fmt.Fprintf(b, "\n%s\n", strings.Join(entries, " · "))
}

// anchor derives the heading anchor a Forge generates for an Ecosystem name.
// Every Ecosystem name is a single word, so lowercasing is the whole of the
// slug rule that applies here.
func anchor(name string) string {
	return strings.ToLower(name)
}

// sectionTotal counts every change in a Section, whichever grouping it uses.
func sectionTotal(s dependencydiff.Section) int {
	return len(s.Production) + len(s.Development) + len(s.Combined)
}

// changeCount renders a change total as an English noun phrase, so a collapsed
// section still tells the reader how much it hides.
func changeCount(n int) string {
	if n == 1 {
		return "1 change"
	}

	return fmt.Sprintf("%d changes", n)
}

// writeGroup renders one group of a Section as a collapsible table. A blank
// line after the <summary> is required: without it neither Forge parses the
// Markdown table nested inside the HTML block.
func writeGroup(b *strings.Builder, title string, changes []dependencydiff.Change, open bool) {
	openAttr := ""
	if open {
		openAttr = " open"
	}

	fmt.Fprintf(b, "\n<details%s>\n<summary>%s (%d)</summary>\n\n", openAttr, title, len(changes))

	b.WriteString("| Package | Change | Version |\n")
	b.WriteString("|---|---|---|\n")

	for _, c := range changes {
		fmt.Fprintf(b, "| %s | %s | %s |\n", escapeCell(nameMarkdown(c)), changeLabel(c), escapeCell(versionCell(c)))
	}

	b.WriteString("\n</details>\n")
}

// changeLabel renders the Change column: what happened to the package, and for
// an update which way its version moved. An update whose versions cannot be
// ordered — a Composer dev-* alias, a git dependency, or a Reference Change
// where only the resolved commit moved — is reported as a plain change rather
// than being guessed at as an upgrade (see dependencydiff.Direction).
func changeLabel(c dependencydiff.Change) string {
	switch c.Type {
	case dependencydiff.Added:
		return "➕ Added"
	case dependencydiff.Removed:
		return "➖ Removed"
	case dependencydiff.Updated:
		switch c.Direction {
		case dependencydiff.Upgrade:
			return "⬆️ Upgraded"
		case dependencydiff.Downgrade:
			return "⬇️ Downgraded"
		default:
			return "🔄 Changed"
		}
	default:
		return ""
	}
}

// versionCell renders the Version column: the resulting version for an
// addition, the departing one for a removal, and both sides for an update.
func versionCell(c dependencydiff.Change) string {
	switch c.Type {
	case dependencydiff.Added:
		return formatVersionRef(c.ToVersion, c.ToReference)
	case dependencydiff.Removed:
		return formatVersionRef(c.FromVersion, c.FromReference)
	case dependencydiff.Updated:
		from := formatVersionRef(c.FromVersion, c.FromReference)
		to := formatVersionRef(c.ToVersion, c.ToReference)

		return fmt.Sprintf("%s → %s", from, to)
	default:
		return ""
	}
}

// escapeCell escapes the pipes that would otherwise split a table cell in two.
// Package names never contain one, but a version can: npm records git
// dependencies as URLs, which may carry a query string.
func escapeCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

// RenderText renders r as a plain-text Dependency Report for a terminal (see
// CONTEXT.md: Local Comparison), grouped by Ecosystem then Production /
// Development (or a single Dependencies group), with markers for added,
// removed, upgraded, downgraded, and otherwise changed packages. Unlike Render
// it carries no hidden marker and no Markdown — neither tables nor collapsible
// sections mean anything on a terminal — but it reports the same information,
// so a Local Comparison stays a faithful preview of the Bot Comment.
func RenderText(r dependencydiff.Report) string {
	var b strings.Builder

	b.WriteString("Dependency changes\n")

	if r.IsEmpty() {
		b.WriteString("\nNo dependency changes detected.\n")
		return b.String()
	}

	for _, s := range r.Sections {
		if s.IsEmpty() {
			continue
		}

		fmt.Fprintf(&b, "\n%s\n", s.Ecosystem)

		if len(s.Combined) > 0 {
			writeTextGroup(&b, "Dependencies", s.Combined)
			continue
		}

		if len(s.Production) > 0 {
			writeTextGroup(&b, "Production dependencies", s.Production)
		}

		if len(s.Development) > 0 {
			writeTextGroup(&b, "Development dependencies", s.Development)
		}
	}

	return b.String()
}

// writeTextGroup renders one section's changes for RenderText under an
// indented group title. changes is assumed already sorted alphabetically by
// name, as dependencydiff.Diff guarantees; it is not re-sorted here.
func writeTextGroup(b *strings.Builder, title string, changes []dependencydiff.Change) {
	fmt.Fprintf(b, "  %s\n", title)

	for _, c := range changes {
		b.WriteString(renderChangeText(c))
		b.WriteString("\n")
	}
}

// renderChangeText renders a single Change as one plain-text line for
// RenderText, mirroring the Change column of the Bot Comment's tables: ~ marks
// an update whose direction cannot be determined.
func renderChangeText(c dependencydiff.Change) string {
	switch c.Type {
	case dependencydiff.Added:
		return fmt.Sprintf("    + %s  %s", c.Name, formatVersionRef(c.ToVersion, c.ToReference))
	case dependencydiff.Removed:
		return fmt.Sprintf("    - %s  %s", c.Name, formatVersionRef(c.FromVersion, c.FromReference))
	case dependencydiff.Updated:
		from := formatVersionRef(c.FromVersion, c.FromReference)
		to := formatVersionRef(c.ToVersion, c.ToReference)

		return fmt.Sprintf("    %s %s  %s -> %s", directionMarker(c.Direction), c.Name, from, to)
	default:
		return fmt.Sprintf("    %s", c.Name)
	}
}

// directionMarker returns the single-character marker RenderText uses for an
// update, falling back to ~ when no direction could be determined.
func directionMarker(d dependencydiff.Direction) string {
	switch d {
	case dependencydiff.Upgrade:
		return "↑"
	case dependencydiff.Downgrade:
		return "↓"
	default:
		return "~"
	}
}

// HasMarker reports whether body was previously generated by Render (i.e.
// contains Marker), used by the orchestrator to find the bot's existing
// comment among all comments on a Change Request.
func HasMarker(body string) bool {
	return strings.Contains(body, Marker)
}

// nameMarkdown renders a Change's package name, as a Markdown link to
// SourceURL when available, or as plain text otherwise.
func nameMarkdown(c dependencydiff.Change) string {
	if c.SourceURL == "" {
		return c.Name
	}

	return fmt.Sprintf("[%s](%s)", c.Name, c.SourceURL)
}

// formatVersionRef renders a version, appending its short reference in
// parentheses when ref is non-empty so a Reference Change (see CONTEXT.md)
// on a package whose version label doesn't change (typical of dev-*
// branches) is still visible.
func formatVersionRef(version, ref string) string {
	if ref == "" {
		return version
	}

	return fmt.Sprintf("%s (%s)", version, shortRef(ref))
}

// shortRef truncates a reference to shortRefLength characters, mirroring the
// short hash format Git itself uses. References shorter than that are
// returned unchanged.
func shortRef(ref string) string {
	if len(ref) > shortRefLength {
		return ref[:shortRefLength]
	}

	return ref
}
