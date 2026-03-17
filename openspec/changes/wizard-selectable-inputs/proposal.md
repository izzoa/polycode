## Why

The CLI setup wizard (`polycode init`) currently uses plain text input for all fields, requiring users to type exact values for pre-defined options like provider type and auth method. This is error-prone and unfriendly — users must know valid values upfront. Model selection already shows a numbered list, but it still requires typing a number rather than navigating with arrow keys. Replacing text input with interactive selectable lists for constrained fields and a filterable model picker pre-populated from litellm will make the first-run experience polished and intuitive.

## What Changes

- **Provider type selection**: Replace text input with an arrow-key navigable list of valid provider types (anthropic, openai, google, openai_compatible)
- **Auth method selection**: Replace text input with a selectable list filtered by the chosen provider type
- **Model selection**: Replace the numbered list with an interactive, filterable/searchable list pre-populated from litellm metadata, with a "Custom model..." escape hatch for manual entry
- **Primary provider toggle**: Replace text input with a yes/no selector (when applicable, i.e. when other providers already exist)
- **Retain text input** for free-form fields: provider name, API key, base URL

## Capabilities

### New Capabilities
- `cli-selectable-prompts`: Interactive arrow-key list selection component for the CLI setup wizard, replacing text input for all fields with pre-defined option sets
- `cli-model-browser`: Filterable model picker for the CLI wizard that loads models from litellm metadata, shows capabilities, and allows type-to-filter with a custom model fallback

### Modified Capabilities
<!-- No existing specs to modify -->

## Impact

- **Code**: `cmd/polycode/setup.go` — rewrite of `runSetupWizard()` and `selectModel()` to use interactive selection
- **Dependencies**: Add `charmbracelet/huh` (or similar Charm forms library) for interactive terminal prompts — fits naturally with the existing Bubble Tea ecosystem
- **UX**: First-run experience changes from a plain text Q&A to a guided, interactive wizard with arrow-key navigation
- **No breaking changes**: Config format, provider interface, and TUI wizard are all unaffected
