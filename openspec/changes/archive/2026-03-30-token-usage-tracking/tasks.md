## 1. Token Types & Model Limits Registry

- [x] 1.1 Create `internal/tokens/` package with `Usage` struct (InputTokens, OutputTokens int) and `TokenTracker` struct (per-provider accumulator with mutex, Add/Get/Summary methods)
- [x] 1.2 Create `internal/tokens/limits.go` with `KnownLimits` map (model name → context window size) covering Claude, GPT-4o, GPT-4o-mini, Gemini 2.5 Pro/Flash
- [x] 1.3 Add `func LimitForModel(model string, configOverride int) int` that returns the config override if set, else the built-in limit, else 0 (unlimited)

## 2. Provider Adapter Changes

- [x] 2.1 Add `InputTokens` and `OutputTokens` fields to `provider.StreamChunk`
- [x] 2.2 Update Anthropic adapter to parse `usage.input_tokens` and `usage.output_tokens` from the `message_delta` event and set them on the final Done chunk
- [x] 2.3 Update OpenAI adapter to parse `usage` from the final SSE chunk (when `stream_options.include_usage` is set) and set on the Done chunk
- [x] 2.4 Update Gemini adapter to parse `usageMetadata.promptTokenCount` and `usageMetadata.candidatesTokenCount` and set on the Done chunk
- [x] 2.5 Ensure OpenAI-compatible adapter passes through usage data when available (zero when absent)

## 3. Config Extension

- [x] 3.1 Add `MaxContext int` field to `ProviderConfig` with `yaml:"max_context,omitempty"` tag
- [x] 3.2 Wire `MaxContext` through to the token tracker initialization so each provider gets its resolved limit (config override > built-in > unlimited)

## 4. Fan-Out & Pipeline Integration

- [x] 4.1 Update `FanOutResult` to include per-provider `tokens.Usage` alongside responses
- [x] 4.2 Update the fan-out goroutines to extract usage from the final StreamChunk and populate FanOutResult
- [x] 4.3 Add pre-dispatch context limit check: before sending to a provider, compare last known input tokens against limit; skip providers that would exceed
- [x] 4.4 Handle primary-provider-excluded edge case: return an error if the primary would exceed its limit

## 5. App Layer Wiring

- [x] 5.1 Create a `TokenTracker` in `startTUI`, initialized with each provider's resolved limit
- [x] 5.2 After each pipeline run, update the tracker with usage from `FanOutResult` and consensus synthesis
- [x] 5.3 Pass tracker (or a read-only snapshot) to the TUI model for display

## 6. TUI Display

- [x] 6.1 Add token usage to the status bar: `providerName used/limit` format (e.g., `12.4K/200K`) using `formatTokenCount` helper (K for thousands, M for millions)
- [x] 6.2 Color-code usage: default (normal), yellow/amber at >80% of limit, red at >95%
- [x] 6.3 Show "N/A" for providers that report zero usage (no data from API)
- [x] 6.4 Show notice in chat when a provider is excluded due to context limit

## 7. Testing

- [x] 7.1 Unit tests for `TokenTracker`: Add, Get, Summary, concurrent access
- [x] 7.2 Unit tests for `LimitForModel`: built-in lookup, config override, unknown model
- [x] 7.3 Unit tests for `formatTokenCount` helper
- [x] 7.4 Unit test for pre-dispatch limit check logic
