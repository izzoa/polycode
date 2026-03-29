package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// diffStyles returns lipgloss styles for diff rendering based on the active theme.
func diffStyles(t Theme) (added, removed, header, context lipgloss.Style) {
	added = lipgloss.NewStyle().Foreground(t.DiffAdded)
	removed = lipgloss.NewStyle().Foreground(t.DiffRemoved)
	header = lipgloss.NewStyle().Foreground(t.DiffHeader).Bold(true)
	context = lipgloss.NewStyle().Foreground(t.DiffContext)
	return
}

// renderColorizedDiff takes a plain text diff (with +/- prefixes) and returns
// a colorized version using lipgloss styles.
func renderColorizedDiff(diff string, t Theme) string {
	if diff == "" {
		return ""
	}

	diffAddedStyle, diffRemovedStyle, diffHeaderStyle, diffContextStyle := diffStyles(t)

	lines := strings.Split(diff, "\n")
	var b strings.Builder

	for _, line := range lines {
		if line == "" {
			b.WriteString("\n")
			continue
		}

		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			b.WriteString(diffHeaderStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			b.WriteString(diffHeaderStyle.Render(line))
		case strings.HasPrefix(line, "+ ") || strings.HasPrefix(line, "+\t") || line == "+":
			b.WriteString(diffAddedStyle.Render(line))
		case strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "-\t") || line == "-":
			b.WriteString(diffRemovedStyle.Render(line))
		default:
			b.WriteString(diffContextStyle.Render(line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// isDiffContent returns true if the text appears to be a unified diff.
// Requires at least one strong marker (@@, +++, ---) plus additional +/- lines
// to avoid false positives on markdown lists.
func isDiffContent(text string) bool {
	lines := strings.SplitN(text, "\n", 30) // check first 30 lines
	hasStrongMarker := false
	changeLines := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") || strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			hasStrongMarker = true
		}
		if strings.HasPrefix(line, "+ ") || strings.HasPrefix(line, "- ") {
			changeLines++
		}
	}
	return hasStrongMarker && changeLines >= 1
}
