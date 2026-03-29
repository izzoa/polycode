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

	// Title — show focus indicator for active section
	providerTitleText := "Providers"
	if !m.mcpSettingsFocused {
		providerTitleText = "▸ Providers"
	}
	title := m.styles.Title.Render(providerTitleText)
	sections = append(sections, title)
	sections = append(sections, "")

	if m.cfg == nil || len(m.cfg.Providers) == 0 {
		// Empty state
		empty := lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Italic(true).
			Render("No providers configured — press 'a' to add one")
		sections = append(sections, empty)
	} else {
		// Table header
		header := fmt.Sprintf("  %-20s %-20s %-30s %-10s %-10s %-8s",
			"NAME", "TYPE", "MODEL", "AUTH", "STATUS", "PRIMARY")
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(m.theme.Text).
			Background(m.theme.BgPanel)
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
			isSelected := !m.mcpSettingsFocused && i == m.settingsCursor
			if isSelected {
				cursor = m.styles.Prompt.Render("> ")
			}

			status := m.styles.StatusHealthy.Render("enabled")
			if p.Disabled {
				status = m.styles.StatusUnhealthy.Render("disabled")
			}

			row := fmt.Sprintf("%s%-20s %-20s %-30s %-10s %-10s %-8s",
				cursor,
				p.Name,
				string(p.Type),
				truncate(p.Model, 28),
				authDisplay,
				status,
				primary,
			)

			rowStyle := lipgloss.NewStyle()
			if isSelected {
				rowStyle = rowStyle.
					Background(m.theme.BgSelected).
					Foreground(m.theme.Text)
			}
			sections = append(sections, rowStyle.Width(m.width-4).Render(row))
		}
	}

	// MCP Servers section
	sections = append(sections, "")
	mcpTitleText := "MCP Servers"
	if m.mcpSettingsFocused {
		mcpTitleText = "▸ MCP Servers"
	}
	mcpTitle := m.styles.Title.Render(mcpTitleText)
	sections = append(sections, mcpTitle)

	mcpServerCount := 0
	if m.cfg != nil {
		mcpServerCount = len(m.cfg.MCP.Servers)
	}

	if mcpServerCount == 0 && len(m.mcpServers) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Italic(true).
			Render("No MCP servers configured")
		sections = append(sections, empty)
	} else {
		// MCP table header
		mcpHeader := fmt.Sprintf("  %-20s %-12s %-14s %-8s",
			"NAME", "TRANSPORT", "STATUS", "TOOLS")
		mcpHeaderStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(m.theme.Text).
			Background(m.theme.BgPanel)
		sections = append(sections, mcpHeaderStyle.Width(m.width-4).Render(mcpHeader))

		// Build status map from MCPStatusMsg
		statusMap := make(map[string]MCPServerStatus)
		for _, s := range m.mcpServers {
			statusMap[s.Name] = s
		}

		for i := 0; i < mcpServerCount; i++ {
			srv := m.cfg.MCP.Servers[i]
			transport := "stdio"
			if srv.URL != "" {
				transport = "sse"
			}

			status := "disconnected"
			toolCount := "—"
			statusStyle := lipgloss.NewStyle().Foreground(m.theme.TextMuted)

			if s, ok := statusMap[srv.Name]; ok {
				status = s.Status
				if s.ToolCount > 0 {
					toolCount = fmt.Sprintf("%d", s.ToolCount)
				}
				switch s.Status {
				case "connected":
					statusStyle = lipgloss.NewStyle().Foreground(m.theme.Success)
				case "failed":
					statusStyle = lipgloss.NewStyle().Foreground(m.theme.Error)
				}
			}

			cursor := "  "
			if m.mcpSettingsFocused && i == m.mcpSettingsCursor {
				cursor = m.styles.Prompt.Render("> ")
			}

			row := fmt.Sprintf("%s%-20s %-12s %s %-8s",
				cursor,
				srv.Name,
				transport,
				statusStyle.Render(fmt.Sprintf("%-14s", status)),
				toolCount,
			)

			rowStyle := lipgloss.NewStyle()
			if m.mcpSettingsFocused && i == m.mcpSettingsCursor {
				rowStyle = rowStyle.
					Background(m.theme.BgSelected).
					Foreground(m.theme.Text)
			}
			sections = append(sections, rowStyle.Width(m.width-4).Render(row))
		}
	}

	// Status message (transient)
	if m.settingsMsg != "" {
		sections = append(sections, "")
		sections = append(sections, m.settingsMsg)
	}

	// Delete confirmation prompts
	if m.confirmDelete && !m.mcpSettingsFocused && m.cfg != nil && m.settingsCursor < len(m.cfg.Providers) {
		sections = append(sections, "")
		name := m.cfg.Providers[m.settingsCursor].Name
		warnStyle := lipgloss.NewStyle().
			Foreground(m.theme.Error).
			Bold(true)
		sections = append(sections, warnStyle.Render(
			fmt.Sprintf("Remove provider '%s'? (y/n)", name)))
	}
	if m.mcpConfirmDelete && m.mcpSettingsFocused && m.cfg != nil && m.mcpSettingsCursor < len(m.cfg.MCP.Servers) {
		sections = append(sections, "")
		name := m.cfg.MCP.Servers[m.mcpSettingsCursor].Name
		warnStyle := lipgloss.NewStyle().
			Foreground(m.theme.Error).
			Bold(true)
		sections = append(sections, warnStyle.Render(
			fmt.Sprintf("Remove MCP server '%s'? (y/n)", name)))
	}

	// Testing spinner
	if m.testingProvider != "" {
		sections = append(sections, "")
		sections = append(sections, m.spinner.View()+" Testing "+m.testingProvider+"...")
	}
	if m.mcpTestingServer != "" {
		sections = append(sections, "")
		sections = append(sections, m.spinner.View()+" Testing MCP server "+m.mcpTestingServer+"...")
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
		Foreground(m.theme.TextMuted)
	focusHint := ""
	if m.mcpSettingsFocused {
		focusHint = " (MCP)"
	}
	hints := hintStyle.Render(fmt.Sprintf("a:add  e:edit  d:delete  x:disable/enable  t:test  Tab:switch%s  Esc:back", focusHint))
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
