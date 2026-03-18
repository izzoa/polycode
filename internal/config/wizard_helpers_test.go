package config

import (
	"strings"
	"testing"
)

// 5.1: AuthMethodsByType returns correct methods per provider type.
func TestAuthMethodsByType(t *testing.T) {
	// All 4 provider types should have entries
	expectedTypes := []ProviderType{
		ProviderTypeAnthropic,
		ProviderTypeOpenAI,
		ProviderTypeGoogle,
		ProviderTypeOpenAICompatible,
	}
	for _, pt := range expectedTypes {
		methods, ok := AuthMethodsByType[pt]
		if !ok {
			t.Errorf("AuthMethodsByType missing entry for %q", pt)
			continue
		}
		if len(methods) == 0 {
			t.Errorf("AuthMethodsByType[%q] has no auth methods", pt)
		}
	}

	// Anthropic should have api_key only (no OAuth for third-party apps)
	anthMethods := AuthMethodsByType[ProviderTypeAnthropic]
	if !containsAuthMethod(anthMethods, AuthMethodAPIKey) {
		t.Error("anthropic should support api_key")
	}
	if containsAuthMethod(anthMethods, AuthMethodOAuth) {
		t.Error("anthropic should not support oauth (not available for third-party apps)")
	}

	// OpenAI should have api_key only (no OAuth for third-party apps)
	openaiMethods := AuthMethodsByType[ProviderTypeOpenAI]
	if !containsAuthMethod(openaiMethods, AuthMethodAPIKey) {
		t.Error("openai should support api_key")
	}
	if containsAuthMethod(openaiMethods, AuthMethodOAuth) {
		t.Error("openai should not support oauth (not available for third-party apps)")
	}

	// openai_compatible should have api_key and none
	compatMethods := AuthMethodsByType[ProviderTypeOpenAICompatible]
	if !containsAuthMethod(compatMethods, AuthMethodAPIKey) {
		t.Error("openai_compatible should support api_key")
	}
	if !containsAuthMethod(compatMethods, AuthMethodNone) {
		t.Error("openai_compatible should support none")
	}
	if containsAuthMethod(compatMethods, AuthMethodOAuth) {
		t.Error("openai_compatible should not support oauth")
	}
}

func containsAuthMethod(methods []AuthMethod, target AuthMethod) bool {
	for _, m := range methods {
		if m == target {
			return true
		}
	}
	return false
}

// 5.3: FormatCapabilities produces expected badge strings.
func TestFormatCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		summary ModelSummary
		want    string
	}{
		{
			name: "all capabilities",
			summary: ModelSummary{
				Name:                    "test-model",
				MaxInputTokens:          200000,
				SupportsFunctionCalling: true,
				SupportsVision:          true,
				SupportsReasoning:       true,
			},
			want: "200K context | tools | vision | reasoning",
		},
		{
			name: "tools only",
			summary: ModelSummary{
				Name:                    "basic-model",
				MaxInputTokens:          128000,
				SupportsFunctionCalling: true,
				SupportsVision:          false,
				SupportsReasoning:       false,
			},
			want: "128K context | tools",
		},
		{
			name: "no capabilities",
			summary: ModelSummary{
				Name:           "minimal-model",
				MaxInputTokens: 4096,
			},
			want: "4K context",
		},
		{
			name:    "zero tokens no capabilities",
			summary: ModelSummary{Name: "empty"},
			want:    "",
		},
		{
			name: "million tokens",
			summary: ModelSummary{
				Name:                    "big-model",
				MaxInputTokens:          1048576,
				SupportsFunctionCalling: true,
				SupportsVision:          true,
				SupportsReasoning:       true,
			},
			want: "1.0M context | tools | vision | reasoning",
		},
		{
			name: "vision and reasoning only",
			summary: ModelSummary{
				Name:              "special-model",
				MaxInputTokens:    32000,
				SupportsVision:    true,
				SupportsReasoning: true,
			},
			want: "32K context | vision | reasoning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCapabilities(tt.summary)
			if got != tt.want {
				t.Errorf("FormatCapabilities() = %q, want %q", got, tt.want)
			}
		})
	}
}

// 5.4: DefaultModelByType has entries for all standard provider types.
func TestDefaultModelByType(t *testing.T) {
	expectedTypes := []ProviderType{
		ProviderTypeAnthropic,
		ProviderTypeOpenAI,
		ProviderTypeGoogle,
	}

	for _, pt := range expectedTypes {
		model, ok := DefaultModelByType[pt]
		if !ok {
			t.Errorf("DefaultModelByType missing entry for %q", pt)
			continue
		}
		if model == "" {
			t.Errorf("DefaultModelByType[%q] is empty", pt)
		}
	}

	// Verify specific defaults
	if DefaultModelByType[ProviderTypeAnthropic] != "claude-sonnet-4-20250514" {
		t.Errorf("expected anthropic default to be claude-sonnet-4-20250514, got %q",
			DefaultModelByType[ProviderTypeAnthropic])
	}
	if DefaultModelByType[ProviderTypeOpenAI] != "gpt-4o" {
		t.Errorf("expected openai default to be gpt-4o, got %q",
			DefaultModelByType[ProviderTypeOpenAI])
	}
	if !strings.HasPrefix(DefaultModelByType[ProviderTypeGoogle], "gemini") {
		t.Errorf("expected google default to start with gemini, got %q",
			DefaultModelByType[ProviderTypeGoogle])
	}
}
