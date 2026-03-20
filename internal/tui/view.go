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

	// Tab bar — combines app title, mode, provider tabs, and query status
	sections = append(sections, m.renderTabBar())

	// Main content area — depends on active tab
	if m.activeTab <= 0 {
		// Consensus tab (or mode selector — show consensus content)
		chatContent := m.buildChatLog()
		if m.querying {
			if m.consensusRendered != "" {
				chatContent += m.consensusRendered
			} else if m.consensusContent.Len() > 0 {
				// First chunks before initial render throttle fires
				chatContent += m.consensusContent.String()
			} else {
				chatContent += "\n" + m.spinner.View() + " Thinking..."
			}
		}
		if m.lastError != "" && !m.querying {
			errStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)
			chatContent += "\n\n" + errStyle.Render("Error: "+m.lastError)
		}
		m.chatView.SetContent(chatContent)
		// Only auto-scroll to bottom during active queries (not when user is reading)
		if m.querying {
			m.chatView.GotoBottom()
		}
		if len(m.history) > 0 || m.querying {
			sections = append(sections, m.renderChatPanel())
		}
	} else if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
		// Provider tab: show that provider's full response
		panel := m.panels[m.activeTab-1]
		sections = append(sections, m.renderSingleProviderPanel(panel))
	}

	// Worker progress (during /plan execution)
	if m.planRunning && len(m.agentStages) > 0 {
		sections = append(sections, m.renderWorkerProgress())
	}

	// Provenance panel (toggled with 'p')
	if m.showProvenance && !m.querying && m.activeTab == 0 {
		sections = append(sections, m.renderProvenance())
	}

	// Slash command completion hints (shown above input when typing /)
	if len(m.slashMatches) > 0 && !m.querying {
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
		selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
		var hints []string
		for i, cmd := range m.slashMatches {
			if i == m.slashCompIdx {
				hints = append(hints, selectedStyle.Render(cmd))
			} else {
				hints = append(hints, hintStyle.Render(cmd))
			}
		}
		sections = append(sections, "  "+strings.Join(hints, hintStyle.Render("  ")))
	}

	// Mode picker overlay
	if m.modePickerOpen {
		sections = append(sections, m.renderModePicker())
	}

	// Confirmation prompt (overlays input area when pending)
	if m.confirmPending {
		sections = append(sections, m.renderConfirmPrompt())
	} else if !m.modePickerOpen {
		// Input area (always visible when not confirming or picking mode)
		sections = append(sections, m.renderInput())
	}

	output := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Constrain output to terminal height to prevent overflow
	if m.height > 0 {
		output = lipgloss.NewStyle().MaxHeight(m.height).Render(output)
	}

	return output
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
		{"/save", "Save session to disk"},
		{"/export [path]", "Export session as shareable artifact"},
		{"/plan <request>", "Run multi-model agent team pipeline"},
		{"/mode <name>", "Switch mode: quick, balanced, thorough, yolo"},
		{"/yolo", "Toggle yolo mode (auto-approve all tool actions)"},
		{"/memory", "View repo memory"},
		{"/skill [list|install|remove]", "Manage installed skills"},
		{"/help", "Toggle this help overlay"},
		{"/exit", "Quit polycode"},
		{"Tab", "Accept slash completion"},
		{"↑ (input empty)", "Focus tab bar for ←/→ navigation"},
		{"↓ / Enter / Esc", "Return focus to input"},
		{"p", "Toggle provenance (when input empty)"},
		{"?", "Toggle help (when input empty)"},
		{"Enter", "Submit prompt"},
		{"PgUp / PgDn", "Scroll chat history"},
		{"Ctrl+U / Ctrl+D", "Half-page scroll"},
		{"Home / End", "Jump to top / bottom of chat"},
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


func (m Model) renderChatPanel() string {
	style := m.styles.ConsensusBorder.Width(m.width - 4)
	return style.Render(m.chatView.View())
}

