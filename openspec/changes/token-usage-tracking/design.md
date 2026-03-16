## Context

Polycode currently sends queries to multiple LLM providers and synthesizes consensus, but has zero visibility into token consumption. Each provider API returns usage data in its response (input tokens, output tokens), and each model has a known context window limit. This data is currently ignored. As conversations grow, users have no way to know when they're approaching limits or how much each turn costs.

The existing `StreamChunk` type carries only text deltas — it has no usage fields. The `conversationState` in `app.go` accumulates messages but doesn't track their token footprint.

## Goals / Non-Goals

**Goals:**
- Parse and surface token usage from every provider API response (Anthropic, OpenAI, Gemini, OpenAI-compatible)
- Maintain a session-wide running total of input/output tokens per provider
- Display live usage in the TUI status bar as `used/limit` per provider
- Warn users visually when any provider approaches its context limit (>80%)
- Skip providers that would exceed their context limit on the next query
- Track consensus synthesis token usage separately

**Non-Goals:**
- Cost estimation in dollars (provider pricing changes frequently — out of scope for v1)
- Client-side tokenization or token counting (we rely on API-reported usage)
- Persistent usage tracking across sessions (session-scoped only)
- Token-aware message pruning or summarization to stay within limits

## Decisions

### 1. Token data source: API response metadata

**Choice**: Extract token counts from each provider's API response rather than counting tokens client-side.

**Rationale**: Every major LLM API returns `usage` metadata (input_tokens, output_tokens) in its response. This is authoritative and accounts for the provider's actual tokenization. Client-side tokenizers (like tiktoken) are model-specific, don't exist for all providers, and add dependencies.

**Alternatives considered**:
- **Client-side tiktoken**: Only works for OpenAI models, adds a dependency, may disagree with server counts
- **Estimate by character count**: Inaccurate (varies 2-4x depending on language/content)

### 2. Where to carry usage data: on the final StreamChunk

**Choice**: Add `InputTokens` and `OutputTokens` fields to `StreamChunk`. The final chunk (where `Done: true`) carries the usage for the entire response. Mid-stream chunks have zero values.

**Rationale**: Usage data is only available after the full response completes (Anthropic's `message_delta` event, OpenAI's final chunk with `usage` field). Putting it on `StreamChunk` avoids adding a separate channel or callback — the existing streaming pipeline naturally delivers it.

### 3. Model limits: built-in registry with config override

**Choice**: A hardcoded map of known model → context window size, with per-provider `max_context` override in config.

```go
var KnownLimits = map[string]int{
    "claude-sonnet-4-20250514": 200000,
    "claude-opus-4-20250514":   200000,
    "gpt-4o":                    128000,
    "gpt-4o-mini":               128000,
    "gemini-2.5-pro":           1048576,
    "gemini-2.5-flash":         1048576,
}
```

Config override:
```yaml
providers:
  - name: claude
    type: anthropic
    model: claude-sonnet-4-20250514
    max_context: 150000  # optional override
```

**Rationale**: Hardcoded defaults cover the common models. Config override handles new/custom models and lets users set conservative limits. The registry is trivial to update.

### 4. Tracker architecture: per-provider accumulator

**Choice**: A `TokenTracker` struct with a map of provider ID → `UsageRecord` (input total, output total, limit). Thread-safe via mutex. Lives alongside `conversationState` in the app layer.

**Rationale**: Simple, centralized, no coupling to the pipeline. The app layer updates it after each query completes. The TUI reads it for display.

### 5. Context limit enforcement: pre-query check

**Choice**: Before dispatching a query to a provider, check if `currentInputTokens + estimatedNewTokens > limit`. If so, skip that provider for this turn and show a warning. The "estimate" uses the last known input token count as a proxy.

**Rationale**: We can't know exact token count for the next query without tokenizing, but the last usage report gives a close approximation since conversation grows incrementally. This is a best-effort guardrail, not a hard guarantee.

## Risks / Trade-offs

- **Usage data availability varies by provider**: Some OpenAI-compatible endpoints may not return usage data. → **Mitigation**: Treat missing usage as zero; show "N/A" in the TUI for those providers.

- **Token counts lag by one turn**: We only know exact usage after the API responds, so the limit check for the *next* query uses the *previous* turn's count. → **Mitigation**: Use 80% threshold for warnings to provide buffer. Users see the warning and can start a new session.

- **Hardcoded limits go stale**: Model context windows change. → **Mitigation**: Config override exists. The hardcoded map is easy to update. Unknown models default to no limit (infinite).
