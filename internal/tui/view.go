package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Splash screen takes over the entire view
	if m.showSplash {
		return m.renderSplash()
	}

	var sections []string

	// Status bar (always visible)
	sections = append(sections, m.renderStatusBar())

	// Main chat conversation log (always visible)
	chatContent := m.buildChatLog()
	if m.querying && m.consensusContent.Len() > 0 {
		// Show streaming consensus inline in the chat during query
		chatContent += m.consensusContent.String()
	}
	m.chatView.SetContent(chatContent)
	if len(m.history) > 0 || m.querying {
		sections = append(sections, m.renderChatPanel())
	}

	// Provider panels (only during active query, toggled with Tab)
	if m.querying && m.showIndividual && m.hasContent() {
		sections = append(sections, m.renderProviderPanels())
	}

	// Input area (always visible)
	sections = append(sections, m.renderInput())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderStatusBar() string {
	var parts []string
	parts = append(parts, m.styles.Title.Render("polycode"))
	parts = append(parts, m.styles.Dimmed.Render(" | "))

	for i, panel := range m.panels {
		var indicator string
		switch panel.Status {
		case StatusIdle:
			indicator = m.styles.Dimmed.Render("○")
		case StatusLoading:
			indicator = m.spinner.View()
		case StatusDone:
			indicator = m.styles.StatusHealthy.Render("●")
		case StatusFailed:
			indicator = m.styles.StatusUnhealthy.Render("✕")
		case StatusTimedOut:
			indicator = m.styles.StatusUnhealthy.Render("⏱")
		}

		name := panel.Name
		if panel.IsPrimary {
			name = m.styles.StatusPrimary.Render(name + "★")
		}

		// Token usage display
		usageStr := ""
		if td, ok := m.tokenUsage[panel.Name]; ok && td.HasData {
			if td.Limit != "" {
				usageStr = fmt.Sprintf(" %s/%s", td.Used, td.Limit)
			} else {
				usageStr = fmt.Sprintf(" %s", td.Used)
			}
			// Color code based on percentage
			switch {
			case td.Percent >= 95:
				usageStr = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(usageStr) // red
			case td.Percent >= 80:
				usageStr = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(usageStr) // amber
			default:
				usageStr = m.styles.Dimmed.Render(usageStr)
			}
		}

		parts = append(parts, fmt.Sprintf("%s %s%s", indicator, name, usageStr))
		if i < len(m.panels)-1 {
			parts = append(parts, m.styles.Dimmed.Render("  "))
		}
	}

	if m.querying {
		parts = append(parts, m.styles.Dimmed.Render("  │  "))
		parts = append(parts, m.spinner.View()+" querying...")
	}

	bar := strings.Join(parts, "")
	return m.styles.StatusBar.Width(m.width).Render(bar)
}

func (m Model) renderChatPanel() string {
	style := m.styles.ConsensusBorder.Width(m.width - 4)
	return style.Render(m.chatView.View())
}

func (m Model) renderProviderPanels() string {
	var panels []string

	for _, panel := range m.panels {
		statusIcon := ""
		switch panel.Status {
		case StatusLoading:
			statusIcon = m.spinner.View() + " "
		case StatusDone:
			statusIcon = m.styles.StatusHealthy.Render("✓ ")
		case StatusFailed:
			statusIcon = m.styles.StatusUnhealthy.Render("✕ ")
		case StatusTimedOut:
			statusIcon = m.styles.StatusUnhealthy.Render("⏱ ")
		}

		title := fmt.Sprintf("%s%s", statusIcon, panel.Name)
		if panel.IsPrimary {
			title += m.styles.StatusPrimary.Render(" ★")
		}

		content := panel.Viewport.View()
		if content == "" && panel.Status == StatusLoading {
			content = m.styles.Dimmed.Render("waiting for response...")
		}

		style := m.styles.PanelBorder.Width(m.width - 4)
		rendered := style.Render(fmt.Sprintf("%s\n%s", title, content))
		panels = append(panels, rendered)
	}

	return lipgloss.JoinVertical(lipgloss.Left, panels...)
}

func (m Model) renderInput() string {
	label := m.styles.Prompt.Render("❯ ")
	input := m.textarea.View()

	style := m.styles.InputBorder.Width(m.width - 4)
	return style.Render(fmt.Sprintf("%s\n%s", label, input))
}

func (m Model) hasContent() bool {
	for _, p := range m.panels {
		if p.Content.Len() > 0 {
			return true
		}
	}
	return false
}