// renderTabBar renders the combined status + tab bar.
func (m Model) renderTabBar() string {
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Background(lipgloss.Color("236")).
		Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Background(lipgloss.Color("235")).
		Padding(0, 1)

	// When tab bar is focused, use a brighter active style with underline
	if m.tabBarFocused {
		activeStyle = activeStyle.
			Foreground(lipgloss.Color("82")).
			Background(lipgloss.Color("238")).
			Underline(true)
	}

	// App title + mode + focus indicator
	var header []string
	if m.tabBarFocused {
		header = append(header, m.styles.Title.Render("▸ polycode"))
	} else {
		header = append(header, m.styles.Title.Render("polycode"))
	}
	if m.currentMode != "" {
		modeLabel := m.currentMode
		if m.yoloMode {
			modeLabel += "|yolo"
		}
		if m.tabBarFocused && m.activeTab == -1 {
			// Mode selector is highlighted
			header = append(header, activeStyle.Render(modeLabel))
		} else {
			modeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
			header = append(header, m.styles.Dimmed.Render("["))
			header = append(header, modeStyle.Render(modeLabel))
			header = append(header, m.styles.Dimmed.Render("] "))
		}
	} else {
		header = append(header, " ")
	}

	// Consensus tab
	consensusLabel := "Consensus"
	if m.querying {
		if m.consensusActive {
			consensusLabel = m.spinner.View() + " Synthesizing"
		} else {
			consensusLabel = m.spinner.View() + " Querying"
		}
	}
	if m.activeTab == 0 {
		header = append(header, activeStyle.Render(consensusLabel))
	} else {
		header = append(header, inactiveStyle.Render(consensusLabel))
	}

	// Provider tabs with status + token usage
	for i, panel := range m.panels {
		var icon string
		switch panel.Status {
		case StatusIdle:
			icon = "○"
		case StatusLoading:
			icon = m.spinner.View()
		case StatusDone:
			icon = "✓"
		case StatusFailed:
			icon = "✕"
		case StatusTimedOut:
			icon = "⏱"
		}

		label := icon + " " + panel.Name
		if panel.IsPrimary {
			label += "★"
		}

		// Compact token usage
		if td, ok := m.tokenUsage[panel.Name]; ok && td.HasData {
			label += " " + td.Used
		}

		if m.activeTab == i+1 {
			header = append(header, activeStyle.Render(label))
		} else {
			header = append(header, inactiveStyle.Render(label))
		}
	}

	bar := strings.Join(header, "")
	return m.styles.StatusBar.Width(m.width).Render(bar)
}

// renderSingleProviderPanel renders one provider's response as a full panel.
func (m Model) renderSingleProviderPanel(panel ProviderPanel) string {
	content := panel.Viewport.View()
	if content == "" {
		switch panel.Status {
		case StatusLoading:
			content = m.spinner.View() + " Waiting for response..."
		case StatusIdle:
			content = m.styles.Dimmed.Render("No response yet")
		case StatusDone:
			content = m.styles.Dimmed.Render("Model responded with tool calls only (no text output).\nTool execution is handled by the consensus orchestrator.")
		case StatusFailed:
			content = m.styles.StatusUnhealthy.Render("Provider failed")
		}
	}

	style := m.styles.ConsensusBorder.Width(m.width - 4)
	return style.Render(content)
}

func (m Model) renderModePicker() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82")).Background(lipgloss.Color("236"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	yoloOnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))

	var lines []string
	lines = append(lines, titleStyle.Render("Select Mode"))
	lines = append(lines, "")

	descriptions := map[string]string{
		"quick":    "Single best provider, no consensus",
		"balanced": "All providers + consensus synthesis",
		"thorough": "Extended context, deeper analysis",
	}

	// Mode items (0..2) + yolo toggle (3)
	totalItems := len(m.modePickerItems) + 1 // +1 for yolo toggle

	for i, item := range m.modePickerItems {
		cursor := "  "
		style := normalStyle
		if i == m.modePickerIdx {
			cursor = "▸ "
			style = selectedStyle
		}
		current := ""
		if item == m.currentMode {
			current = " (current)"
		}
		line := cursor + style.Render(item+current)
		if desc, ok := descriptions[item]; ok {
			line += "  " + hintStyle.Render(desc)
		}
		lines = append(lines, line)
	}

	// Yolo toggle as last item
	lines = append(lines, "")
	yoloIdx := totalItems - 1
	cursor := "  "
	style := normalStyle
	if m.modePickerIdx == yoloIdx {
		cursor = "▸ "
		style = selectedStyle
	}
	checkbox := "[ ]"
	if m.yoloMode {
		checkbox = yoloOnStyle.Render("[✓]")
	}
	lines = append(lines, cursor+checkbox+" "+style.Render("yolo")+"  "+hintStyle.Render("Auto-approve all tool actions"))

	lines = append(lines, "")
	lines = append(lines, hintStyle.Render("  ↑/↓ navigate  Enter select  Esc cancel"))

	border := m.styles.InputBorder.Width(m.width - 4)
	return border.Render(strings.Join(lines, "\n"))
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
