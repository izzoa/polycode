package config

import (
	"fmt"
	"strings"
)

// AuthMethodsByType maps provider type to valid auth methods.
var AuthMethodsByType = map[ProviderType][]AuthMethod{
	ProviderTypeAnthropic:        {AuthMethodAPIKey, AuthMethodOAuth},
	ProviderTypeOpenAI:           {AuthMethodAPIKey, AuthMethodOAuth},
	ProviderTypeGoogle:           {AuthMethodAPIKey, AuthMethodOAuth},
	ProviderTypeOpenAICompatible: {AuthMethodAPIKey, AuthMethodNone},
}

// DefaultModelByType maps provider type to its most popular model.
var DefaultModelByType = map[ProviderType]string{
	ProviderTypeAnthropic: "claude-sonnet-4-20250514",
	ProviderTypeOpenAI:    "gpt-4o",
	ProviderTypeGoogle:    "gemini-2.5-pro",
}

// ModelSummary holds display info for a model in the wizard.
type ModelSummary struct {
	Name                    string
	MaxInputTokens          int
	SupportsFunctionCalling bool
	SupportsVision          bool
	SupportsReasoning       bool
}

// FormatCapabilities returns a human-readable capability string.
// e.g. "200K context | tools | vision | reasoning"
func FormatCapabilities(m ModelSummary) string {
	var parts []string

	if m.MaxInputTokens > 0 {
		parts = append(parts, formatTokenCount(m.MaxInputTokens)+" context")
	}
	if m.SupportsFunctionCalling {
		parts = append(parts, "tools")
	}
	if m.SupportsVision {
		parts = append(parts, "vision")
	}
	if m.SupportsReasoning {
		parts = append(parts, "reasoning")
	}

	return strings.Join(parts, " | ")
}

// formatTokenCount formats a token count for display.
// e.g., 1234 -> "1.2K", 1234567 -> "1.2M", 500 -> "500"
func formatTokenCount(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.0fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
