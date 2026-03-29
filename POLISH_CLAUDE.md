# POLISH_CLAUDE.md — UX/UI Polish Roadmap

Inspired by features from **Crush** (Charmbracelet's terminal coding agent, formerly OpenCode) and **OpenCode** (both Go and TypeScript versions). This plan targets concrete improvements to polycode's Bubble Tea TUI, ordered by impact and feasibility.

---

## Current State Assessment

### What polycode already does well (differentiators)
- **Multi-model consensus** — unique; neither Crush nor OpenCode supports parallel multi-model querying with synthesis
- **Provenance panel** — confidence levels, agreement/minority reports, evidence citations
- **Per-provider trace tabs** — phase-aware activity logs (fanout, synthesis, tool, verify)
- **MCP dashboard** — richer server/tool inspection than Crush
- **Session persistence** — auto-save/resume per working directory (already implemented)
- **Command palette** — slash commands with filtering, tab-accept, subcommand hiding

### Where polycode falls behind
- **85+ hardcoded `lipgloss.Color()` calls** scattered across view.go (47), settings.go (16), wizard.go (3), mcp_wizard.go (1), splash.go (4) — only 12 colors go through the `Styles` struct
- **No theme switching** — single fixed 256-color palette
- **No inline file references** — no `@` file picker for attaching context
- **No fuzzy search** — command palette uses substring matching only
- **No tool call collapse/conceal** — every tool call is always fully visible
- **No desktop notifications** — no awareness of terminal focus state
- **No auto-compact** — no context overflow protection
- **Fixed textarea height** — always 3 lines regardless of content
- **No diff visualization** — tool results shown as raw text, no git-style coloring

---

## Phase 1: Theme System

**Goal**: Extract all hardcoded colors into a semantic theme, add 5-8 switchable themes, add runtime theme picker.

**Why first**: Highest visual impact, touches every screen, and forces the color consolidation that all other visual improvements depend on.

### 1.1 Define Theme Interface

Create `internal/tui/theme.go`:

```go
type Theme struct {
    Name        string
    // Base palette
    Primary     lipgloss.Color // main accent (currently 214 orange)
    Secondary   lipgloss.Color // interactive elements (currently 63 blue)
    Tertiary    lipgloss.Color // active/selected (currently 86 cyan)
    Success     lipgloss.Color // healthy/done (currently 42 green)
    Error       lipgloss.Color // failure/error (currently 196 red)
    Warning     lipgloss.Color // attention (currently 214 orange)
    Info        lipgloss.Color // informational (currently 82 cyan)

    // Text
    Text        lipgloss.Color // primary text (currently 252)
    TextMuted   lipgloss.Color // dimmed/secondary (currently 241)
    TextHint    lipgloss.Color // hints/descriptions (currently 243)

    // Backgrounds
    BgBase      lipgloss.Color // app background
    BgPanel     lipgloss.Color // panel/card background (currently 235)
    BgSelected  lipgloss.Color // selected row (currently 236)
    BgFocused   lipgloss.Color // focused element (currently 238)

    // Borders
    BorderNormal  lipgloss.Color // default borders (currently 240)
    BorderFocused lipgloss.Color // focused borders (currently 63)
    BorderAccent  lipgloss.Color // accent borders (currently 214)

    // Diff (new)
    DiffAdded       lipgloss.Color
    DiffRemoved     lipgloss.Color
    DiffContext     lipgloss.Color
    DiffAddedBg     lipgloss.Color
    DiffRemovedBg   lipgloss.Color

    // Markdown (for glamour style overrides)
    MdHeading   lipgloss.Color
    MdLink      lipgloss.Color
    MdCode      lipgloss.Color
    MdBlockquote lipgloss.Color
}
```

**Semantic slots**: ~30 colors (vs. OpenCode's 60+ — start lean, expand as needed).

### 1.2 Built-in Themes

Implement 6 themes to start:
- **Polycode** (default) — current orange/blue/cyan palette, preserved exactly
- **Catppuccin Mocha** — popular pastel dark theme
- **Tokyo Night** — purple/blue dark theme
- **Dracula** — green/purple/pink dark theme
- **Gruvbox Dark** — warm retro palette
- **Nord** — cool blue-gray palette

Each theme is a `Theme` struct literal in `themes.go`. No external config files needed initially.

### 1.3 Consolidate Hardcoded Colors

Refactor in order of density:
1. **view.go** (47 hardcoded calls) — renderTabBar, renderCommandPalette, renderModePicker, renderProvenance, renderMCPDashboard, renderChat, renderHelp
2. **settings.go** (16 calls) — table headers, status indicators, row rendering
3. **splash.go** (4 calls) — logo, version, tagline
4. **wizard.go** (3 calls) — step indicators
5. **mcp_wizard.go** (1 call)

Replace every `lipgloss.Color("214")` etc. with `m.theme.Primary` etc. The `defaultStyles()` function should accept a `Theme` and derive all `Styles` from it.

### 1.4 Theme Picker Overlay

- Trigger: `Ctrl+T` or `/theme` slash command
- UI: List overlay (reuse mode picker pattern) showing theme names with a live preview swatch
- State: `themePickerOpen bool`, `themePickerIdx int`
- Persistence: Save selected theme name to config YAML (`theme: "catppuccin"`)
- On select: rebuild `Styles` from new theme, re-render all cached content

### 1.5 Glamour Theme Integration

Pass theme colors to glamour's `ansi.StyleConfig` so markdown rendering matches the active theme. This affects:
- Code block backgrounds
- Heading colors
- Link colors
- Blockquote styling

**Files to modify**: `theme.go` (new), `model.go` (Styles derivation), `view.go`, `settings.go`, `splash.go`, `wizard.go`, `mcp_wizard.go`, `markdown.go`, `config/config.go` (persist theme choice)

**Estimated scope**: ~800-1200 lines changed across 8-10 files

---

## Phase 2: `@` File Completion

**Goal**: Typing `@` in the textarea triggers an inline fuzzy file picker overlay, allowing users to attach file contents as context.

**Why second**: Highest functional impact for a coding assistant. The infrastructure for overlays and filtering already exists.

### 2.1 File Index

Create `internal/tui/filepicker.go`:
- On startup (and periodically), walk the working directory respecting `.gitignore`
- Store as `[]string` of relative paths
- Cap at ~10,000 entries for performance
- Use `go-gitignore` or shell out to `git ls-files` for gitignore handling

### 2.2 Trigger Detection

In `updateChat()`, after textarea update:
- Detect `@` character in textarea value
- Extract filter text: everything after the last `@` until cursor position
- If filter is active, open file picker overlay

### 2.3 Fuzzy Matching

Add `github.com/sahilm/fuzzy` dependency for ranked fuzzy matching:
- Match against relative file paths
- Show top 10 results
- Highlight matching characters in results

### 2.4 File Picker Overlay

Render above the textarea (same position as command palette):
```
  src/internal/tui/view.go          [Go]
  src/internal/tui/update.go        [Go]
> src/internal/tui/model.go         [Go]

  @mod  ↑↓ navigate  Tab accept  Esc cancel
```

- Arrow keys navigate; Tab/Enter accepts
- On accept: replace `@filter` text with `@filepath` in textarea
- Store selected file paths in `m.attachedFiles []string`
- On submit: read file contents and prepend to prompt as context block

### 2.5 Visual Indicators

Show attached files as pills/badges above the textarea:
```
  [@src/tui/model.go ×] [@go.mod ×]
  ┌──────────────────────────────────────┐
  │ Ask polycode anything...             │
  └──────────────────────────────────────┘
```

`Ctrl+Backspace` or clicking `×` removes an attachment.

**Files to modify**: `filepicker.go` (new), `update.go` (trigger detection, key handling), `view.go` (overlay rendering, attachment pills), `model.go` (state fields)

**New dependency**: `github.com/sahilm/fuzzy`

**Estimated scope**: ~400-600 lines new code

---

## Phase 3: Enhanced Command Palette

**Goal**: Upgrade the existing slash command palette into a unified fuzzy command palette (`Ctrl+P`) that searches across commands, files, sessions, and models.

### 3.1 Fuzzy Filtering

Replace `filterPaletteCommands()` substring matching with fuzzy matching:
- Highlight matched characters in command names
- Score-based ranking (not just insertion order)
- Same `sahilm/fuzzy` library from Phase 2

### 3.2 Multi-Source Search

The palette should search across multiple categories:
- **Commands** (existing): `/clear`, `/export`, `/mcp list`, etc.
- **Files** (from Phase 2 index): prefix with `>` or auto-detect non-`/` input
- **Sessions** (from `config.ListSessions()`): switch to a previous session
- **Models** (from provider configs): quick model/provider switching
- **Themes** (from Phase 1): quick theme switching

Category headers in results:
```
  Commands
    /clear                  Clear conversation
    /export                 Export session as JSON
  Files
    internal/tui/view.go
    internal/tui/model.go
  Sessions
    polybot session (3h ago, 12 exchanges)
```

### 3.3 Activation

- `Ctrl+P` opens the palette (universal, like VS Code)
- `/` in empty textarea still opens command-filtered palette (existing behavior preserved)
- `@` opens file-filtered palette (from Phase 2)
- `Esc` closes; `Enter` executes; `Tab` accepts without executing

### 3.4 MRU (Most Recently Used)

Track command usage frequency. Show recently used commands at the top when palette opens with no filter.

**Files to modify**: `update.go` (filtering, key handling), `view.go` (rendering), `model.go` (state)

**Estimated scope**: ~300-500 lines changed

---

## Phase 4: Tool Call UX Improvements

**Goal**: Make tool execution more readable and less noisy.

### 4.1 Conceal/Collapse Tool Calls

Add a toggle (`Ctrl+H` or `/conceal`) that collapses tool call details in the chat view:

**Expanded (default)**:
```
🔧 file_read — Reading internal/tui/view.go
   ┌─────────────────────────────────────┐
   │ func renderChat(m Model) string {   │
   │   ...                               │
   │ }                                   │
   └─────────────────────────────────────┘

🔧 shell_exec — Running `go test ./...` (2.3s)
   ┌─────────────────────────────────────┐
   │ ok  ./internal/tui  0.45s           │
   │ ok  ./internal/config  0.12s        │
   └─────────────────────────────────────┘
```

**Concealed**:
```
⚙ 2 tool calls (file_read, shell_exec) — 2.5s total
```

- State: `m.concealTools bool`
- Renders differently in `renderChat()` based on flag
- Individual tool calls can be expanded/collapsed with click or cursor

### 4.2 Friendly Action Descriptions

Replace raw tool names with human-readable verbs:
| Tool Name | Display |
|-----------|---------|
| `file_read` | "Reading {path}" |
| `file_write` | "Writing {path}" |
| `file_edit` | "Editing {path}" |
| `file_delete` | "Deleting {path}" |
| `shell_exec` | "Running `{command}`" |
| `grep_search` | "Searching for '{pattern}'" |
| `find_files` | "Finding files matching '{pattern}'" |
| `list_directory` | "Listing {path}" |
| `mcp_*` | "MCP: {server} → {tool}" |

This mapping already partially exists in `ToolCallMsg.Description` — standardize it.

### 4.3 Elapsed Time Display

Track and display per-tool-call duration:
- Start timer on `ToolCallMsg` receipt
- Show elapsed on completion: `"Running go test ./... (2.3s)"`
- Show spinner + elapsed while running: `"Running go test ./... ⠋ 1.2s"`

### 4.4 Result Truncation with Expand

Tool results longer than 10 lines show truncated with a `[+42 more lines]` indicator. Pressing Enter or a key expands the full result inline.

**Files to modify**: `update.go` (conceal toggle, timing), `view.go` (rendering), `model.go` (state fields)

**Estimated scope**: ~300-400 lines changed

---

## Phase 5: Desktop Notifications

**Goal**: Notify the user when polycode needs attention while the terminal is unfocused.

### 5.1 Terminal Focus Detection

Use terminal focus reporting (ANSI escape sequences):
- Send `\x1b[?1004h` on startup to enable focus events
- Terminal sends `\x1b[I` (focus in) and `\x1b[O` (focus out)
- Track state in `m.terminalFocused bool`

### 5.2 Notification Triggers

Send OS notification when terminal is unfocused and:
- **Tool confirmation needed** — "polycode needs approval: shell_exec `rm -rf build/`"
- **Query complete** — "polycode: consensus ready (3 providers, 4.2s)"
- **Error occurred** — "polycode: provider failed: OpenAI rate limit"

### 5.3 Platform Implementation

Create `internal/notify/notify.go`:
- **macOS**: `osascript -e 'display notification ...'`
- **Linux**: `notify-send` (freedesktop)
- Gate behind `--notify` flag or config option (opt-in)

**Files to modify**: `notify/notify.go` (new), `model.go` (focus state), `update.go` (focus events, notification dispatch), `config/config.go` (opt-in flag)

**Estimated scope**: ~150-250 lines new code

---

## Phase 6: Auto-Compact on Context Overflow

**Goal**: Prevent context window overflow by auto-summarizing when token usage reaches a threshold.

### 6.1 Context Usage Display

Enhance the status bar or tab bar to show context usage as a percentage:
```
polycode [balanced] | Context: 45.2K/128K (35%) | $0.34
```

Color transitions:
- Green (< 60%)
- Yellow (60-80%)
- Orange (80-95%)
- Red blinking (> 95%, auto-compact triggered)

### 6.2 Auto-Compact Trigger

When primary model's context usage reaches 95%:
1. Pause accepting new input
2. Show status: "Compacting context..."
3. Send summarization prompt to primary model (summary of conversation so far)
4. Create new session with summary as system context
5. Resume with fresh context window

### 6.3 Manual Compact

`/compact` command triggers the same flow manually at any time.

**Files to modify**: `update.go` (threshold detection), `view.go` (usage display), `model.go` (state), app.go (compact logic), `tokens/tracker.go` (percentage calculation)

**Estimated scope**: ~200-400 lines

---

## Phase 7: Visual Polish Details

**Goal**: Small touches that collectively elevate perceived quality.

### 7.1 Dynamic Textarea Growth

Textarea grows from 1 to 8 lines as content increases:
- Calculate line count from textarea value
- `ta.SetHeight(min(max(lineCount+1, 1), 8))`
- Recalculate viewport height when textarea height changes

### 7.2 Overlay Drop Shadows

Add `░` character shadows to modal overlays (help, mode picker, MCP dashboard, provenance):
```go
shadow := strings.Repeat("░", width+2)
overlay = lipgloss.JoinVertical(lipgloss.Left,
    lipgloss.JoinHorizontal(lipgloss.Top, content, "░"),
    " "+shadow,
)
```

### 7.3 Gradient Header

Use `go-colorful` for a subtle gradient on the "polycode" title in the tab bar:
- Interpolate between theme.Primary and theme.Secondary across characters
- Only when terminal supports true color (detect via COLORTERM env var)
- Fall back to solid color on 256-color terminals

### 7.4 Improved Diff Rendering

When tool results contain diffs (detected by `@@` / `+++` / `---` markers):
- Colorize added lines (green), removed lines (red), context (dimmed)
- Use theme's diff color slots from Phase 1
- Apply in `renderChat()` when displaying tool results

### 7.5 Scrollbar Indicators

Add thin scrollbar track to chat viewport and provider panels:
- Use `┃` for thumb, `│` for track
- Theme-colored (muted)
- Only show when content exceeds viewport

### 7.6 Splash Screen Enhancement

- Add subtle animation: fade-in effect by rendering progressively
- Show version + last session info ("Resuming session from 2h ago, 5 exchanges")
- Auto-dismiss after 1.5s (current behavior) or on keypress

**Files to modify**: Various across `view.go`, `model.go`, `update.go`, `splash.go`

**Estimated scope**: ~300-500 lines total across all items

---

## Phase 8: Session Management Enhancements

**Goal**: Make session management a first-class feature rather than slash-command-only.

### 8.1 Session Picker Overlay

`Ctrl+S` or `/sessions` opens a session picker overlay:
```
  Sessions ─────────────────────────────────────────

  > polybot (current)         12 exchanges    2h ago
    polybot — refactor auth    8 exchanges    1d ago
    polybot — fix mcp bug      3 exchanges    3d ago

  enter open  n new  d delete  r rename  Esc close
```

- Fuzzy search over session names
- Shows exchange count and relative time
- Inline rename with text input
- Delete with confirmation

### 8.2 Session Export Improvements

- `/export md` — export as markdown (readable conversation format)
- `/export json` — existing JSON export
- `/share` — copy session as markdown to clipboard

### 8.3 Session Auto-Naming

After the first exchange, auto-generate a descriptive session name by asking the primary model for a 3-5 word summary of the conversation topic.

**Files to modify**: `update.go`, `view.go`, `model.go`, `config/session.go`

**Estimated scope**: ~300-500 lines

---

## Phase 9: Undo/Redo with Git Snapshots

**Goal**: Allow reverting file changes made by tool execution.

### 9.1 Snapshot Creation

Before each tool call that modifies files (`file_write`, `file_edit`, `file_delete`, `shell_exec`):
- Create a git stash or lightweight tag as a snapshot point
- Store snapshot reference in the exchange data model

### 9.2 Undo/Redo Commands

- `/undo` — restore files to state before last modifying tool call
- `/redo` — re-apply the undone change
- Stack-based: multiple undos supported up to session start

### 9.3 Visual Feedback

Show undo state in status area:
```
⟲ Undone: file_write internal/tui/view.go (3 more undoable)
```

**Files to modify**: `action/executor.go` (snapshot creation), `app.go` (undo/redo logic), `update.go`, `view.go`

**Estimated scope**: ~300-400 lines

---

## Implementation Order & Dependencies

```
Phase 1: Theme System ──────────────────────────┐
  (no dependencies, enables all visual work)     │
                                                  ▼
Phase 2: @ File Completion ──► Phase 3: Enhanced Command Palette
  (adds fuzzy dep + file index)   (builds on fuzzy + file index)

Phase 4: Tool Call UX ──────── (independent, benefits from themes)

Phase 5: Desktop Notifications (independent)

Phase 6: Auto-Compact ──────── (independent, uses token tracker)

Phase 7: Visual Polish ──────── (depends on Phase 1 themes)

Phase 8: Session Management ── (independent, extends existing sessions)

Phase 9: Undo/Redo ─────────── (independent, extends action system)
```

**Critical path**: Phase 1 → Phase 7 (themes must land first for visual polish to use semantic colors)

**Parallel tracks after Phase 1**:
- Track A: Phase 2 → Phase 3 (file completion → command palette)
- Track B: Phase 4 (tool call UX)
- Track C: Phase 5 + 6 (notifications + auto-compact)
- Track D: Phase 8 + 9 (sessions + undo/redo)

---

## New Dependencies

| Package | Purpose | Phase |
|---------|---------|-------|
| `github.com/sahilm/fuzzy` | Fuzzy matching for file picker + command palette | 2, 3 |
| `github.com/lucasb-eyer/go-colorful` | Perceptual color interpolation for gradients | 7 |

Both are small, well-maintained, and used by the Charm ecosystem.

---

## What NOT to Copy

Some features from Crush/OpenCode that **don't fit** polycode's design:

- **LSP integration** — heavy dependency, out of scope for a consensus-focused tool
- **Image rendering (Kitty/Sixel)** — niche terminal support, not worth the complexity
- **Plugin ecosystem** — premature; stabilize core UX first
- **Leader key system** — polycode's shortcut space isn't crowded enough to need it yet
- **Transparent mode** — nice but low impact relative to effort
- **Custom theme JSON files** — start with built-in themes; add custom themes later if requested
