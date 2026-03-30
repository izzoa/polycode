## 1. Metadata Fetcher & Cache

- [x] 1.1 Create `internal/tokens/metadata.go` with `ModelInfo` struct (MaxInputTokens, MaxOutputTokens, SupportsFunctionCalling, SupportsVision, SupportsReasoning, SupportsResponseSchema)
- [x] 1.2 Implement `FetchMetadata(url string, timeout time.Duration) (map[string]ModelInfo, error)` — HTTP GET the litellm JSON, parse each entry into ModelInfo, return the map
- [x] 1.3 Implement `LoadCachedMetadata(path string) (map[string]ModelInfo, time.Time, error)` — read cached JSON from disk, return parsed map and file mtime
- [x] 1.4 Implement `SaveCachedMetadata(path string, data []byte) error` — write raw JSON to cache path with 0600 permissions, creating parent dirs if needed

## 2. MetadataStore

- [x] 2.1 Create `MetadataStore` struct wrapping `map[string]ModelInfo` with lookup methods
- [x] 2.2 Implement `NewMetadataStore(cfg MetadataConfig) (*MetadataStore, error)` — orchestrates the fetch/cache/fallback flow: check cache mtime vs TTL → fetch if stale → parse → fall back to cache → fall back to empty
- [x] 2.3 Implement model name lookup with multi-strategy matching: exact key → `{providerType}/{model}` → bare model scan
- [x] 2.4 Implement `LimitForModel(model string, providerType string, configOverride int) int` — three-tier fallback: config override → litellm max_input_tokens → hardcoded KnownLimits → 0
- [x] 2.5 Implement `CapabilitiesForModel(model string, providerType string) ModelInfo` — returns parsed capabilities or zero-value struct if not found

## 3. Config Extension

- [x] 3.1 Add `MetadataConfig` struct to config with fields: `URL string`, `CacheTTL time.Duration` (raw string + parsed), default URL constant
- [x] 3.2 Add `Metadata MetadataConfig` field to the top-level `Config` struct with sensible defaults (24h TTL, default litellm URL)

## 4. Integration

- [x] 4.1 Refactor `LimitForModel()` in `limits.go` to delegate to `MetadataStore.LimitForModel()` when a store is available, falling back to the existing hardcoded logic
- [x] 4.2 Update `startTUI` in `app.go` to create a `MetadataStore` from config and pass provider type info when resolving limits for the `TokenTracker`
- [x] 4.3 Log a warning on startup if metadata fetch fails (but proceed with fallback)

## 5. Testing

- [x] 5.1 Unit test: parse a sample litellm JSON snippet into `map[string]ModelInfo`, verify fields
- [x] 5.2 Unit test: model name lookup with exact match, provider-prefixed match, and miss
- [x] 5.3 Unit test: `LimitForModel` three-tier fallback (config override → litellm → hardcoded → 0)
- [x] 5.4 Unit test: cache TTL logic — fresh cache skips fetch, stale cache triggers fetch, missing cache triggers fetch
- [x] 5.5 Unit test: capabilities lookup returns correct flags, unknown model returns zero-value
