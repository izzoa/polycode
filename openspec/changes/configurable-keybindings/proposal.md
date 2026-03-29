## Why

All keybindings are hardcoded across `update.go`, `settings.go`, and other TUI handlers. Users cannot remap keys, and adding new bindings requires editing multiple switch/case blocks. A config-driven keybinding system centralizes key definitions, enables user customization, and supports a leader key for namespace-free shortcuts -- all without changing existing default behavior.

## What Changes

- Introduce a `keybindings:` section in config YAML mapping action names to key sequences
- Parse keybindings at config load time into a lookup structure
- Support an optional leader key with 500ms prefix timeout and visual indicator
- Expand `<leader>` prefix in bindings at parse time (e.g., `<leader>t` -> `Space t` if leader is Space)
- Default bindings match all current hardcoded behavior exactly
- Validate bindings on load: warn on conflicts (same key mapped to multiple actions)
- Replace hardcoded key checks in Update handlers with lookups against the binding map

## Capabilities

### New Capabilities
- `keybinding-config`: YAML-driven keybinding definitions, parsing, conflict detection, default bindings
- `leader-key`: Optional leader key with prefix timeout state and visual indicator

### Modified Capabilities
<!-- No existing spec-level changes -- defaults preserve current behavior -->

## Impact

- **Files created** (1): `internal/tui/keys.go`
- **Files modified** (3): `internal/config/config.go`, `internal/tui/model.go`, `internal/tui/update.go`
- **Dependencies**: None new
- **Config schema**: New optional `keybindings` section in config.yaml
- **Scope**: ~400-500 lines
