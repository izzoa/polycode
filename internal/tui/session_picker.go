package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/izzoa/polycode/internal/config"
)

// SessionPickerMsg delivers session list data for the picker overlay.
type SessionPickerMsg struct {
	Sessions []config.SessionInfo
	Error    error
}

// sessionSource adapts a SessionInfo slice for fuzzy matching.
type sessionSource []config.SessionInfo

func (s sessionSource) Len() int           { return len(s) }
func (s sessionSource) String(i int) string { return s[i].Name }

// renderSessionPicker renders the session browser overlay.
func (m Model) renderSessionPicker() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Tertiary)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Info).Background(m.theme.BgSelected)
	normalStyle := lipgloss.NewStyle().Foreground(m.theme.TextBright)
	dimStyle := lipgloss.NewStyle().Foreground(m.theme.TextHint)
	currentStyle := lipgloss.NewStyle().Foreground(m.theme.Primary)
	filterStyle := lipgloss.NewStyle().Foreground(m.theme.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(m.theme.TextHint)

	var lines []string
	lines = append(lines, titleStyle.Render("Sessions")+" "+lipgloss.NewStyle().Foreground(m.theme.BorderNormal).Render(strings.Repeat("─", 40)))
	lines = append(lines, "")

	// Filter
	if m.sessionPickerFilter != "" {
		lines = append(lines, "  / "+filterStyle.Render(m.sessionPickerFilter))
		lines = append(lines, "")
	}

	sessions := m.filteredSessions()

	if len(sessions) == 0 {
		if len(m.sessionPickerData) == 0 {
			lines = append(lines, "  "+dimStyle.Render("No saved sessions."))
		} else {
			lines = append(lines, "  "+dimStyle.Render("No matching sessions."))
		}
	} else {
		for i, s := range sessions {
			style := normalStyle
			cursor := "  "
			if i == m.sessionPickerCursor {
				style = selectedStyle
				cursor = "> "
			}

			name := s.Name
			if s.IsCurrent {
				name += " " + currentStyle.Render("(current)")
			}

			// Relative time
			ago := relativeTime(s.UpdatedAt)

			// Format line
			info := dimStyle.Render(fmt.Sprintf("%d exchanges  %s", s.Exchanges, ago))
			line := cursor + style.Render(name)

			// Pad name to align info
			padding := 40 - lipgloss.Width(name)
			if padding < 2 {
				padding = 2
			}
			line += strings.Repeat(" ", padding) + info
			lines = append(lines, line)
		}
	}

	lines = append(lines, "")
	if m.sessionPickerRenaming {
		lines = append(lines, "  "+hintStyle.Render("Type new name, Enter to confirm, Esc to cancel"))
	} else {
		lines = append(lines, "  "+hintStyle.Render("enter:open  d:delete  r:rename  /:filter  Esc:close"))
	}

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.BorderFocused).
		Padding(0, 1).
		Width(m.width - 4)
	return addDropShadow(border.Render(strings.Join(lines, "\n")), m.theme)
}

// filteredSessions returns sessions filtered by the current search term.
func (m Model) filteredSessions() []config.SessionInfo {
	if m.sessionPickerFilter == "" {
		return m.sessionPickerData
	}
	results := fuzzy.FindFrom(m.sessionPickerFilter, sessionSource(m.sessionPickerData))
	filtered := make([]config.SessionInfo, len(results))
	for i, r := range results {
		filtered[i] = m.sessionPickerData[r.Index]
	}
	return filtered
}

// renderSessionPickerFull renders the session picker as a full-screen overlay.
func (m Model) renderSessionPickerFull() string {
	var sections []string

	sections = append(sections, m.renderSessionPicker())

	// Spacer to fill remaining height
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	contentLines := strings.Count(content, "\n") + 1
	remaining := m.height - contentLines - 2
	if remaining > 0 {
		content += strings.Repeat("\n", remaining)
	}

	return m.styles.App.Width(m.width).Render(content)
}

// relativeTime formats a time as a human-readable relative string.
func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}
