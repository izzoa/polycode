package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderStatusBar renders the persistent bottom status bar showing runtime state.
func (m Model) renderStatusBar() string {
	barStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted).
		Background(m.theme.BgPanel).
		Width(m.width)

	sepStyle := lipgloss.NewStyle().
		Foreground(m.theme.BgFocused)
	sep := sepStyle.Render(" │ ")

	var left []string
	var right []string

	// Session name (if available)
	if m.sessionName != "" {
		nameStyle := lipgloss.NewStyle().Foreground(m.theme.TextBright)
		left = append(left, nameStyle.Render(m.sessionName))
	}

	// Mode
	modeStyle := lipgloss.NewStyle().Foreground(m.theme.Secondary)
	modeLabel := m.currentMode
	if m.yoloMode {
		modeLabel += "|yolo"
	}
	left = append(left, modeStyle.Render(modeLabel))

	// Query status
	if m.querying {
		queryStyle := lipgloss.NewStyle().Foreground(m.theme.Primary)
		left = append(left, queryStyle.Render("querying..."))
	}

	// Tool conceal indicator
	if m.concealTools {
		left = append(left, lipgloss.NewStyle().Foreground(m.theme.TextMuted).Render("tools:hidden"))
	}

	// Aggregate token usage and cost from all providers
	var totalUsed, totalLimit int
	var totalCost float64
	var maxPercent float64
	var primaryModel string
	for _, panel := range m.panels {
		if td, ok := m.tokenUsage[panel.Name]; ok && td.HasData {
			// Parse numeric values from formatted strings for aggregation
			totalCost += parseCost(td.Cost)
			if td.Percent > maxPercent {
				maxPercent = td.Percent
			}
		}
		if panel.IsPrimary {
			primaryModel = panel.Name
		}
	}
	// Use the raw ProviderUsage data if available for accurate totals
	for _, td := range m.tokenUsage {
		if td.HasData {
			totalUsed += parseTokenCount(td.Used)
			totalLimit += parseTokenCount(td.Limit)
		}
	}

	// When a provider tab is selected, show that provider's context first
	activeProviderName := ""
	if m.activeTab > 0 && m.activeTab-1 < len(m.panels) {
		activeProviderName = m.panels[m.activeTab-1].Name
	}

	if activeProviderName != "" {
		if td, ok := m.tokenUsage[activeProviderName]; ok && td.HasData {
			// Show active provider's context
			provLabel := activeProviderName + ":"
			if td.Percent > 0 {
				ctxStyle := contextPercentStyle(td.Percent, m.theme)
				provLabel += ctxStyle.Render(fmt.Sprintf("%.0f%%", td.Percent))
			}
			tokenStr := td.Used
			if td.Limit != "" {
				tokenStr += "/" + td.Limit
			}
			provLabel += " " + tokenStr
			if td.Cost != "" {
				provLabel += " " + td.Cost
			}
			right = append(right, provLabel)
		}
		// Separator before primary context
		if primaryModel != "" && primaryModel != activeProviderName {
			right = append(right, lipgloss.NewStyle().Foreground(m.theme.TextMuted).Render("|"))
		}
	}

	// Primary/consensus context (always shown)
	if primaryModel != "" {
		var primaryParts []string
		if td, ok := m.tokenUsage[primaryModel]; ok && td.HasData {
			if td.Percent > 0 {
				ctxStyle := contextPercentStyle(td.Percent, m.theme)
				primaryParts = append(primaryParts, ctxStyle.Render(fmt.Sprintf("ctx:%.0f%%", td.Percent)))
			}
			tokenStr := td.Used
			if td.Limit != "" {
				tokenStr += "/" + td.Limit
			}
			primaryParts = append(primaryParts, tokenStr)
			if td.Cost != "" {
				primaryParts = append(primaryParts, td.Cost)
			}
		} else if maxPercent > 0 {
			// Fallback to aggregated stats
			ctxStyle := contextPercentStyle(maxPercent, m.theme)
			primaryParts = append(primaryParts, ctxStyle.Render(fmt.Sprintf("ctx:%.0f%%", maxPercent)))
			if totalUsed > 0 {
				tokenStr := formatCompactTokens(totalUsed)
				if totalLimit > 0 {
					tokenStr += "/" + formatCompactTokens(totalLimit)
				}
				primaryParts = append(primaryParts, tokenStr)
			}
		}
		if len(primaryParts) > 0 {
			right = append(right, strings.Join(primaryParts, " "))
		}
		right = append(right, lipgloss.NewStyle().Foreground(m.theme.TextHint).Render(primaryModel))
	} else {
		// No primary identified — show aggregated
		if maxPercent > 0 {
			ctxStyle := contextPercentStyle(maxPercent, m.theme)
			right = append(right, ctxStyle.Render(fmt.Sprintf("ctx:%.0f%%", maxPercent)))
		}
		if totalUsed > 0 {
			tokenStr := formatCompactTokens(totalUsed)
			if totalLimit > 0 {
				tokenStr += "/" + formatCompactTokens(totalLimit)
			}
			right = append(right, tokenStr)
		}
	}

	// Total cost
	if totalCost > 0 {
		costStyle := lipgloss.NewStyle().Foreground(m.theme.Text)
		right = append(right, costStyle.Render(fmt.Sprintf("$%.2f", totalCost)))
	}

	leftStr := strings.Join(left, sep)
	rightStr := strings.Join(right, sep)

	// Truncate to fit single line if terminal is narrow
	maxContent := m.width - 2 // 1 char padding each side
	leftW := lipgloss.Width(leftStr)
	rightW := lipgloss.Width(rightStr)
	if leftW+rightW+3 > maxContent && maxContent > 0 {
		// Prioritize right side (metrics); truncate left
		available := maxContent - rightW - 3
		if available < 3 {
			// Terminal too narrow — show right side only
			leftStr = ""
			leftW = 0
		} else if leftW > available {
			// Truncate left side
			leftStr = leftStr[:available] + "…"
			leftW = available + 1
		}
	}

	// Pad to fill width
	gap := maxContent - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	content := " " + leftStr + strings.Repeat(" ", gap) + rightStr + " "

	return barStyle.Render(content)
}

// contextPercentStyle returns a lipgloss style color-coded by context pressure.
func contextPercentStyle(percent float64, t Theme) lipgloss.Style {
	switch {
	case percent >= 95:
		return lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	case percent >= 80:
		return lipgloss.NewStyle().Foreground(t.Warning)
	case percent >= 60:
		return lipgloss.NewStyle().Foreground(t.YellowW)
	default:
		return lipgloss.NewStyle().Foreground(t.Success)
	}
}

// parseCost extracts a float from a cost string like "$0.12".
func parseCost(s string) float64 {
	s = strings.TrimPrefix(s, "$")
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

// parseTokenCount extracts an integer from a formatted token string like "12.4K".
func parseTokenCount(s string) int {
	if s == "" {
		return 0
	}
	var v float64
	if strings.HasSuffix(s, "M") {
		fmt.Sscanf(strings.TrimSuffix(s, "M"), "%f", &v)
		return int(v * 1_000_000)
	}
	if strings.HasSuffix(s, "K") {
		fmt.Sscanf(strings.TrimSuffix(s, "K"), "%f", &v)
		return int(v * 1_000)
	}
	fmt.Sscanf(s, "%f", &v)
	return int(v)
}

// formatCompactTokens formats a token count compactly.
func formatCompactTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
