package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const splitMinWidth = 140

// splitPaneActive returns true if the terminal is wide enough for split pane.
func (m Model) splitPaneActive() bool {
	return m.width >= splitMinWidth && m.splitPaneEnabled && len(m.panels) > 0
}

// renderSplitPane renders the chat view with a side panel.
func (m Model) renderSplitPane() string {
	leftWidth := int(float64(m.width-2) * float64(m.splitRatio) / 100.0)
	rightWidth := m.width - 2 - leftWidth - 1 // 1 for divider

	if leftWidth < 20 {
		leftWidth = 20
	}
	if rightWidth < 20 {
		rightWidth = 20
	}

	// Left panel: chat/consensus (reuse existing rendering)
	leftStyle := lipgloss.NewStyle().Width(leftWidth).MaxHeight(m.chatView.Height + 2)
	leftContent := m.chatView.View()
	left := leftStyle.Render(leftContent)

	// Divider
	divStyle := lipgloss.NewStyle().Foreground(m.theme.ScrollTrack)
	divider := divStyle.Render(strings.Repeat("│\n", m.chatView.Height+1))

	// Right panel: selected provider or provider at index splitPanelIdx
	var rightContent string
	idx := m.splitPanelIdx
	if idx >= 0 && idx < len(m.panels) {
		panel := m.panels[idx]
		header := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Tertiary).Render(panel.Name)
		rightContent = header + "\n" + panel.Viewport.View()
	} else {
		rightContent = lipgloss.NewStyle().Foreground(m.theme.TextMuted).Render("Press 1-9 to show provider panel")
	}
	rightStyle := lipgloss.NewStyle().Width(rightWidth).MaxHeight(m.chatView.Height + 2)
	right := rightStyle.Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, divider, right)
}
