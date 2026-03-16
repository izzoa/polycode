## ADDED Requirements

### Requirement: Fetch model metadata from litellm on startup
The system SHALL fetch the litellm model metadata JSON from the configured URL on startup when the local cache is expired or missing.

#### Scenario: First startup with no cache
- **WHEN** polycode starts and no cached metadata file exists
- **THEN** the system fetches the litellm JSON from the remote URL, parses it, and caches it locally

#### Scenario: Startup with fresh cache
- **WHEN** polycode starts and the cached metadata file is newer than the configured TTL
- **THEN** the system uses the cached file without making a network request

#### Scenario: Startup with stale cache
- **WHEN** polycode starts and the cached metadata file is older than the configured TTL
- **THEN** the system attempts to fetch fresh data; if successful, updates the cache; if fetch fails, uses the stale cache

#### Scenario: Startup with no network and no cache
- **WHEN** polycode starts, no cache exists, and the network fetch fails
- **THEN** the system falls back to the hardcoded KnownLimits map and logs a warning

### Requirement: Cache metadata locally with configurable TTL
The system SHALL cache the fetched metadata at `~/.config/polycode/model_metadata.json` and support a configurable TTL in the config file (default 24 hours).

#### Scenario: Default TTL
- **WHEN** no `metadata.cache_ttl` is set in config
- **THEN** the system uses a 24-hour TTL

#### Scenario: Custom TTL
- **WHEN** `metadata.cache_ttl: 1h` is set in config
- **THEN** the system re-fetches metadata if the cache is older than 1 hour

#### Scenario: Cache file created after fetch
- **WHEN** a successful fetch completes
- **THEN** the raw JSON is written to the cache path with appropriate file permissions (0600)

### Requirement: Resolve model token limits from metadata
The system SHALL resolve a model's context window limit using a three-tier fallback: (1) config `max_context` override, (2) litellm metadata `max_input_tokens`, (3) hardcoded `KnownLimits`. If none match, the limit is 0 (unlimited).

#### Scenario: Config override takes precedence
- **WHEN** a provider has `max_context: 100000` in config and litellm reports `max_input_tokens: 200000`
- **THEN** the resolved limit is 100,000

#### Scenario: Litellm data used when no override
- **WHEN** a provider has no `max_context` override and litellm reports `max_input_tokens: 200000`
- **THEN** the resolved limit is 200,000

#### Scenario: Hardcoded fallback when model not in litellm
- **WHEN** a model is not found in the litellm data but exists in the hardcoded KnownLimits
- **THEN** the resolved limit comes from the hardcoded map

#### Scenario: Unknown model with no data anywhere
- **WHEN** a model is not found in litellm data, not in hardcoded limits, and has no config override
- **THEN** the resolved limit is 0 (unlimited)

### Requirement: Model name matching with provider-prefixed lookup
The system SHALL support multiple name matching strategies when looking up a model in the litellm data: exact match, provider-prefixed match (e.g., `openai/gpt-4o`), and bare model name.

#### Scenario: Exact match
- **WHEN** the model name `gpt-4o` exists as a key in the litellm data
- **THEN** that entry is returned

#### Scenario: Provider-prefixed match
- **WHEN** the model `gpt-4o` does not exist as an exact key but `openai/gpt-4o` does
- **THEN** the entry for `openai/gpt-4o` is returned

#### Scenario: No match found
- **WHEN** the model name matches no key in the litellm data under any strategy
- **THEN** the lookup returns no result and the system falls through to hardcoded limits

### Requirement: Expose model capabilities
The system SHALL parse and expose capability flags from the litellm data including `supports_function_calling`, `supports_vision`, `supports_reasoning`, and `supports_response_schema`.

#### Scenario: Model supports function calling
- **WHEN** the litellm entry for a model has `"supports_function_calling": true`
- **THEN** the capabilities lookup for that model returns `SupportsFunctionCalling: true`

#### Scenario: Model capabilities unknown
- **WHEN** a model is not found in the litellm data
- **THEN** all capability flags default to false

### Requirement: Configurable metadata source URL
The system SHALL allow the metadata source URL to be overridden via config for users who want to use a mirror, fork, or local file.

#### Scenario: Default URL
- **WHEN** no `metadata.url` is set in config
- **THEN** the system fetches from the default litellm GitHub raw URL

#### Scenario: Custom URL
- **WHEN** `metadata.url: https://internal.example.com/models.json` is set in config
- **THEN** the system fetches from that URL instead
