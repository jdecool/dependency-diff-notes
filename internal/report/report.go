// Package report renders a dependencydiff.Report for its two audiences: the
// Markdown body of the Bot Comment (Render), and plain terminal text for a
// Local Comparison (RenderText) — see CONTEXT.md. The Bot Comment is the
// single comment the bot maintains on a Change Request, identified by a
// hidden marker and updated in place on every pipeline run; a Local
// Comparison instead prints to the terminal and posts nothing.
package report

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jdecool/dependency-diff-notes/internal/dependencydiff"
)

// Marker is a hidden HTML comment identifying content as the bot's own, so
// the bot can find and update it instead of creating a duplicate. In a Bot
// Comment it merely identifies the comment; in a Description Region (see
// CONTEXT.md) it is also the opening delimiter of the region the bot owns.
//
// Its value is deliberately unchanged from when the Bot Comment was the only
// Report Destination: Bot Comments already published on open Change Requests
// keep being recognized, so none of them is orphaned and duplicated.
const Marker = "<!-- dependency-diff-notes -->"

// EndMarker closes a Description Region. It is emitted in a Bot Comment too,
// where it delimits nothing (the bot owns the whole body), so that one Render
// serves both Report Destinations.
//
// It contains Marker's text but not Marker itself (the leading slash breaks
// the substring), so searching a description for Marker can never match the
// closing delimiter by accident.
const EndMarker = "<!-- /dependency-diff-notes -->"

// ErrUnterminatedRegion reports a description carrying Marker with no
// EndMarker after it. The bot refuses to guess where its region ends rather
// than assume it runs to the end of the document: that assumption would delete
// whatever a human wrote below it, and this error means the document has
// demonstrably been hand-edited (see docs/adr/0008-report-destination.md).
var ErrUnterminatedRegion = errors.New("description contains the opening marker without a closing one: the bot's region cannot be delimited, restore or remove the marker by hand")

// shortRefLength is how many leading characters of a reference (a Git commit
// hash) are shown, mirroring the short hash format Git itself uses.
const shortRefLength = 7

// Fold is the Report Fold (see CONTEXT.md): the outermost level of a rendered
// Dependency Report that starts collapsed.
//
// Its three values are not independent presets but three positions on one
// axis, ordered from the innermost fold outwards — which is why widening this
// into a set of per-level booleans would be a step backwards: the levels are
// nested, so only their outermost shut one is ever visible to a reader, and
// naming that one level says everything the other flags could.
type Fold int

const (
	// FoldDevelopment folds at Development dependencies, leaving Ecosystem
	// sections and production tables open. It is the default, and first here
	// so that the zero value is the historical behavior.
	FoldDevelopment Fold = iota
	// FoldEcosystem folds at the Ecosystem section.
	FoldEcosystem
	// FoldNone folds nowhere: every table is open on arrival.
	FoldNone
)

// String returns the fold's CLI spelling, so error messages and flag
// descriptions use the same word an operator types.
func (f Fold) String() string {
	switch f {
	case FoldEcosystem:
		return "ecosystem"
	case FoldNone:
		return "none"
	default:
		return "development"
	}
}

// ecosystemOpen reports whether an Ecosystem section starts expanded.
func (f Fold) ecosystemOpen() bool {
	return f != FoldEcosystem
}

// developmentOpen reports whether a Development dependencies group starts
// expanded.
//
// FoldEcosystem opens it deliberately: the section enclosing it is shut, so
// the group's own state is invisible until a reader expands that section, and
// at that moment they have asked to see the section — showing them a second
// layer of shut headers would charge a click for the counts they can already
// read in the section summary.
func (f Fold) developmentOpen() bool {
	return f != FoldDevelopment
}

