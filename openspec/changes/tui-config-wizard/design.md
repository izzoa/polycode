## Context

Polycode's config is a YAML file at `~/.config/polycode/config.yaml`. The existing `polycode init` command is a basic stdin-based wizard that creates a minimal config with one provider. Once in the TUI, there's no way to modify configuration — users must quit, edit the YAML, and relaunch. This is friction for onboarding and for iterating on provider setups.

The TUI is built with Bubble Tea (Elm architecture: Model/Update/View). Adding a settings screen means introducing a new "view mode" alongside the existing chat view and splash screen.

## Goals / Non-Goals

**Goals:**
- Full provider CRUD (add, edit, remove) within the TUI
- Step-by-step add-provider wizard with type selection, auth, model entry
- Connection testing from within settings
- Live reload — changes take effect immediately without restarting
- Persist changes to the YAML config file
- Accessible via keyboard shortcut from the main chat view

**Non-Goals:**
- Replacing YAML config entirely (power users can still edit YAML directly)
- Editing consensus or TUI settings from the settings screen (v1 is provider management only)
- A mouse-driven GUI or clickable UI (keyboard-only, consistent with the TUI paradigm)
- Model browsing/search (user types model name; litellm metadata validates it)

## Decisions

### 1. View mode architecture: enum-based view state

**Choice**: Add a `viewMode` enum to the TUI Model with values `viewChat`, `viewSettings`, `viewAddProvider`, `viewEditProvider`. The `View()` method dispatches to the appropriate render function based on the current mode. `Update()` routes key events based on mode.

**Rationale**: This is the standard Bubble Tea pattern for multi-screen apps. Clean separation, no nested models needed for v1.

**Alternatives considered**:
- **Nested Bubble Tea programs**: Over-engineered for a settings screen
- **Overlay/modal**: Harder to implement in Bubble Tea; settings has enough content to warrant a full screen

### 2. Add-provider flow: multi-step form with Bubbles components

**Choice**: Use `bubbles/textinput` for text fields and a simple list selector for enum fields (type, auth method). The flow is:

1. **Select type** → list: anthropic, openai, google, openai_compatible
2. **Enter name** → text input (auto-suggested from type, e.g., "claude")
3. **Select auth** → list: api_key, oauth, none (filtered by type)
4. **Enter API key** → masked text input (if api_key selected)
5. **Enter model** → text input (with hint showing popular models for the type)
6. **Enter base URL** → text input (only for openai_compatible)
7. **Set as primary?** → y/n
8. **Test connection** → optional, sends a minimal query
9. **Confirm** → saves to config, refreshes registry

Each step is a `formStep` with a prompt, input component, and validation function.

**Rationale**: Step-by-step is less overwhelming than a single form with many fields. Users only see fields relevant to their chosen provider type.

### 3. Live reload: save config + rebuild registry + rebuild pipeline

**Choice**: After any settings change:
1. Call `config.Save()` to persist to YAML
2. Create a new `provider.Registry` from the updated config
3. Authenticate + validate new providers
4. Rebuild the consensus `Pipeline` with the new registry
5. Update the TUI model's provider panels

This is done via a `ConfigChangedMsg` sent through the Bubble Tea program.

**Rationale**: Rebuilding the registry and pipeline is cheap (no persistent connections to tear down). It's simpler than trying to hot-patch individual providers.

### 4. Settings list: table-style provider display

**Choice**: The settings screen shows a table of providers with columns: name, type, model, auth, primary, status. Navigation with j/k or arrow keys. Actions: `a` (add), `e` (edit), `d` (delete), `t` (test), `Esc` (back to chat).

**Rationale**: Familiar pattern from tools like lazygit, k9s. Dense information display with clear action keys.

### 5. Entry point: Ctrl+S shortcut

**Choice**: `Ctrl+S` from the chat view opens settings. `Esc` returns to chat. Also accessible via typing `/settings` in the input.

**Rationale**: `Ctrl+S` is memorable ("S for settings"). The `/settings` command provides discoverability.

## Risks / Trade-offs

- **Form complexity in Bubble Tea**: Multi-step forms with conditional fields are verbose in the Elm architecture. → **Mitigation**: Keep each step simple — one input per screen. Use a step counter and switch statement.

- **API key handling in TUI**: Displaying API keys on screen is a security concern. → **Mitigation**: Use masked input (show `••••••` while typing). Never display stored keys in the settings list — show "configured" or "not set".

- **Config file conflicts**: If user edits YAML while TUI is running, the TUI save could overwrite their changes. → **Mitigation**: Document that TUI settings and YAML editing shouldn't happen simultaneously. Future: file watcher for external changes.

- **Registry rebuild during active query**: If settings change while a query is in flight, the old pipeline continues with stale providers. → **Mitigation**: Block settings changes while querying (show "cannot modify settings during active query").
