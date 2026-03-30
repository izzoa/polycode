## 1. View Mode Infrastructure

- [x] 1.1 Add `viewMode` type and constants (`viewChat`, `viewSettings`, `viewAddProvider`, `viewEditProvider`) to the TUI model
- [x] 1.2 Update `View()` to dispatch to the appropriate render function based on `viewMode`
- [x] 1.3 Update `Update()` to route key events based on `viewMode` — each mode gets its own key handling block
- [x] 1.4 Add `Ctrl+S` shortcut in chat mode to switch to `viewSettings` (blocked during active query)
- [x] 1.5 Add `/settings` command detection in the input handler — if input starts with `/settings`, switch to settings view instead of submitting as a prompt

## 2. Settings List Screen

- [x] 2.1 Create `internal/tui/settings.go` with `renderSettings()` — table-style provider list with columns: name, type, model, auth, primary (★), status
- [x] 2.2 Add `settingsCursor int` field to the model for navigation within the provider list
- [x] 2.3 Handle j/↓ and k/↑ for cursor movement in settings mode
- [x] 2.4 Handle `Esc` to return to chat view
- [x] 2.5 Display action hints at the bottom: `a:add  e:edit  d:delete  t:test  Esc:back`
- [x] 2.6 Handle empty state: show "No providers configured — press 'a' to add one"

## 3. Add Provider Wizard

- [x] 3.1 Define `wizardStep` type and step constants (stepType, stepName, stepAuth, stepAPIKey, stepModel, stepBaseURL, stepPrimary, stepConfirm)
- [x] 3.2 Add wizard state fields to the model: `wizardStep`, `wizardData` (partial ProviderConfig being built), `wizardInput textinput.Model`, `wizardList` (for selection steps)
- [x] 3.3 Implement stepType: list selector with options [anthropic, openai, google, openai_compatible]
- [x] 3.4 Implement stepName: text input with auto-suggested default based on type (e.g., "claude" for anthropic)
- [x] 3.5 Implement stepAuth: list selector filtered by provider type (e.g., openai_compatible gets [api_key, none])
- [x] 3.6 Implement stepAPIKey: masked text input (only shown when auth is api_key)
- [x] 3.7 Implement stepModel: text input with hint showing popular models for the selected type
- [x] 3.8 Implement stepBaseURL: text input (only shown for openai_compatible type)
- [x] 3.9 Implement stepPrimary: y/n selection — "Set as primary provider?"
- [x] 3.10 Implement stepConfirm: show summary of all entered fields, Enter to save, Esc to cancel
- [x] 3.11 On confirm: append new ProviderConfig to config, store API key in auth store, save config, send ConfigChangedMsg
- [x] 3.12 Handle Esc at any step: cancel wizard, return to settings list

## 4. Edit Provider

- [x] 4.1 Handle `e` key in settings mode: switch to `viewEditProvider` with the selected provider's data pre-filled
- [x] 4.2 Show editable fields: model, auth method, API key (re-enter), max_context, primary designation
- [x] 4.3 On confirm: update the ProviderConfig in the config slice, save, send ConfigChangedMsg
- [x] 4.4 Handle primary change: un-mark the old primary, mark the new one

## 5. Remove Provider

- [x] 5.1 Handle `d` key in settings mode: show confirmation prompt "Remove provider '{name}'? (y/n)"
- [x] 5.2 Block removal if the selected provider is primary — show error message
- [x] 5.3 On confirm: remove from config.Providers, save config, delete stored credentials, send ConfigChangedMsg

## 6. Test Connection

- [x] 6.1 Handle `t` key in settings mode: send a minimal test query to the selected provider ("ping" or a short prompt)
- [x] 6.2 Display spinner while testing, then show "✓ Connected (350ms)" or "✕ Error: {message}"
- [x] 6.3 Run the test in a goroutine, send result back via a TUI message

## 7. Live Config Reload

- [x] 7.1 Define `ConfigChangedMsg` with the updated `*config.Config`
- [x] 7.2 Handle `ConfigChangedMsg` in the app layer: rebuild Registry, re-authenticate, rebuild Pipeline, update TUI model's provider panels
- [x] 7.3 Update `provider.Registry` with a `Refresh(cfg *config.Config)` method or create a new registry
- [x] 7.4 Update the TUI model's panels list and status bar after reload

## 8. Polish

- [x] 8.1 Add help overlay accessible via `?` from any view showing all keyboard shortcuts
- [x] 8.2 Mask API key display throughout — show "configured" or "••••" in settings list, never the raw key
- [x] 8.3 Show model hints in the wizard: display 2-3 popular model names for each provider type as placeholder text
