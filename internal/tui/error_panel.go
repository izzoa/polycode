package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ErrorRecord holds structured error information for display.
type ErrorRecord struct {
	Summary   string    // one-line summary
	Detail    string    // full error message
	Timestamp time.Time // when the error occurred
	Collapsed bool      // whether the detail is hidden
}

// renderErrorPanel renders a structured error panel.
func renderErrorPanel(err ErrorRecord, width int, t Theme) string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Error).
		Padding(0, 1).
		Width(width - 4)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Error)

	hintStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	timeStyle := lipgloss.NewStyle().
		Foreground(t.ScrollTrack)

	header := headerStyle.Render("✕ Error: " + err.Summary)

	var content string
	if err.Collapsed {
		content = fmt.Sprintf("%s  %s\n%s",
			header,
			timeStyle.Render(err.Timestamp.Format("15:04:05")),
			hintStyle.Render("  e: expand  r: retry  c: copy error"),
		)
	} else {
		content = fmt.Sprintf("%s  %s\n\n%s\n\n%s",
			header,
			timeStyle.Render(err.Timestamp.Format("15:04:05")),
			err.Detail,
			hintStyle.Render("  e: collapse  r: retry  c: copy error"),
		)
	}

	return borderStyle.Render(content)
}
