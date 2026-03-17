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

	// Help overlay takes priority
	if m.showHelp {
		return m.renderHelp()
	}

	// Dispatch based on current view mode
	switch m.mode {
	case viewSettings:
		return m.renderSettings()
	case viewAddProvider, viewEditProvider:
		return m.renderWizard()
	default:
		return m.renderChat()
	}
}

// renderChat renders the main chat view.
func (m Model) renderChat() string {
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

	// Worker progress (during /plan execution)
	if m.planRunning && len(m.agentStages) > 0 {
		sections = append(sections, m.renderWorkerProgress())
	} else if m.querying && m.showIndividual && m.hasContent() {
		// Provider panels (only during active query, toggled with Tab)
		sections = append(sections, m.renderProviderPanels())
	}

	// Provenance panel (toggled with 'p')
	if m.showProvenance && !m.querying {
		sections = append(sections, m.renderProvenance())
	}

	// Confirmation prompt (overlays input area when pending)
	if m.confirmPending {
		sections = append(sections, m.renderConfirmPrompt())
	} else {
		// Input area (always visible when not confirming)
		sections = append(sections, m.renderInput())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderConfirmPrompt renders the action confirmation prompt.
func (m Model) renderConfirmPrompt() string {
	warnStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(0, 1).
		Width(m.width - 4)

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("Confirm Action")
	desc := m.confirmDescription
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  y: approve  n: reject  Esc: cancel")

	return warnStyle.Render(fmt.Sprintf("%s\n\n%s\n\n%s", title, desc, hint))
}

// renderProvenance renders the consensus provenance panel.
func (m Model) renderProvenance() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		Width(m.width - 4)

	var lines []string

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).Render("Consensus Provenance")
	lines = append(lines, title, "")

	// Confidence
	if m.consensusConfidence != "" {
		var confStyle lipgloss.Style
		switch m.consensusConfidence {
		case "high":
			confStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
		case "medium":
			confStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
		case "low":
			confStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		default:
			confStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		}
		lines = append(lines, "Confidence: "+confStyle.Render(m.consensusConfidence))
	}

	// Agreements
	if len(m.consensusAgreements) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("Agreement:"))
		for _, a := range m.consensusAgreements {
			lines = append(lines, "  "+a)
		}
	}

	// Minority reports
	if len(m.minorityReports) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("Minority Report:"))
		for _, mr := range m.minorityReports {
			lines = append(lines, "  "+mr)
		}
	} else {
		lines = append(lines, "", m.styles.Dimmed.Render("All models agreed"))
	}

	// Evidence
	if len(m.consensusEvidence) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("Evidence:"))
		for _, e := range m.consensusEvidence {
			lines = append(lines, "  "+e)
		}
	}

	lines = append(lines, "", m.styles.Dimmed.Render("Press p to close"))

	return style.Render(strings.Join(lines, "\n"))
}

// renderWorkerProgress renders the agent team progress panel during /plan.
func (m Model) renderWorkerProgress() string {
	style := m.styles.ConsensusBorder.Width(m.width - 4)

	var lines []string
	title := m.styles.Title.Render("Agent Team")
	lines = append(lines, title)

	for _, stage := range m.agentStages {
		for _, w := range stage.Workers {
			var icon string
			switch w.Status {
			case "complete":
				icon = m.styles.StatusHealthy.Render("✓")
			case "running":
				icon = m.spinner.View()
			default:
				icon = m.styles.Dimmed.Render("○")
			}

			line := fmt.Sprintf("  %s %s (%s)", icon, w.Role, w.Provider)
			if w.Summary != "" {
				summary := w.Summary
				if len(summary) > 60 {
					summary = summary[:57] + "..."
				}
				line += " — " + m.styles.Dimmed.Render(summary)
			}
			lines = append(lines, line)
		}
	}

	return style.Render(strings.Join(lines, "\n"))
}

// renderHelp renders the help overlay showing all keyboard shortcuts.
func (m Model) renderHelp() string {
	var sections []string

	title := m.styles.Title.Render("Keyboard Shortcuts")
	sections = append(sections, title)
	sections = append(sections, "")

	labelStyle := lipgloss.NewStyle().Bold(true).Width(16)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	helpItems := []struct{ key, desc string }{
		{"Ctrl+C", "Quit"},
		{"Ctrl+S", "Open settings"},
		{"/settings", "Open settings (type in input)"},
		{"/clear", "Clear conversation and reset context"},
		{"/plan <request>", "Run multi-model agent team pipeline"},
		{"Tab", "Toggle individual provider panels"},
		{"p", "Toggle consensus provenance panel"},
		{"Enter", "Submit prompt / advance wizard step"},
		{"?", "Toggle this help overlay"},
		{"", ""},
		{"", "Settings Screen"},
		{"j / Down", "Move cursor down"},
		{"k / Up", "Move cursor up"},
		{"a", "Add new provider"},
		{"e", "Edit selected provider"},
		{"d", "Delete selected provider"},
		{"t", "Test selected provider connection"},
		{"Esc", "Return to chat / cancel"},
	}

	for _, item := range helpItems {
		if item.key == "" && item.desc == "" {
			sections = append(sections, "")
			continue
		}
		if item.key == "" {
			sections = append(sections, m.styles.Title.Render(item.desc))
			continue
		}
		sections = append(sections, labelStyle.Render(item.key)+"  "+valueStyle.Render(item.desc))
	}

	sections = append(sections, "")
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	sections = append(sections, hintStyle.Render("Press ? or Esc to close"))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Center vertically
	contentLines := strings.Count(content, "\n") + 1
	topPad := (m.height - contentLines) / 2
	if topPad < 0 {
		topPad = 0
	}

	return lipgloss.NewStyle().
		Width(m.width).
		PaddingTop(topPad).
		PaddingLeft(4).
		Render(content)
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
