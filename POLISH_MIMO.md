# Polycode UX/UI Polish Plan

> Competitive analysis of OpenCode, Crush, Claude Code, and Aider — mapped to actionable improvements for polycode's TUI.

---

## Table of Contents

1. [Competitive Landscape Summary](#1-competitive-landscape-summary)
2. [Current State Audit](#2-current-state-audit)
3. [Phase 1: Foundation](#3-phase-1-foundation)
4. [Phase 2: Visual Polish](#4-phase-2-visual-polish)
5. [Phase 3: Interaction Polish](#5-phase-3-interaction-polish)
6. [Phase 6: Differentiating Features](#6-phase-6-differentiating-features)
7. [Dependency Map](#7-dependency-map)
8. [Effort Estimates](#8-effort-estimates)

---

## 1. Competitive Landscape Summary

### OpenCode (sst/opencode)

- **Stack**: Go TUI, Bubble Tea v2, Lipgloss v2
- **Diff**: Dual modes — split (side-by-side) and unified, auto-switches at ≥160 cols
- **Themes**: 12+ built-in (Tokyo Night, Catppuccin, Dracula, Nord, Gruvbox, etc.) with semantic color keys
- **Key system**: Configurable leader key (`ctrl+x`) with JSON keybind config
- **Layout**: Chat + file viewer side-by-side at wide terminals, overlay below 160 cols
- **Dialogs**: Command palette, theme picker, session list, file finder, model/agent switcher
- **Toasts**: `info`/`success`/`warning`/`error` variants with configurable duration
- **Session management**: Server-side persistence, shareable URLs, `<leader>s` session picker
- **Agents**: Multiple agents (coder, planner), switchable via `<leader>a` or `tab`/`shift+tab`

### Crush (charmbracelet/crush)

- **Stack**: Go TUI, Bubble Tea v2, Lipgloss v2, Glamour v2, Ultraviolet screen buffer
- **Diff**: Custom `diffview` package — unified + split modes, Chroma syntax highlighting, xxh3-cached rendering, line numbers, `+`/`-` prefixes, edit tool uses full-width diffs inline
- **Spinner**: Custom hex-character cycling animation with gradient colors, per-message labels ("Generating", "Thinking", "Summarizing"), staggered character birth offsets
- **Thinking box**: Collapsible reasoning content (10 lines collapsed, expandable via space/click)
- **Dialog stack**: Overlay manager with push/pop — Models, Sessions, Commands, Permissions, FilePicker, OAuth, Reasoning Effort, Quit
- **Status bar**: Model info, provider, reasoning effort, context %, cost — `"model_name via provider_name"`
- **Styles**: Massive `Styles` struct with semantic tokens (`Subtle`, `Muted`, `Base`, `Primary`, `Secondary`) — zero hardcoded colors
- **Skills**: Extensible skill files (`SKILL.md`) injected into system prompt
- **Memory**: Reads `AGENTS.md`, `CLAUDE.md`, `GEMINI.md` from working dir, injected as `<memory>` block
- **Images**: Terminal image rendering via Kitty graphics protocol with block-based fallback
- **Scrolling**: Vim-style (`j`/`k`, `d`/`u`, `f`/`b`, `g`/`G`), `K`/`J` for item-level navigation
- **Clipboard**: `y` or `c` to copy message

### Claude Code (Anthropic)

- **Status line**: Persistent — context %, token count, model, cost
- **Diff**: Word-level inline diff, syntax-highlighted, resize-aware, `/rewind` to undo
- **Sessions**: Picker with branch filtering, AI-generated titles, auto-compaction, Ctrl+R transcript mode
- **Permissions**: Per-tool rules (`Bash(python:*)`), hooks API for programmatic control
- **Todo list**: Dynamic sizing based on terminal height, visual task tracking
- **@-mentions**: Typeahead file autocomplete
- **Key features**: Vim mode (`/vim`), Ctrl+B background commands, real-time steering (send while working), Enter to queue, Shift+Tab auto-accept toggle

### Aider

- **Edit formats**: Model-adaptive — picks `diff`, `udiff`, `whole` per model capability
- **Git integration**: Auto-commits changes, `/undo` for revert, `/diff` invokes system git diff
- **Repo map**: Auto-generated code structure via tree-sitter for context
- **Watch mode**: Monitors files for `# AI!` comment triggers
- **Architect mode**: Separate planning vs editing models

---

## 2. Current State Audit

### What Polycode Has

| Feature | Status | Details |
|---------|--------|---------|
| Markdown rendering | ✅ | Glamour with auto-style, throttled during streaming |
| Spinner | ✅ | `bubbles/spinner` dot style — used everywhere |
| Tab bar | ✅ | Per-provider tabs with status icons, token/cost display |
| Overlays | ✅ | Splash, help, MCP dashboard, command palette, mode picker, confirm prompt |
| Viewports | ✅ | Separate viewports for chat, consensus, each provider panel |
| Command palette | ✅ | Slash-command autocomplete, fuzzy filter, 30+ commands |
| Mouse capture | ✅ | `tea.WithMouseCellMotion()` enabled — but no custom handlers |
| Clipboard dep | ✅ | `atotto/clipboard` in go.mod (transitive) — not used directly |
| Input history | ✅ | Up/down cycling with draft preservation |
| Consensus provenance | ✅ | Toggleable panel with confidence, agreements, minorities |
| Session infrastructure | ✅ | `/save`, `/sessions list/show/delete/name`, history rebuild |
| MCP dashboard | ✅ | Full overlay table with server status, tools, reconnect |

### What Polycode Is Missing

| Feature | Impact | Gap Description |
|---------|--------|-----------------|
| Diff rendering | 🔴 High | No colorized diffs — `unifiedDiff()` in action/ produces plain text only |
| Theme system | 🔴 High | Colors hardcoded as ANSI indices — no user config, no dark/light toggle |
| Syntax highlighting | 🟡 Med | Glamour handles markdown code blocks, but no standalone Chroma usage for tool output or file previews |
| Toast notifications | 🟡 Med | No global transient notification mechanism |
| Status bar | 🟡 Med | No persistent bottom bar showing context health, cost, model |
| Collapsible thinking | 🟡 Med | Reasoning/trace text shown inline, no collapse/expand |
| @-file mentions | 🟡 Med | No typeahead file reference in input |
| Split panes | 🟠 Low | Single-panel — can't view consensus + provider simultaneously |
| Session picker UI | 🟠 Low | CLI commands exist but no fzf-style visual browser |
| Copy-to-clipboard | 🟠 Low | No keybinding for copying response/code blocks |
| Configurable keys | 🟠 Low | All keybindings hardcoded |
| Error recovery UI | 🟠 Low | Raw `[ERROR: ...]` text, no retry, no expandable details |
| Vim scroll keys | 🟠 Low | Only PgUp/PgDn/Home/End — no j/k/d/u/g/G |
| Progress bars | 🟠 Low | Only spinner — no determinate progress for token limits or pipeline stages |

---

## 3. Phase 1: Foundation

*Architectural prerequisites that other features depend on.*

### 3.1 Theme System

**Why first**: Every visual feature (diffs, toasts, status bar) needs semantic colors. Fixing the color architecture first prevents rework.

**Approach**:
- Define a `Theme` struct in `internal/tui/theme.go` with semantic keys:
  ```go
  type Theme struct {
      Name           string
      Primary        lipgloss.TerminalColor  // accent, titles, highlights
      Secondary      lipgloss.TerminalColor  // prompt prefix, input border
      Success        lipgloss.TerminalColor  // healthy status, completion
      Error          lipgloss.TerminalColor  // failures, delete confirm
      Warning        lipgloss.TerminalColor  // caution, timeouts
      Text           lipgloss.TerminalColor  // primary text
      TextMuted      lipgloss.TerminalColor  // dimmed, secondary text
      TextBright     lipgloss.TerminalColor  // highlights, selected items
      Background     lipgloss.TerminalColor  // status bar, panels
      BackgroundAlt  lipgloss.TerminalColor  // selected rows, active tabs
      Border         lipgloss.TerminalColor  // panel borders
      BorderFocus    lipgloss.TerminalColor  // focused panel borders
      DiffAdd        lipgloss.TerminalColor  // diff insertions
      DiffDelete     lipgloss.TerminalColor  // diff deletions
      DiffContext     lipgloss.TerminalColor  // diff context
      SyntaxKeyword  lipgloss.TerminalColor  // code highlighting
      SyntaxString   lipgloss.TerminalColor
      SyntaxComment  lipgloss.TerminalColor
      SyntaxFunc     lipgloss.TerminalColor
      SyntaxType     lipgloss.TerminalColor
      SyntaxNumber   lipgloss.TerminalColor
  }
  ```
- Ship 4 built-in themes: `polycode` (current orange/purple), `tokyo-night`, `catppuccin-mocha`, `dracula`
- Store active theme name in `config.Config.Theme string` with YAML tag
- Replace `defaultStyles()` to accept a `*Theme` and derive all lipgloss styles from it
- Add `/theme` slash command → opens a mode-picker-style overlay listing available themes with live preview
- Config persistence: `theme: tokyo-night` in `~/.config/polycode/config.yaml`

**Files to create/modify**:
- `internal/tui/theme.go` (new) — Theme struct, built-in themes, `LoadTheme(name string) Theme`
- `internal/tui/model.go` — `Styles` struct derived from `Theme`, `defaultStyles(theme)` signature change
- `internal/tui/view.go` — replace all hardcoded `lipgloss.Color("214")` etc. with `m.styles.XXX`
- `internal/tui/update.go` — `/theme` command handler, theme overlay rendering
- `internal/config/config.go` — add `Theme` field, YAML tag, validation

### 3.2 Toast Notification System

**Why second**: Many subsequent features (config saved, theme applied, MCP connected, clipboard copied) need a non-blocking feedback mechanism.

**Approach**:
- Create `internal/tui/toast.go` with:
  ```go
  type Toast struct {
      Message   string
      Variant   ToastVariant  // Info, Success, Warning, Error
      Duration  time.Duration // auto-dismiss, 0 = manual
      CreatedAt time.Time
  }
  type ToastVariant int
  const (
      ToastInfo ToastVariant = iota
      ToastSuccess
      ToastWarning
      ToastError
  )
  ```
- Add `toasts []Toast` field to Model
- `addToast(msg string, variant ToastVariant, duration time.Duration)` method
- `ToastTickMsg` message sent on timer expiry to remove stale toasts
- Render as bottom-right anchored overlay (stacked if multiple, max 3 visible)
- Style: left border colored by variant (blue/green/yellow/red), subtle background, fade-out animation via decreasing opacity (lipgloss doesn't support alpha, so use color dimming across tick intervals)
- Wire into existing events: config save → success toast, MCP reconnect → info toast, test failure → error toast

**Files to create/modify**:
- `internal/tui/toast.go` (new) — Toast struct, render function, tick handler
- `internal/tui/model.go` — add `toasts []Toast` field, `ToastTickMsg`
- `internal/tui/update.go` — toast tick handler, `addToast` callsites
- `internal/tui/view.go` — `renderToasts()` in `renderChat()` composition (after confirm prompt, before input)

---

## 4. Phase 2: Visual Polish

*Features that make the TUI look and feel professional.*

### 4.1 Syntax-Highlighted Diff View

**Context**: Polycode's `action/file_ops.go` has a `unifiedDiff()` function that produces plain-text unified diffs. These are shown during tool confirmation prompts as raw text with `+`/`-` prefixes. No color, no syntax highlighting, no side-by-side mode.

**Approach**:
- Add `github.com/alecthomas/chroma/v2` as a direct dependency (glamour already uses it internally)
- Create `internal/tui/diff.go` (new):
  ```go
  type DiffStyle int
  const (
      DiffUnified DiffStyle = iota
      DiffSplit
  )

  func RenderDiff(oldContent, newContent, filename string, width int, style DiffStyle, theme Theme) string
  func renderUnifiedDiff(diffs []diffLine, width int, theme Theme) string
  func renderSplitDiff(oldContent, newContent string, width int, theme Theme) string
  func highlightSyntax(content, language string, theme Theme) string
  ```
- Diff algorithm: use `github.com/sourcegraph/go-diff` or keep the existing line-by-line approach but enhance rendering
- Chroma language detection from filename extension
- Cache syntax-highlighted output keyed by content hash + theme (like Crush's xxh3 approach)
- Auto-switch: unified at <120 cols, split at ≥120 cols (OpenCode uses 160, but that's aggressive)
- Toggle via `d` key in confirm prompt, persisted in `m.diffStyle`
- Apply to: tool call confirmation prompts, `/diff` command (show staged changes), edit tool results in chat history

**Files to create/modify**:
- `internal/tui/diff.go` (new) — diff rendering with Chroma highlighting
- `internal/tui/model.go` — add `diffStyle DiffStyle` field
- `internal/tui/update.go` — `d` key toggle in confirm handler
- `internal/tui/view.go` — update `renderConfirmPrompt` to use colored diffs
- `go.mod` — add chroma v2 direct dependency

### 4.2 Enhanced Spinner / Thinking Indicator

**Context**: Polycode uses `bubbles/spinner` (dot style) uniformly. Crush has a custom hex-character spinner with gradient colors and per-message labels.

**Approach**:
- Create `internal/tui/spinner.go` (new) with a custom spinner:
  - Character set: `⠋⠙⠹⠸⠼⠴⠦⠧⠇` (braille, more visually interesting than dot)
  - Phase-aware labels: "Dispatching", "Thinking", "Synthesizing", "Executing tools", "Verifying"
  - Color: cycle through theme's Primary → Secondary gradient (2-3 colors)
  - Model-aware: show model name during fan-out phase ("Sonnet thinking...", "GPT-4o thinking...")
- Replace `spinner.Model` usage throughout with the custom spinner
- Add to tab bar: show per-provider label (not just spinner icon)

**Files to create/modify**:
- `internal/tui/spinner.go` (new) — custom spinner with labels and color cycling
- `internal/tui/model.go` — replace `spinner.Model` with custom type
- `internal/tui/update.go` — update spinner tick handling
- `internal/tui/view.go` — update tab bar and worker progress rendering

### 4.3 Status Bar (Bottom)

**Context**: No persistent bottom bar. Claude Code shows context %, tokens, model, cost at all times. Crush shows model info, reasoning effort, context usage.

**Approach**:
- Add a persistent bottom status bar rendered in `renderChat()` after input:
  ```
  [Context: 42%] [Tokens: 12.4K/200K] [$0.38] [claude-sonnet via Anthropic] [balanced mode]
  ```
- Context percentage: color-coded (green <50%, yellow 50-80%, red >80%)
- Compact format — single line, right-aligned info, left-aligned mode
- Subtle styling: dimmed text, thin top border, no background (or very dark)
- Replace the current token display in tab bar with this unified bar
- Update on every `TokenUpdateMsg` and `ModeChangedMsg`

**Files to create/modify**:
- `internal/tui/view.go` — add `renderStatusBar()` function, wire into `renderChat()`
- `internal/tui/model.go` — add computed fields for aggregate token/cost/context

### 4.4 Collapsible Thinking / Reasoning Box

**Context**: During synthesis, polycode shows trace sections (`── Fan-out ──`, `── Synthesis ──`) inline in the consensus panel. These consume vertical space and are rarely useful after the query completes.

**Approach**:
- Add `expandedTrace bool` field to Model (per-exchange in history)
- Render trace sections inside a bordered, collapsible container:
  - Collapsed: single line with `▶ Trace: 3 sections, 47 lines` (dimmed)
  - Expanded: full content with `▼ Trace:` header
  - Toggle with `t` key when chat viewport is focused (not input)
- During active streaming: always expanded
- After query completes: auto-collapse, save to Exchange

**Files to create/modify**:
- `internal/tui/model.go` — add `expandedTrace bool`, add to `Exchange`
- `internal/tui/update.go` — `t` key handler for trace toggle
- `internal/tui/view.go` — update `renderChatPanel()` for collapsible trace

---

## 5. Phase 3: Interaction Polish

*Features that make the TUI feel responsive and intuitive.*

### 5.1 Vim-Style Scroll Keys

**Context**: Polycode has PgUp/PgDn/Home/End and Ctrl+U/Ctrl+D. Crush, Claude Code, and most modern TUIs support vim navigation.

**Approach**:
- In `updateChat()`, when input is empty (or tab bar focused), add:
  - `j` → scroll down 1 line
  - `k` → scroll up 1 line
  - `d` → half-page down (like Ctrl+D)
  - `u` → half-page up (like Ctrl+U)
  - `g` → jump to top (like Home)
  - `G` → jump to bottom (like End)
- Only active when input textarea is empty to avoid conflicting with text entry
- Reuse existing `chatScrollBy()` infrastructure

**Files to modify**:
- `internal/tui/update.go` — add vim key cases in `updateChat()` with empty-input guard

### 5.2 Copy-to-Clipboard

**Context**: `atotto/clipboard` is already in go.mod as transitive dependency. No copy functionality exists.

**Approach**:
- Add `y` keybinding in chat mode (when input empty): copies last assistant response to clipboard
- Add `/copy` slash command: copies the current consensus response (or selected provider's response)
- Visual feedback: toast notification "Copied to clipboard" (requires toast system)
- Support copying code blocks specifically: if cursor is within a code block in the rendered view, copy just that block

**Files to create/modify**:
- `internal/tui/update.go` — `y` key handler, `/copy` command handler
- `internal/tui/clipboard.go` (new) — `copyToClipboard(content string) error` using `atotto/clipboard`
- `internal/tui/model.go` — track last assistant response for quick copy

### 5.3 Enhanced Mouse Support

**Context**: `tea.WithMouseCellMotion()` is enabled but no custom mouse events are handled.

**Approach**:
- Handle `tea.MouseMsg` in `updateChat()`:
  - Click on tab bar → switch tab
  - Click on chat area → focus chat viewport
  - Click on input → focus textarea
  - Mouse wheel → scroll active viewport
- Handle in overlays:
  - Click on command palette item → select it
  - Click on mode picker item → select it
  - Click on MCP dashboard row → select it
- Visual feedback: cursor change on clickable areas (not possible in all terminals, but focus change provides feedback)

**Files to modify**:
- `internal/tui/update.go` — add `tea.MouseMsg` handling in `Update()` and mode-specific handlers
- `internal/tui/view.go` — track clickable regions if needed

### 5.4 Session Picker Overlay

**Context**: Polycode has `/sessions list/show/delete/name` CLI commands and session persistence, but no visual session browser.

**Approach**:
- Create `internal/tui/session_picker.go` (new):
  - Full-screen overlay similar to MCP dashboard
  - List of saved sessions: name, date, message count, active providers
  - Fuzzy search/filter at top
  - `j`/`k` navigation, `Enter` to load, `d` to delete (with confirm), `n` to rename
  - `Esc` to dismiss
- Trigger via `Ctrl+S` (currently opens settings — rebind settings to `/settings` or `Ctrl+E`)
- Wire into existing session callbacks: `onSessionsList`, `onSessionLoad`

**Files to create/modify**:
- `internal/tui/session_picker.go` (new) — session picker overlay
- `internal/tui/model.go` — add `showSessionPicker bool`, `sessionPickerItems`, `sessionPickerCursor`
- `internal/tui/update.go` — `Ctrl+S` keybinding, session picker key handling
- `internal/tui/view.go` — `renderSessionPicker()` in View() priority chain

### 5.5 @-Mention File Autocomplete

**Context**: No way to reference files in prompts. Claude Code's `@` typeahead is the gold standard.

**Approach**:
- When user types `@` in the textarea, activate a file autocomplete popup:
  - Fuzzy match against project files (use existing `find_files` tool logic)
  - Show matching files in a dropdown below the input
  - `Tab`/`Enter` to complete, `Esc` to dismiss
  - Completed reference becomes `@filepath` in the input
- On submit, expand `@filepath` references into file content (or path for the LLM)
- Render file references with a distinct style (cyan/underlined) in the input display

**Files to create/modify**:
- `internal/tui/file_complete.go` (new) — file matching, dropdown rendering
- `internal/tui/model.go` — add `fileCompleteActive bool`, `fileCompleteItems`, `fileCompleteCursor`
- `internal/tui/update.go` — `@` detection in textarea, completion handling
- `internal/tui/view.go` — `renderFileComplete()` overlay in input area

---

## 6. Phase 4: Differentiating Features

*Features that leverage polycode's unique multi-model architecture.*

### 6.1 Split Pane Layout

**Context**: Polycode is single-panel. You can view consensus OR a provider, not both. OpenCode shows chat + file viewer side-by-side at wide terminals.

**Approach**:
- When terminal width ≥ 140 cols:
  - Left panel (60%): consensus/chat view
  - Right panel (40%): selected provider's response, or diff view, or file preview
  - Resizable with `Ctrl+←`/`Ctrl+→`
- When terminal width < 140 cols: current single-panel behavior
- Right panel content toggles: `1` = provider response, `2` = diff view, `3` = file preview, `Esc` = hide
- Each panel has its own viewport with independent scroll

**Files to modify**:
- `internal/tui/model.go` — add `splitPane bool`, `splitRatio float64`, `rightPanelContent enum`
- `internal/tui/update.go` — resize keybindings, panel content toggle keys
- `internal/tui/view.go` — horizontal split layout in `renderChat()`, update `updateLayout()` for split dimensions

### 6.2 Configurable Keybindings

**Context**: All keys hardcoded. OpenCode uses JSON config with a leader key system.

**Approach**:
- Add `Keybindings` struct to config:
  ```yaml
  keybindings:
    leader: "ctrl+x"
    quit: "ctrl+c,ctrl+d,<leader>q"
    settings: "<leader>e"
    sessions: "<leader>s"
    theme: "<leader>t"
    help: "?"
    # ... all configurable
  ```
- Parse `<leader>` prefix at load time, expand to actual key sequence
- Default bindings match current behavior (backward compatible)
- Validation: warn on conflicts, reject unparseable sequences

**Files to create/modify**:
- `internal/config/config.go` — add `Keybindings` struct, YAML tags, defaults
- `internal/tui/keys.go` (new) — keybinding resolver, `<leader>` expansion, conflict detection
- `internal/tui/update.go` — replace hardcoded key checks with resolved bindings

### 6.3 Error Recovery UI

**Context**: Errors render as raw `[ERROR: ...]` text inline. No retry, no expandable details.

**Approach**:
- Create `internal/tui/error_panel.go` (new):
  - Red-bordered collapsible panel for errors
  - Header: `✕ Error: <summary>` (truncated to 1 line)
  - Expanded: full error message, stack trace if available, timestamp
  - Actions: `r` to retry last query, `c` to copy error to clipboard
- Replace inline error text in consensus panel with error panel component
- Persist errors in Exchange history for review

**Files to create/modify**:
- `internal/tui/error_panel.go` (new) — error panel component
- `internal/tui/model.go` — add `lastError` field, `errors []ErrorInfo` in Exchange
- `internal/tui/update.go` — `r` key for retry, error panel expand/collapse
- `internal/tui/view.go` — replace raw error rendering with error panel

### 6.4 Progress Bars for Token Usage

**Context**: Only spinner exists. No visual indicator of context window filling up or approaching limits.

**Approach**:
- In the status bar (Phase 2.3), render a thin progress bar:
  ```
  ████████████░░░░░░░░ 42% (84K/200K tokens)
  ```
- Color: green → yellow → red as usage increases
- Per-provider mini-bars in the MCP dashboard or provider tabs
- Tooltip/expand: press `p` on status bar to see per-turn breakdown

**Files to modify**:
- `internal/tui/view.go` — progress bar rendering in `renderStatusBar()`
- `internal/tui/model.go` — aggregate context calculation helpers

---

## 7. Dependency Map

```
Phase 1: Foundation
├── 3.1 Theme System ──────────────────────────── blocks ──┐
├── 3.2 Toast System ──────────────────────────── blocks ──┤
│                                                           │
Phase 2: Visual Polish                                      │
├── 4.1 Diff View ◄────────────────────────────── requires ─┤ (theme + toast)
├── 4.2 Enhanced Spinner ◄─────────────────────── requires ─┤ (theme)
├── 4.3 Status Bar ◄───────────────────────────── requires ─┤ (theme)
├── 4.4 Collapsible Thinking ◄─────────────────── requires ─┤ (theme)
│                                                           │
Phase 3: Interaction                                        │
├── 5.1 Vim Scroll Keys                                    │ (independent)
├── 5.2 Copy-to-Clipboard ◄────────────────────── requires ─┘ (toast)
├── 5.3 Enhanced Mouse Support                             (independent)
├── 5.4 Session Picker ◄───────────────────────── requires ── (theme)
├── 5.5 @-Mention Autocomplete                             (independent)
│
Phase 4: Differentiating
├── 6.1 Split Panes ◄──────────────────────────── requires ── (theme)
├── 6.2 Configurable Keys ◄────────────────────── requires ── (theme)
├── 6.3 Error Recovery UI ◄────────────────────── requires ── (theme + toast)
└── 6.4 Token Progress Bars ◄──────────────────── requires ── (status bar)
```

### Critical Path

1. Theme System (3.1) → blocks everything
2. Toast System (3.2) → blocks diffs, clipboard, errors
3. Diff View (4.1) → biggest visual gap
4. Status Bar (4.3) → enables progress bars
5. Everything else can proceed in parallel

---

## 8. Effort Estimates

| # | Feature | Effort | Priority | Complexity |
|---|---------|--------|----------|------------|
| 3.1 | Theme System | 2-3 days | 🔴 P0 | Medium — refactor existing styles, create theme struct, 4 themes |
| 3.2 | Toast System | 1 day | 🔴 P0 | Low — new component, timer-based dismiss, wired to existing events |
| 4.1 | Diff View | 3-4 days | 🔴 P0 | High — Chroma integration, diff algorithm, split/unified modes, caching |
| 4.2 | Enhanced Spinner | 0.5 day | 🟡 P1 | Low — custom character set, phase labels, color cycling |
| 4.3 | Status Bar | 1 day | 🟡 P1 | Low — new render function, aggregate calculations |
| 4.4 | Collapsible Thinking | 1 day | 🟡 P1 | Low — toggle state, conditional rendering |
| 5.1 | Vim Scroll Keys | 0.5 day | 🟡 P1 | Low — add key cases with guards |
| 5.2 | Copy-to-Clipboard | 0.5 day | 🟡 P1 | Low — clipboard binding, toast feedback |
| 5.3 | Enhanced Mouse | 1-2 days | 🟠 P2 | Medium — mouse event routing, click region tracking |
| 5.4 | Session Picker | 2-3 days | 🟠 P2 | Medium — new overlay, session data wiring, search/filter |
| 5.5 | @-Mention Autocomplete | 2-3 days | 🟠 P2 | Medium — file indexing, fuzzy match, dropdown, input integration |
| 6.1 | Split Panes | 2-3 days | 🟠 P2 | Medium — layout math, dual viewport, resize handling |
| 6.2 | Configurable Keys | 2-3 days | 🟠 P2 | Medium — config struct, key parser, replacement of all key checks |
| 6.3 | Error Recovery UI | 1-2 days | 🟠 P2 | Low-Medium — error panel component, retry wiring |
| 6.4 | Token Progress Bars | 0.5 day | 🟡 P1 | Low — bar rendering in status bar |

**Total estimated effort: 18-27 days**

### Recommended Sprint Plan

**Sprint 1 (Foundation): 3.1 + 3.2 + 4.2 = ~4 days**
Theme system, toast system, enhanced spinner. Unblocks everything else.

**Sprint 2 (Core Visual): 4.1 + 4.3 + 4.4 + 5.1 + 5.2 = ~6 days**
Diff view (the big one), status bar, collapsible thinking, vim keys, clipboard.

**Sprint 3 (Interaction): 5.3 + 5.4 + 5.5 = ~5-7 days**
Mouse support, session picker, @-mentions.

**Sprint 4 (Differentiators): 6.1 + 6.2 + 6.3 + 6.4 = ~6-8 days**
Split panes, configurable keys, error recovery, progress bars.

---

## Appendix: Key Binding Comparison

| Action | Polycode | OpenCode | Crush | Claude Code |
|--------|----------|----------|-------|-------------|
| Quit | `Ctrl+C` | `Ctrl+C` / `<leader>q` | `Ctrl+C` | `Ctrl+C` |
| Settings | `Ctrl+S` | `<leader>e` | — | — |
| Sessions | `/sessions` | `<leader>s` | `Ctrl+S` | `--resume` |
| Theme | — | `<leader>t` | — | `/theme` |
| Help | `?` | `<leader>?` | `Ctrl+G` | — |
| File list | — | `<leader>f` | `Ctrl+F` | `@` typeahead |
| Model list | — | `<leader>m` | — | — |
| New session | — | `<leader>n` | `Ctrl+N` | — |
| Copy response | — | — | `y` / `c` | — |
| Scroll down | `PgDn` | — | `j` | — |
| Scroll up | `PgUp` | — | `k` | — |
| Half page down | `Ctrl+D` | — | `d` | — |
| Half page up | `Ctrl+U` | — | `u` | — |
| Top | `Home` | — | `g` | — |
| Bottom | `End` | — | `G` | — |
| Command palette | `/` prefix | `Ctrl+P` | `Ctrl+P` | — |
| Diff toggle | — | — | — | — |
| Background cmd | — | — | — | `Ctrl+B` |
| Auto-accept | — | — | — | `Shift+Tab` |
| External editor | — | `<leader>e` | `Ctrl+O` | `Ctrl+G` |
