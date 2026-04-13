package tui

import (
	"fmt"
	"strings"
)

// providerStatusLabel returns a human-readable label for a ProviderStatus.
func providerStatusLabel(s ProviderStatus) string {
	switch s {
	case StatusFailed:
		return "Failed"
	case StatusTimedOut:
		return "Timed Out"
	case StatusCancelled:
		return "Cancelled"
	case StatusLoading:
		return "In Progress"
	default:
		return ""
	}
}

// providerHeading builds a markdown heading for a provider panel.
// Examples: "## claude-sonnet (Primary)", "## gpt-4o (Failed)",
// "## claude-sonnet (Primary, Timed Out)".
func providerHeading(name string, isPrimary bool, status ProviderStatus) string {
	var annotations []string
	if isPrimary {
		annotations = append(annotations, "Primary")
	}
	if label := providerStatusLabel(status); label != "" {
		annotations = append(annotations, label)
	}
	if len(annotations) > 0 {
		return fmt.Sprintf("## %s (%s)", name, strings.Join(annotations, ", "))
	}
	return fmt.Sprintf("## %s", name)
}

// buildShareMarkdown assembles a markdown document containing the user's
// prompt, all provider responses (individually labeled), and the consensus
// response for the latest exchange. Returns empty string if there is nothing
// to share. Providers that were not routed (StatusIdle) are omitted.
func (m *Model) buildShareMarkdown() string {
	// Determine data source: live panels (mid-stream) or last history entry.
	if m.querying {
		return m.buildShareFromPanels()
	}
	if len(m.history) > 0 {
		return m.buildShareFromHistory()
	}
	return ""
}

// buildShareFromPanels builds share markdown from live panel state (mid-stream).
func (m *Model) buildShareFromPanels() string {
	var b strings.Builder

	// Prompt
	b.WriteString("## Prompt\n\n")
	b.WriteString(m.currentPrompt)
	b.WriteString("\n\n")

	// Individual provider responses — skip idle (non-routed) providers
	for i := range m.panels {
		p := &m.panels[i]
		if p.Status == StatusIdle {
			continue
		}
		heading := providerHeading(p.Name, p.IsPrimary, p.Status)
		b.WriteString(heading)
		b.WriteString("\n\n")
		content := p.Content.String()
		if content != "" {
			b.WriteString(content)
			b.WriteString("\n\n")
		}
	}

	// Consensus
	consensus := m.consensusContent.String()
	if consensus != "" {
		b.WriteString("## Consensus\n\n")
		b.WriteString(consensus)
		b.WriteString("\n")
	}

	return b.String()
}

// buildShareFromHistory builds share markdown from the last completed exchange.
// Uses the exchange's own ProviderOrder and PrimaryProvider metadata to avoid
// depending on the current panel configuration (which may have changed since
// the exchange was recorded).
func (m *Model) buildShareFromHistory() string {
	last := m.history[len(m.history)-1]
	var b strings.Builder

	// Prompt
	b.WriteString("## Prompt\n\n")
	b.WriteString(last.Prompt)
	b.WriteString("\n\n")

	// Determine provider iteration order.
	// Prefer the exchange's own ProviderOrder (captured at exchange time).
	// Fall back to current panels for older exchanges that lack it,
	// then append any historical providers not in the panel list.
	providerNames := last.ProviderOrder
	if len(providerNames) == 0 {
		providerNames = make([]string, len(m.panels))
		for i, p := range m.panels {
			providerNames[i] = p.Name
		}
		// Append historical providers not in current panels.
		seen := make(map[string]bool, len(providerNames))
		for _, n := range providerNames {
			seen[n] = true
		}
		for name := range last.IndividualResponse {
			if !seen[name] {
				providerNames = append(providerNames, name)
				seen[name] = true
			}
		}
	}

	// Determine primary provider name.
	// Prefer the exchange's own PrimaryProvider; fall back to current panels.
	primaryName := last.PrimaryProvider
	if primaryName == "" {
		for _, p := range m.panels {
			if p.IsPrimary {
				primaryName = p.Name
				break
			}
		}
	}

	// Individual provider responses — skip idle (non-routed) providers
	for _, name := range providerNames {
		status := StatusDone // default for completed exchanges
		if s, ok := last.ProviderStatuses[name]; ok {
			status = s
		}
		if status == StatusIdle {
			continue
		}
		isPrimary := name == primaryName
		heading := providerHeading(name, isPrimary, status)
		b.WriteString(heading)
		b.WriteString("\n\n")
		if content, ok := last.IndividualResponse[name]; ok && content != "" {
			b.WriteString(content)
			b.WriteString("\n\n")
		}
	}

	// Consensus
	if last.ConsensusResponse != "" {
		b.WriteString("## Consensus\n\n")
		b.WriteString(last.ConsensusResponse)
		b.WriteString("\n")
	}

	return b.String()
}
