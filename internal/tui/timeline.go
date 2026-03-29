package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderTurnTimeline renders a structured timeline for a completed exchange.
// Shows provider fan-out status, synthesis, tools, and files.
func renderTurnTimeline(ex Exchange, panels []ProviderPanel, t Theme) string {
	treeStyle := lipgloss.NewStyle().Foreground(t.BorderNormal)
	doneStyle := lipgloss.NewStyle().Foreground(t.Success)
	failStyle := lipgloss.NewStyle().Foreground(t.Error)
	timeoutStyle := lipgloss.NewStyle().Foreground(t.Warning)
	dimStyle := lipgloss.NewStyle().Foreground(t.TextMuted)

	var lines []string

	// Provider fan-out — use persisted status for accurate display
	if len(ex.ProviderStatuses) > 0 {
		var providerParts []string
		providerNames := sortedKeys(ex.ProviderStatuses)
		for _, name := range providerNames {
			status := ex.ProviderStatuses[name]
			switch status {
			case StatusDone:
				providerParts = append(providerParts, doneStyle.Render(name+" ✓"))
			case StatusFailed:
				providerParts = append(providerParts, failStyle.Render(name+" ✕"))
			case StatusTimedOut:
				providerParts = append(providerParts, timeoutStyle.Render(name+" ⏱"))
			case StatusIdle:
				providerParts = append(providerParts, dimStyle.Render(name+" ○"))
			default:
				providerParts = append(providerParts, dimStyle.Render(name+" ○"))
			}
		}
		if len(providerParts) > 0 {
			lines = append(lines, treeStyle.Render("├─ ")+"Fan-out: "+strings.Join(providerParts, ", "))
		}
	} else if len(ex.IndividualResponse) > 0 {
		// Fallback for old exchanges without ProviderStatuses
		var providerParts []string
		for _, name := range sortedKeys(ex.IndividualResponse) {
			if ex.IndividualResponse[name] != "" {
				providerParts = append(providerParts, doneStyle.Render(name+" ✓"))
			} else {
				providerParts = append(providerParts, dimStyle.Render(name+" ○"))
			}
		}
		if len(providerParts) > 0 {
			lines = append(lines, treeStyle.Render("├─ ")+"Fan-out: "+strings.Join(providerParts, ", "))
		}
	}

	// Tool calls
	if len(ex.ToolCalls) > 0 {
		names := make(map[string]int)
		var totalDur time.Duration
		for _, tc := range ex.ToolCalls {
			names[tc.ToolName]++
			totalDur += tc.Duration
		}
		var parts []string
		sortedToolNames := sortedKeys(names)
		for _, name := range sortedToolNames {
			count := names[name]
			if count > 1 {
				parts = append(parts, fmt.Sprintf("%s ×%d", name, count))
			} else {
				parts = append(parts, name)
			}
		}
		durStr := ""
		if totalDur > 0 {
			durStr = fmt.Sprintf(" (%s total)", totalDur.Round(time.Millisecond))
		}
		lines = append(lines, treeStyle.Render("├─ ")+"Tools: "+strings.Join(parts, ", ")+durStr)
	}

	if len(lines) == 0 {
		return ""
	}

	// Fix last line to use └─
	if len(lines) > 0 {
		last := lines[len(lines)-1]
		last = strings.Replace(last, "├─", "└─", 1)
		lines[len(lines)-1] = last
	}

	return strings.Join(lines, "\n")
}

// sortedKeys returns the sorted keys of a map.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
