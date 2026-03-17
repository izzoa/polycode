## Context

The litellm `MetadataStore` already holds 1000+ models with `max_input_tokens`, `supports_function_calling`, `supports_vision`, etc. The `Lookup()` method finds a model by name, and the raw data is keyed by strings like `anthropic/claude-sonnet-4-20250514`, `openai/gpt-4o`, `google/gemini-2.5-pro`. This key format means we can filter by provider prefix to get all models for a given provider type.

The CLI wizard is in `cmd/polycode/setup.go` (basic stdin prompts). The TUI wizard is in `internal/tui/wizard.go` (multi-step Bubble Tea form). Both need the same smarts.

## Goals / Non-Goals

**Goals:**
- Filter auth methods by provider type in both wizards
- List available models from litellm metadata, filtered by provider type
- Show model metadata (context window, capabilities) during selection
- Auto-suggest the best default model per provider type
- Validate connection after credential entry
- Shared logic between CLI and TUI wizards

**Non-Goals:**
- Real-time API calls to provider endpoints for model listing (we use cached litellm data)
- Paginated/scrollable model browsing in the CLI wizard (CLI shows top 10 + manual entry)
- Custom model training or fine-tuned model support

## Decisions

### 1. Model listing: filtered from litellm metadata by provider prefix

**Choice**: Add `ModelsForProvider(providerType string) []ModelSummary` to `MetadataStore` that filters the litellm data by key prefix (`anthropic/`, `openai/`, `google/`). Returns a sorted list of `ModelSummary{Name, MaxInputTokens, SupportsFunctionCalling, SupportsVision}`.

**Rationale**: No new data source needed — litellm already has everything. Prefix filtering is simple and covers the major providers.

### 2. Provider type → auth methods mapping

**Choice**: A static map in a shared helper:

```go
var AuthMethodsByType = map[ProviderType][]AuthMethod{
    "anthropic":        {"api_key", "oauth"},
    "openai":           {"api_key", "oauth"},
    "google":           {"api_key", "oauth"},
    "openai_compatible": {"api_key", "none"},
}
```

### 3. Provider type → default model mapping

**Choice**: A static map of the most popular model per provider:

```go
var DefaultModelByType = map[ProviderType]string{
    "anthropic": "claude-sonnet-4-20250514",
    "openai":    "gpt-4o",
    "google":    "gemini-2.5-pro",
}
```

### 4. CLI wizard: show top models + allow manual entry

**Choice**: After selecting provider type, the CLI wizard shows up to 10 popular models with numbered selection:

```
Available models for anthropic:
  1. claude-sonnet-4-20250514 (200K context, tools, vision)
  2. claude-opus-4-20250514 (200K context, tools, vision, reasoning)
  3. claude-haiku-3-5-20241022 (200K context, tools)
  ...
  0. Enter model name manually

Select model [1]:
```

### 5. TUI wizard: filterable list with capability badges

**Choice**: The TUI `stepModel` changes from a text input to a list selector (like stepType). Each item shows the model name + capability badges. The user can type to filter. A "custom..." option at the bottom allows manual entry.

### 6. Connection validation integrated into wizard

**Choice**: After credentials are entered (stepAPIKey or OAuth), the wizard automatically sends a test query ("Say hello in one word") to validate the connection. If it fails, the user is told why and given the option to re-enter credentials or skip validation.

## Risks / Trade-offs

- **Litellm data may be stale/unavailable on first run**: → **Mitigation**: Fall back to hardcoded default models and allow manual entry. Never block the wizard on network.

- **Too many models in the list**: → **Mitigation**: Sort by popularity (hardcoded order for top models), limit CLI to 10, TUI supports scrolling + type-to-filter.
