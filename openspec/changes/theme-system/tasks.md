## 1. Theme Core

- [x] 1.1 Create `internal/tui/theme.go` with Theme struct (~35 semantic color fields)
- [x] 1.2 Create `internal/tui/themes.go` with 6 built-in theme definitions (Polycode, Catppuccin Mocha, Tokyo Night, Dracula, Gruvbox Dark, Nord)
- [x] 1.3 Verify Polycode default theme matches all current hardcoded color values exactly
- [x] 1.4 Add `theme Theme` field to Model, initialize with default in NewModel
- [x] 1.5 Update `defaultStyles(theme Theme) Styles` to accept and derive from Theme

## 2. Config Persistence

- [x] 2.1 Add `Theme string` field to config.Config with YAML tag `theme`
- [x] 2.2 Load theme by name on startup in NewModel, fallback to default
- [x] 2.3 Save theme name to config on theme switch via ConfigChangedMsg

## 3. Color Migration — High-Density Files

- [x] 3.1 Replace all hardcoded colors in `view.go` (~47 calls) with theme references
- [x] 3.2 Replace all hardcoded colors in `settings.go` (~16 calls) with theme references
- [x] 3.3 Replace all hardcoded colors in `status_bar.go` with theme references
- [x] 3.4 Replace all hardcoded colors in `diff.go` with theme references
- [x] 3.5 Replace all hardcoded colors in `error_panel.go` with theme references

## 4. Color Migration — Remaining Files

- [x] 4.1 Replace hardcoded colors in `splash.go` (~4 calls)
- [x] 4.2 Replace hardcoded colors in `wizard.go` (~3 calls)
- [x] 4.3 Replace hardcoded colors in `mcp_wizard.go` (~1 call)
- [x] 4.4 Replace hardcoded colors in `timeline.go`
- [x] 4.5 Replace hardcoded colors in `session_picker.go`
- [x] 4.6 Replace hardcoded colors in `split.go`

## 5. Glamour Integration

- [x] 5.1 Create `rebuildMarkdownRenderer(theme Theme)` that builds glamour with theme-derived ansi.StyleConfig
- [x] 5.2 Call renderer rebuild on theme switch
- [x] 5.3 Verify markdown headings, links, code blocks, blockquotes use theme colors

## 6. Theme Picker

- [x] 6.1 Add theme picker state to Model (themePickerOpen, themePickerCursor, themePickerItems)
- [x] 6.2 Add `/theme` slash command and `Ctrl+T` keybinding to open picker
- [x] 6.3 Implement renderThemePicker overlay (list with cursor, current indicator, live preview)
- [x] 6.4 Implement theme selection: apply theme, rebuild styles, rebuild glamour, invalidate caches, persist to config
- [x] 6.5 Add picker to help overlay

## 7. Verification

- [x] 7.1 Grep for remaining `lipgloss.Color("` calls outside theme.go/themes.go — should be zero
- [x] 7.2 Run `go build ./...` and `go test ./...` — all pass
- [x] 7.3 Manual: verify default theme matches current visual appearance exactly
- [x] 7.4 Manual: switch between all 6 themes and verify no rendering artifacts
