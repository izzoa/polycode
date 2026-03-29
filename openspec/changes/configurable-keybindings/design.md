## Context

Polycode's key handling is distributed across `updateChat`, `updateSettings`, `updateWizard`, and other mode-specific handlers in the TUI. Each handler has inline `case "ctrl+c":` style checks. There are roughly 30-40 distinct keybindings across modes. There is no way for users to customize them. The existing key handling uses Bubble Tea's `tea.KeyMsg` which provides key type and runes.

## Goals / Non-Goals

**Goals:**
- Single config-driven source of truth for all keybindings
- User customization via YAML (keybindings section)
- Leader key support with configurable prefix timeout
- Default bindings exactly match current hardcoded behavior
- Conflict detection on config load (warn, do not fail)
- Clean separation: `keys.go` owns definitions, Update handlers query by action name

**Non-Goals:**
- Per-mode binding overrides (bindings are global, context determines availability)
- Vim-style modal command sequences (beyond single leader prefix)
- Mouse button remapping
- Runtime keybinding editor UI (config YAML only for now)
- Chords beyond leader+key (no multi-modifier sequences)

## Decisions

### 1. Keybinding registry as a map[Action]Binding in keys.go

**Decision**: Define an `Action` string type and a `Binding` struct (`Key string, Description string`). A `KeyMap` struct holds `map[Action]Binding` plus a reverse lookup `map[string]Action` for efficient key matching. `keys.go` defines all defaults and the resolution logic.

**Rationale**: Named actions decouple key definitions from handler logic. Reverse lookup enables O(1) key-to-action resolution. The KeyMap lives on Model for access in all Update handlers.

### 2. Leader key as a prefix state machine

**Decision**: When the leader key is pressed, Model enters a `leaderArmed` state with a 500ms tick timer. During this window, the next keypress is matched as `<leader>+key`. If no key follows, leader state expires. A subtle indicator (e.g., "LEADER" in status bar) shows when armed.

**Rationale**: This is the standard leader key pattern (Vim, tmux). The 500ms timeout prevents accidental activation from stalling.

### 3. Config YAML merges with defaults

**Decision**: User-provided keybindings in YAML override defaults per-action. Unspecified actions keep their defaults. This is a merge, not a replacement.

**Rationale**: Users only need to specify the bindings they want to change. No risk of losing essential bindings by accident.

### 4. Conflict detection logs warnings

**Decision**: On config load, if two actions map to the same key, log a warning but do not prevent startup. The first binding wins.

**Rationale**: Hard failure would be frustrating for users experimenting. Warnings are surfaced in debug mode and could show as a toast (if toast system is available).

## Risks / Trade-offs

- **[Risk] Migrating all hardcoded checks is error-prone** -> Mitigation: Systematic file-by-file migration; test that all existing shortcuts still work with defaults.
- **[Risk] Leader key conflicts with Space input in textarea** -> Mitigation: Leader key only active when textarea is not focused (chat input, editors).
- **[Risk] Complex key representation across platforms** -> Mitigation: Use Bubble Tea's canonical key names (ctrl+c, alt+x, etc.) as the binding format.