// Render renders r into the full Markdown body published at either Report
// Destination (see CONTEXT.md), delimited by Marker and EndMarker. The body is
// identical for both: the content is the same and both Forges render it with
// the same Markdown engine, so one rendering keeps a Local Comparison a
// faithful preview whatever the configured destination.
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
// What starts collapsed is fold's to decide (see Fold). The default,
// FoldDevelopment, expands Ecosystems and their production dependencies and
// collapses development dependencies, since those are the larger and less
// scrutinized half of a typical Change Request.
func Render(r dependencydiff.Report, fold Fold) string {
	var b strings.Builder

	b.WriteString(Marker)
	b.WriteString("\n## Dependency changes\n")

	if r.IsEmpty() {
		b.WriteString("\nNo dependency changes detected.\n")
		b.WriteString(EndMarker + "\n")
		return b.String()
	}

	sections := nonEmptySections(r)

	writeSummaryLine(&b, sections)

	for _, s := range sections {
		fmt.Fprintf(&b, "\n### %s\n", s.Ecosystem)
		fmt.Fprintf(&b, "\n<details%s>\n<summary><strong>%s</strong></summary>\n", openAttr(fold.ecosystemOpen()), changeCount(sectionTotal(s)))

		if len(s.Combined) > 0 {
			writeGroup(&b, "Dependencies", s.Combined, true)
		} else {
			if len(s.Production) > 0 {
				writeGroup(&b, "Production dependencies", s.Production, true)
			}

			if len(s.Development) > 0 {
				writeGroup(&b, "Development dependencies", s.Development, fold.developmentOpen())
			}
		}

		b.WriteString("\n</details>\n")
	}

	b.WriteString(EndMarker + "\n")

	return b.String()
}

// SpliceRegion returns description with the bot's Description Region (see
// CONTEXT.md) set to body, which must be a Render output — that is, already
// delimited by Marker and EndMarker.
//
// An existing region is replaced where it stands, so a region the author has
// moved is updated in place rather than dragged back to the bottom. When
// there is none, the region is appended at the end, separated by one blank
// line. Everything outside the two markers is returned byte for byte as it
// came in: the description belongs to the author, and the bot only ever
// rewrites what lies strictly between its own delimiters.
//
// The result is what should be published; the caller compares it with the
// original to decide whether an API write is warranted at all.
func SpliceRegion(description, body string) (string, error) {
	start, end, found, err := locateRegion(description)
	if err != nil {
		return "", err
	}

	if found {
		return description[:start] + body + description[end:], nil
	}

	return description + blankLineSeparator(description) + body, nil
}

// RemoveRegion returns description with the bot's Description Region removed,
// or unchanged if there is none. Used to clean up the destination that is no
// longer in effect, so a Change Request never carries a frozen report next to
// a live one (see docs/adr/0008-report-destination.md).
//
// The blank line that separated the region from the author's text is left
// behind rather than trimmed, since it sits outside the markers and is not the
// bot's to delete. It renders as nothing, and a later re-insertion reuses it
// instead of adding another.
func RemoveRegion(description string) (string, error) {
	start, end, found, err := locateRegion(description)
	if err != nil {
		return "", err
	}

	if !found {
		return description, nil
	}

	return description[:start] + description[end:], nil
}

// locateRegion returns the byte offsets delimiting the bot's region in
// description, markers included, and whether one is present. It reports
// ErrUnterminatedRegion when an opening marker has no closing one.
//
// The span extends past the newline following the closing marker when there
// is one, because Render emits it: it is the bot's own byte, not the author's.
// Counting it in is what makes an insert/remove cycle stable instead of
// leaving one more blank line behind every time.
func locateRegion(description string) (start, end int, found bool, err error) {
	start = strings.Index(description, Marker)
	if start < 0 {
		return 0, 0, false, nil
	}

	afterOpen := start + len(Marker)

	rel := strings.Index(description[afterOpen:], EndMarker)
	if rel < 0 {
		return 0, 0, false, ErrUnterminatedRegion
	}

	end = afterOpen + rel + len(EndMarker)
	if strings.HasPrefix(description[end:], "\n") {
		end++
	}

	return start, end, true, nil
}

// blankLineSeparator returns the newlines to insert between description and a
// region appended after it, so exactly one blank line separates them without
// ever removing a character the author wrote.
func blankLineSeparator(description string) string {
	switch {
	case description == "":
		return ""
	case strings.HasSuffix(description, "\n\n"):
		return ""
	case strings.HasSuffix(description, "\n"):
		return "\n"
	default:
		return "\n\n"
	}
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

// openAttr renders the <details> attribute expanding a section on arrival.
func openAttr(open bool) string {
	if open {
		return " open"
	}

	return ""
}

// writeGroup renders one group of a Section as a collapsible table. A blank
// line after the <summary> is required: without it neither Forge parses the
// Markdown table nested inside the HTML block.
func writeGroup(b *strings.Builder, title string, changes []dependencydiff.Change, open bool) {
	fmt.Fprintf(b, "\n<details%s>\n<summary>%s (%d)</summary>\n\n", openAttr(open), title, len(changes))

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
//
// It takes no Fold, and honoring one would be a mistake: a terminal has
// nothing to click, so folding content there would mean withholding it, which
// turns a presentation setting into a content filter and defeats the purpose
// of previewing a Change Request before opening it.
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
