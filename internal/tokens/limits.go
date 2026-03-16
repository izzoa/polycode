package tokens

// KnownLimits maps model identifiers to their context window size in tokens.
var KnownLimits = map[string]int{
	// Anthropic Claude
	"claude-sonnet-4-20250514":   200_000,
	"claude-opus-4-20250514":     200_000,
	"claude-haiku-3-5-20241022":  200_000,
	"claude-sonnet-4-6-latest":   200_000,
	"claude-opus-4-6-latest":     200_000,

	// OpenAI
	"gpt-4o":      128_000,
	"gpt-4o-mini": 128_000,
	"gpt-4-turbo": 128_000,
	"gpt-4":       8_192,
	"o1":          200_000,
	"o1-mini":     128_000,
	"o3":          200_000,
	"o3-mini":     200_000,
	"o4-mini":     200_000,

	// Google Gemini
	"gemini-2.5-pro":   1_048_576,
	"gemini-2.5-flash": 1_048_576,
	"gemini-2.0-flash": 1_048_576,
	"gemini-1.5-pro":   2_097_152,
	"gemini-1.5-flash": 1_048_576,
}

// LimitForModel returns the context window limit for a model.
//   - If configOverride > 0, it takes precedence.
//   - Otherwise, the built-in KnownLimits is consulted.
//   - If the model is unknown and no override is set, returns 0 (unlimited).
func LimitForModel(model string, configOverride int) int {
	if configOverride > 0 {
		return configOverride
	}
	if limit, ok := KnownLimits[model]; ok {
		return limit
	}
	return 0
}
