## Why

The current setup wizard (`polycode init` and the TUI add-provider wizard) is dumb — it asks for a raw model name string and shows generic placeholder hints. Users must know the exact model identifier (e.g., `claude-sonnet-4-20250514`) from memory. Meanwhile, polycode already has a litellm metadata store with 1000+ models and their capabilities. The wizard should use this data to offer a type-aware, guided experience: filter auth methods by provider type, show a selectable list of available models from litellm, display model capabilities (context window, function calling, vision), and validate the configuration before saving.

## What Changes

- **Provider-type-aware auth options**: When the user selects `anthropic`, show only `api_key` and `oauth`; for `openai_compatible`, show `api_key` and `none`; etc.
- **Model selection from litellm data**: After selecting a provider type, query the litellm metadata store for all models matching that provider, and present a searchable/filterable list instead of a raw text input
- **Model capabilities display**: When a model is highlighted in the list, show its context window size, function calling support, vision support, etc.
- **Connection validation**: After entering credentials, test the connection before saving — don't let users save a broken provider
- **Smart defaults**: Auto-suggest the most popular model for each provider type (e.g., `claude-sonnet-4-20250514` for Anthropic, `gpt-4o` for OpenAI)
- **Apply to both wizards**: The CLI `polycode init` wizard and the TUI add-provider wizard should both use the smarter flow

## Capabilities

### New Capabilities
- `smart-wizard`: Provider-type-aware wizard with litellm model browsing, capability display, and connection validation

### Modified Capabilities
_(none — this refines the existing wizard implementation without changing external contracts)_

## Impact

- **`cmd/polycode/setup.go`**: Refactored CLI wizard with type-aware prompts and model listing
- **`internal/tui/wizard.go`**: Refactored TUI wizard with model selection list and capability display
- **`internal/tokens/metadata.go`**: New method to query models by provider type
- **`internal/provider/`**: Connection test reused from existing settings test flow
