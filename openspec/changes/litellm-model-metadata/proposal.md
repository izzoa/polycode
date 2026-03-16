## Why

Polycode currently maintains a hardcoded `KnownLimits` map in `internal/tokens/limits.go` with manually curated context window sizes for ~25 models. This goes stale quickly as new models ship and existing limits change. Meanwhile, the litellm project maintains a comprehensive, community-updated JSON file (`model_prices_and_context_window.json`) covering hundreds of models with not just token limits but also capability flags (function calling, vision, reasoning, etc.). By fetching this data dynamically, polycode can support any model without manual updates and can make smarter decisions about which providers to query.

## What Changes

- **Dynamic model metadata fetching**: On startup (or on demand), polycode fetches litellm's `model_prices_and_context_window.json` from GitHub and caches it locally
- **Replace hardcoded limits**: The `KnownLimits` map becomes a fallback; the primary source of truth for `max_input_tokens` is the litellm data
- **Model capabilities awareness**: Extract capability flags (`supports_function_calling`, `supports_vision`, `supports_reasoning`, etc.) so the consensus pipeline can make informed decisions (e.g., only send tool-use queries to providers that support function calling)
- **Local cache with TTL**: The fetched data is cached at `~/.config/polycode/model_metadata.json` with a configurable TTL (default 24h) so polycode doesn't hit the network on every startup
- **Offline fallback**: If the network fetch fails, fall back to the local cache; if no cache exists, fall back to the existing hardcoded limits

## Capabilities

### New Capabilities
- `model-metadata`: Dynamic fetching, caching, and lookup of model token limits and capabilities from litellm's model database

### Modified Capabilities
_(none — this replaces the internal data source for token limits without changing the external behavior of token tracking)_

## Impact

- **`internal/tokens/limits.go`**: Refactored to use dynamic metadata as primary source, hardcoded map as fallback
- **New `internal/tokens/metadata.go`**: Fetcher, parser, cache logic for litellm data
- **`internal/config/`**: New `metadata` config section with `cache_ttl` and optional `metadata_url` override
- **`internal/provider/`**: Provider adapters can query capabilities to determine if tools/vision should be included in requests
- **Network**: Outbound HTTPS to `raw.githubusercontent.com` on startup (cacheable, optional)
