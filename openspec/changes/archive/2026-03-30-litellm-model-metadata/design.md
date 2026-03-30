## Context

Polycode's `internal/tokens/limits.go` has a `KnownLimits` map with ~25 entries that must be manually updated. The litellm project at `github.com/BerriAI/litellm` maintains `model_prices_and_context_window.json` — a comprehensive database of 1000+ model entries with fields including `max_input_tokens`, `max_output_tokens`, `supports_function_calling`, `supports_vision`, `supports_reasoning`, and more. This is community-maintained and updated frequently.

## Goals / Non-Goals

**Goals:**
- Fetch litellm's model metadata JSON at startup and cache it locally
- Use it as the primary source for `max_input_tokens` (context window limit)
- Expose model capabilities (function calling, vision, reasoning) for use by the pipeline
- Graceful degradation: network failure → cached file → hardcoded fallback
- Configurable cache TTL (default 24h)

**Non-Goals:**
- Using litellm as a proxy or runtime dependency (we only consume the static JSON)
- Cost estimation or pricing data (we ignore the cost fields)
- Auto-selecting models based on capabilities (user still picks models in config)
- Bundling the full JSON in the binary (too large, goes stale)

## Decisions

### 1. Data source: raw GitHub URL

**Choice**: Fetch from `https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json`.

**Rationale**: This is a stable, public URL that doesn't require authentication. Raw GitHub content is CDN-cached. The file is ~2MB but compresses well and is fetched at most once per TTL period.

**Alternatives considered**:
- **litellm PyPI package**: Requires Python runtime — not viable for a Go binary
- **Bundle in binary**: Goes stale between releases; 2MB adds to binary size
- **Custom API**: Over-engineering for a static file

### 2. Cache strategy: file with TTL check

**Choice**: Cache the fetched JSON at `~/.config/polycode/model_metadata.json`. On startup, check the file's modification time. If it's older than `cache_ttl` (default 24h), re-fetch. If fetch fails, use stale cache. If no cache exists and fetch fails, fall back to hardcoded `KnownLimits`.

**Rationale**: Simple, no extra dependencies. `mtime` check is fast. The three-tier fallback (fresh fetch → stale cache → hardcoded) ensures polycode always works.

### 3. Model name matching: fuzzy prefix with normalization

**Choice**: Litellm's keys include provider prefixes like `openai/gpt-4o`, `anthropic/claude-sonnet-4-20250514`, and also bare names like `gpt-4o`. When looking up a model, try exact match first, then try `{provider_type}/{model}` (e.g., `openai/gpt-4o`), then try bare model name.

**Rationale**: Litellm's naming is inconsistent — some models appear under multiple keys. A multi-step lookup handles the common cases without requiring users to match litellm's exact naming.

### 4. Parsed data structure: ModelInfo struct

**Choice**: Parse only the fields we need into a lean struct:

```go
type ModelInfo struct {
    MaxInputTokens           int
    MaxOutputTokens          int
    SupportsFunctionCalling  bool
    SupportsVision           bool
    SupportsReasoning        bool
    SupportsResponseSchema   bool
}
```

**Rationale**: The litellm JSON has many fields we don't need (pricing, caching costs, etc.). Parsing only what we use keeps memory low and the code simple.

### 5. Integration point: MetadataStore replaces direct KnownLimits

**Choice**: A `MetadataStore` struct that wraps the litellm data and exposes `LimitForModel(model, configOverride)` and `CapabilitiesForModel(model)`. The existing `LimitForModel` function delegates to the store, which tries litellm data first, then falls back to `KnownLimits`.

**Rationale**: The store is the single lookup point. The rest of the codebase doesn't need to know whether the data came from litellm or the hardcoded map.

## Risks / Trade-offs

- **Network dependency on startup**: First run with no cache requires network access. → **Mitigation**: Fetch is non-blocking with a 5s timeout. On failure, hardcoded limits work fine.

- **Litellm data quality**: Community-maintained data could have errors. → **Mitigation**: Hardcoded `KnownLimits` serves as a trusted fallback for the most common models. Config `max_context` override always wins.

- **File size**: The JSON is ~2MB. → **Mitigation**: Fetched at most once per 24h, cached locally. No impact on runtime performance after parsing.

- **Breaking changes in litellm JSON format**: Field names could change. → **Mitigation**: We parse with `json.RawMessage` tolerance — unknown fields are ignored. If key fields disappear, we fall back to hardcoded.
