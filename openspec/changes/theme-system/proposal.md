## Why

The TUI has ~85 hardcoded `lipgloss.Color()` calls spread across 15 files. Every new visual feature (diffs, toasts, status bars, overlays) requires picking raw color numbers, and there is no way for users to customize the palette. This is the single largest foundation blocker identified in the UX polish roadmap — fixing the color architecture first prevents refactoring every file twice as new visual work lands.

## What Changes

- Introduce a centralized `Theme` struct with ~35 semantic color slots (Primary, Secondary, Text, BgPanel, DiffAdded, etc.)
- Ship 6 built-in themes as struct literals: Polycode (default, preserves current palette), Catppuccin Mocha, Tokyo Night, Dracula, Gruvbox Dark, Nord
- Replace every hardcoded `lipgloss.Color("214")` etc. with `m.theme.Primary` across all TUI files
- `defaultStyles()` accepts a `Theme` and derives all `Styles` from it
- Add theme picker overlay (`Ctrl+T` or `/theme`) with live preview swatch
- Persist selected theme to config YAML (`theme: "catppuccin"`)
- Pass theme colors to glamour's `ansi.StyleConfig` so markdown rendering matches the active theme

## Capabilities

### New Capabilities
- `theme-engine`: Core Theme struct, built-in theme definitions, theme loading/selection, config persistence
- `theme-picker`: Interactive theme picker overlay with live preview and keyboard navigation
- `theme-integration`: Replacement of all hardcoded colors across TUI files with theme references, glamour style override

### Modified Capabilities
<!-- No existing spec-level behavior changes — this is additive infrastructure -->

## Impact

- **Files modified** (~15): `model.go`, `view.go`, `settings.go`, `splash.go`, `wizard.go`, `mcp_wizard.go`, `diff.go`, `error_panel.go`, `status_bar.go`, `timeline.go`, `session_picker.go`, `markdown.go`, `config/config.go`
- **Files created** (2): `theme.go`, `themes.go`
- **Dependencies**: None new (glamour already uses chroma internally)
- **Config schema**: New optional `theme` field in `config.yaml`
- **Scope**: ~800-1200 lines changed
