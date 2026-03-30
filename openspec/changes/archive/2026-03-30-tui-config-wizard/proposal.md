## Why

Configuring polycode currently requires hand-editing `~/.config/polycode/config.yaml` — users must know the exact provider type strings, auth methods, and model names. The existing `polycode init` wizard is a bare stdin prompt that only sets up one provider and exits. Users should be able to add, edit, remove, and test providers from within the TUI itself, without ever touching YAML. This is especially important for onboarding: a new user should be able to launch `polycode`, configure their providers through a guided flow, and start querying — all within the same session.

## What Changes

- **In-TUI settings screen**: A new view accessible via a keyboard shortcut (e.g., `:` or `Ctrl+S`) that lists all configured providers and allows CRUD operations
- **Add provider wizard**: A step-by-step form within the TUI to add a new provider (select type → enter name → choose auth method → enter API key or start OAuth → select model → set as primary?)
- **Edit provider**: Modify an existing provider's settings (model, auth, primary designation, max_context)
- **Remove provider**: Delete a provider with confirmation
- **Test connection**: Validate a provider by sending a lightweight test query and displaying success/failure
- **Live reload**: Changes made in the TUI settings are immediately applied to the running session without restarting — the config is saved to YAML and the provider registry is refreshed
- **Keyboard shortcut overlay**: A help popup showing all available shortcuts, including how to access settings

## Capabilities

### New Capabilities
- `tui-settings`: In-TUI provider configuration screen with add/edit/remove/test operations and live config reload

### Modified Capabilities
_(none — YAML configuration continues to work exactly as before; this adds a parallel TUI-based path)_

## Impact

- **`internal/tui/`**: New settings view state, form components, provider list view
- **`internal/config/`**: Config needs a `Reload()` or `Save()` + re-initialize flow
- **`internal/provider/`**: Registry needs a `Refresh()` method to re-initialize from updated config
- **`cmd/polycode/app.go`**: Wire settings actions to registry refresh and pipeline rebuild
