package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderSettings renders the settings list screen showing all configured
// providers in a table format with action hints at the bottom.
func (m Model) renderSettings() string {
	var sections []string

	// Title
	title := m.styles.Title.Render("Settings — Provider Management")
	sections = append(sections, title)
	sections = append(sections, "")

	if m.cfg == nil || len(m.cfg.Providers) == 0 {
		// Empty state
		empty := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Render("No providers configured — press 'a' to add one")
		sections = append(sections, empty)
	} else {
		// Table header
		header := fmt.Sprintf("  %-20s %-20s %-30s %-10s %-8s",
			"NAME", "TYPE", "MODEL", "AUTH", "PRIMARY")
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("235"))
		sections = append(sections, headerStyle.Width(m.width-4).Render(header))

		// Provider rows
		for i, p := range m.cfg.Providers {
			primary := ""
			if p.Primary {
				primary = m.styles.StatusPrimary.Render("★")
			}

			authDisplay := string(p.Auth)
			if p.Auth == "api_key" {
				authDisplay = "configured"
			}

			cursor := "  "
			if i == m.settingsCursor {
				cursor = m.styles.Prompt.Render("> ")
			}

			row := fmt.Sprintf("%s%-20s %-20s %-30s %-10s %-8s",
				cursor,
				p.Name,
				string(p.Type),
				truncate(p.Model, 28),
				authDisplay,
				primary,
			)

			rowStyle := lipgloss.NewStyle()
			if i == m.settingsCursor {
				rowStyle = rowStyle.
					Background(lipgloss.Color("236")).
					Foreground(lipgloss.Color("252"))
			}
			sections = append(sections, rowStyle.Width(m.width-4).Render(row))
		}
	}

	// Status message (transient)
	if m.settingsMsg != "" {
		sections = append(sections, "")
		sections = append(sections, m.settingsMsg)
	}

	// Delete confirmation prompt
	if m.confirmDelete && m.cfg != nil && m.settingsCursor < len(m.cfg.Providers) {
		sections = append(sections, "")
		name := m.cfg.Providers[m.settingsCursor].Name
		warnStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		sections = append(sections, warnStyle.Render(
			fmt.Sprintf("Remove provider '%s'? (y/n)", name)))
	}

	// Testing spinner
	if m.testingProvider != "" {
		sections = append(sections, "")
		sections = append(sections, m.spinner.View()+" Testing "+m.testingProvider+"...")
	}

	// Spacer
	contentLines := len(sections)
	// Leave room for hints at bottom
	remaining := m.height - contentLines - 4
	if remaining > 0 {
		sections = append(sections, strings.Repeat("\n", remaining))
	}

	// Action hints
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	hints := hintStyle.Render("a:add  e:edit  d:delete  t:test  Esc:back")
	sections = append(sections, hints)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return m.styles.App.Width(m.width).Render(content)
}

// truncate shortens a string to maxLen, appending "..." if it was truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
