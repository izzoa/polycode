## 1. Shared Wizard Helpers

- [x] 1.1 Create `internal/config/wizard_helpers.go` with `AuthMethodsByType` map (provider type → valid auth methods) and `DefaultModelByType` map (provider type → default model name)
- [x] 1.2 Add `ModelsForProvider(providerType string) []ModelSummary` to `MetadataStore` in `internal/tokens/metadata.go` — filters litellm data by provider prefix, returns sorted list
- [x] 1.3 Define `ModelSummary` struct: Name, MaxInputTokens, SupportsFunctionCalling, SupportsVision, SupportsReasoning
- [x] 1.4 Sort models by popularity: hardcode a priority list of top models per provider (sonnet, opus, gpt-4o, gemini-2.5-pro, etc.) at the top, rest alphabetical
- [x] 1.5 Add `FormatCapabilities(m ModelSummary) string` helper — returns e.g. "200K context | tools | vision | reasoning"

## 2. CLI Wizard Refinement (polycode init)

- [x] 2.1 After provider type selection, show only valid auth methods for that type (use AuthMethodsByType)
- [x] 2.2 After auth method selection, show numbered list of top 10 models from ModelsForProvider with capabilities, plus option 0 for manual entry
- [x] 2.3 Default selection is the first model (from DefaultModelByType); user can press Enter to accept
- [x] 2.4 After API key entry, run a connection test (send "Say hello" to provider) — show success/failure
- [x] 2.5 On connection failure, offer: (r)etry credentials, (s)kip validation, (q)uit
- [x] 2.6 If litellm metadata unavailable, fall back to text input with DefaultModelByType as placeholder

## 3. TUI Wizard Refinement

- [x] 3.1 Update `stepAuth` in wizard.go to filter auth method list using AuthMethodsByType
- [x] 3.2 Update `stepModel` to show a selectable list of models from ModelsForProvider instead of text input
- [x] 3.3 Add capability badges next to each model name in the list (e.g., "claude-sonnet-4... — 200K | tools | vision")
- [x] 3.4 Add a "Custom model..." entry at the bottom of the list that switches to text input
- [x] 3.5 After API key step, auto-run connection test — show spinner, then result. If fail, show error with option to re-enter or skip
- [x] 3.6 If litellm metadata unavailable, fall back to text input with hint text

## 4. MetadataStore Integration

- [x] 4.1 Create the MetadataStore early in app startup (before TUI model creation) so it's available to wizards
- [x] 4.2 Pass the MetadataStore (or a model listing function) to the TUI model via a new `SetModelLister` callback
- [x] 4.3 In the CLI wizard, create a MetadataStore inline for model listing
- [x] 4.4 Handle nil MetadataStore gracefully in all wizard paths (fall back to manual entry)

## 5. Testing

- [x] 5.1 Unit test: AuthMethodsByType returns correct methods per provider type
- [x] 5.2 Unit test: ModelsForProvider filters correctly by provider prefix
- [x] 5.3 Unit test: FormatCapabilities produces expected badge strings
- [x] 5.4 Unit test: DefaultModelByType has entries for all standard provider types
