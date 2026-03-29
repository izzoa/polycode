# POLISH.md — Unified UX/UI Polish Roadmap

Consolidated from competitive analysis of **Crush**, **OpenCode**, **Claude Code**, and **Aider**, synthesized across seven independent review documents into a single execution plan.

---

## Product Thesis

Polycode already has the hard part: parallel fan-out, consensus synthesis, provider traces, provenance, session persistence, MCP integration, tool execution, and verification. The polish gap is that too much of that state is hidden, command-driven, or visually under-explained.

The strongest UX direction for Polycode is:

1. **Make consensus visible, not magical.** Users should see how answers are formed, not just receive them.
2. **Make runtime state legible at a glance.** Mode, providers, context pressure, tool activity — always visible.
3. **Replace chat-output-as-control-surface with navigable UI.** Anything important enough for a slash command should be discoverable through the UI.
4. **Preserve terminal-native speed and density.** Support both a comfortable mode for new users and a dense mode for daily operators.
5. **Borrow proven ergonomics, but adapt them.** Every borrowed feature should answer: how does this improve trust in consensus? How does this help users compare model behavior?

---

## Current State Assessment

### Existing strengths (differentiators)

| Feature | Status | Details |
|---------|--------|---------|
| Multi-model consensus | Unique | Neither Crush nor OpenCode supports parallel multi-model querying with synthesis |
| Provenance panel | Strong | Confidence levels, agreement/minority reports, evidence citations |
| Per-provider trace tabs | Strong | Phase-aware activity logs (fanout, synthesis, tool, verify) |
| MCP dashboard | Strong | Richer server/tool inspection than Crush |
| Session persistence | Solid | Auto-save/resume per working directory, export, list/show/delete/name |
| Command palette | Solid | 32+ slash commands with filtering, tab-accept, subcommand hiding |
| Markdown rendering | Solid | Glamour with auto-style, ~500ms throttle during streaming |
| Input history | Solid | Up/down cycling with draft preservation |
| Tool confirmations | Solid | Y/N gates for mutating tools, YOLO mode toggle |

### Gaps to close

| Gap | Impact | Source |
|-----|--------|--------|
| No theme system — 85+ hardcoded `lipgloss.Color()` calls | High | All docs agree: foundation blocker |
| No colorized diffs — `unifiedDiff()` produces plain text only | High | MIMO, GLM |
| No inline file references (`@` mentions) | High | All docs agree: highest functional impact |
| No persistent status bar (context %, cost, model, mode) | High | GLM, MIMO, CODEX |
| No toast/transient notification system | Medium | GLM, MIMO |
| No tool call collapse/conceal — always fully visible | Medium | CLAUDE, GLM |
| No fuzzy search — command palette uses substring matching only | Medium | CLAUDE, MINIMAX |
| No session picker UI — slash commands only | Medium | CLAUDE, GLM, MIMO, CODEX |
| No desktop notifications when terminal unfocused | Medium | CLAUDE, CODEX |
| No turn-level artifact timeline (what happened during this turn?) | Medium | CODEX, GEMINI |
| No changes drawer (which files were touched?) | Medium | CODEX |
| No copy-to-clipboard keybinding | Low | GLM, MIMO |
| No vim-style scroll keys (j/k/d/u/g/G) | Low | GLM, MIMO |
| No split-pane layout for wide terminals | Low | GLM, MIMO |
| Fixed textarea height (always 3 lines) | Low | CLAUDE |

---

## Phase 0: Foundation

*Architectural prerequisites that all subsequent phases depend on. Ship these first to prevent rework.*

### 0.1 Theme System

**Why first**: Every visual feature (diffs, toasts, status bar, overlays) needs semantic colors. Fixing the color architecture first prevents refactoring every file twice.

**Theme struct** (`internal/tui/theme.go`):

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
    TextBright  lipgloss.Color // highlights, selected items

    // Backgrounds
    BgBase      lipgloss.Color // app background
    BgPanel     lipgloss.Color // panel/card background (currently 235)
    BgSelected  lipgloss.Color // selected row (currently 236)
    BgFocused   lipgloss.Color // focused element (currently 238)

    // Borders
    BorderNormal  lipgloss.Color // default borders (currently 240)
    BorderFocused lipgloss.Color // focused borders (currently 63)
    BorderAccent  lipgloss.Color // accent borders (currently 214)

    // Diff
    DiffAdded     lipgloss.Color
    DiffRemoved   lipgloss.Color
    DiffContext   lipgloss.Color
    DiffAddedBg   lipgloss.Color
    DiffRemovedBg lipgloss.Color

    // Markdown (for glamour style overrides)
    MdHeading    lipgloss.Color
    MdLink       lipgloss.Color
    MdCode       lipgloss.Color
    MdBlockquote lipgloss.Color

    // Syntax (for Chroma highlighting)
    SyntaxKeyword lipgloss.Color
    SyntaxString  lipgloss.Color
    SyntaxComment lipgloss.Color
    SyntaxFunc    lipgloss.Color
    SyntaxType    lipgloss.Color
    SyntaxNumber  lipgloss.Color
}
```

~35 semantic color slots (lean start — OpenCode has 60+, expand as needed).

**Built-in themes** (6):
- **Polycode** (default) — preserves current orange/blue/cyan palette exactly
- **Catppuccin Mocha** — popular pastel dark
- **Tokyo Night** — purple/blue dark
- **Dracula** — green/purple/pink dark
- **Gruvbox Dark** — warm retro
- **Nord** — cool blue-gray

Each theme is a struct literal in `themes.go`. No external config files initially.

**Consolidation roadmap** (by hardcoded color density):
1. `view.go` (47 calls) — renderTabBar, renderCommandPalette, renderModePicker, renderProvenance, renderMCPDashboard, renderChat, renderHelp
2. `settings.go` (16 calls) — table headers, status indicators, row rendering
3. `splash.go` (4 calls) — logo, version, tagline
4. `wizard.go` (3 calls) — step indicators
5. `mcp_wizard.go` (1 call)

Replace every `lipgloss.Color("214")` with `m.theme.Primary` etc. `defaultStyles()` accepts a `Theme` and derives all `Styles` from it.

**Theme picker**: `Ctrl+T` or `/theme` → list overlay (reuse mode picker pattern) with live preview swatch. Persist selection to config YAML (`theme: "catppuccin"`). On select: rebuild Styles, invalidate all cached renders.

**Glamour integration**: Pass theme colors to `ansi.StyleConfig` so markdown rendering matches the active theme (code blocks, headings, links, blockquotes).

**Files**: `theme.go` (new), `themes.go` (new), `model.go`, `view.go`, `settings.go`, `splash.go`, `wizard.go`, `mcp_wizard.go`, `markdown.go`, `config/config.go`

**Scope**: ~800-1200 lines changed across 10 files

### 0.2 Toast Notification System

**Why second**: Many features need non-blocking feedback (config saved, theme applied, MCP reconnected, clipboard copied, error occurred). Without a toast system, feedback gets lost in chat or is invisible.

```go
type Toast struct {
    Message   string
    Variant   ToastVariant // Info, Success, Warning, Error
    Duration  time.Duration
    CreatedAt time.Time
}
```

- Render as bottom-right anchored overlay, stacked (max 3 visible)
- Left border colored by variant (blue/green/yellow/red), subtle background
- Auto-dismiss via `ToastTickMsg` timer
- Wire into existing events: config save → success, MCP reconnect → info, test failure → error, clipboard copy → success

**Files**: `toast.go` (new), `model.go`, `update.go`, `view.go`

**Scope**: ~200-300 lines new code

---

## Phase 1: Context & Input

*Make it easy to inject context and compose prompts.*

### 1.1 `@` File Reference with Fuzzy Search

The single highest-functional-impact feature for a coding assistant. Every competing tool has this.

**File index**:
- On startup, populate via `git ls-files` (respects `.gitignore` automatically)
- Cap at ~10,000 entries
- Refresh periodically or on `ConfigChangedMsg`

**Trigger**: Typing `@` anywhere in textarea opens a fuzzy file picker overlay above the input.

**Fuzzy matching**: Add `github.com/sahilm/fuzzy` for ranked results with highlighted match characters. Show top 10 results.

**Overlay rendering**:
```
  src/internal/tui/view.go          [Go]
  src/internal/tui/update.go        [Go]
> src/internal/tui/model.go         [Go]

  @mod  ↑↓ navigate  Tab accept  Esc cancel
```

**On accept**: Replace `@filter` with `@filepath`. Store in `m.attachedFiles []string`.

**Attachment pills** above textarea:
```
  [@src/tui/model.go ×] [@go.mod ×]
  ┌──────────────────────────────────────┐
  │ Ask polycode anything...             │
  └──────────────────────────────────────┘
```

**On submit**: Read file contents, prepend to prompt as context blocks:
```
@path/to/file:
\`\`\`go
<file contents>
\`\`\`
```

**State fields**: `fileRefOpen bool`, `fileRefFilter string`, `fileRefMatches []string`, `fileRefCursor int`, `attachedFiles []string`

**Files**: `filepicker.go` (new), `update.go`, `view.go`, `model.go`

**New dependency**: `github.com/sahilm/fuzzy`

**Scope**: ~400-600 lines

### 1.2 `!` Shell Command Context Injection

Prefix input with `!` to run a shell command and feed its output into the prompt as context.

- `!git diff HEAD~1`, `!ls -la`, `!go test ./... 2>&1`
- Show "Running..." indicator via `toolStatus`
- Result injected as:
  ```
  $ <command>
  <output>
  ```
- Non-zero exit: include stderr, still submit (user decides how to handle)
- Detection: check for `!` prefix in `updateChat` enter handler, before slash command check

**Files**: `update.go`, `app.go`

**Scope**: ~50-100 lines

### 1.3 Dynamic Textarea Growth

Textarea grows from 1 to 8 lines as content increases:
- Calculate line count from textarea value
- `ta.SetHeight(min(max(lineCount+1, 1), 8))`
- Recalculate viewport height when textarea height changes

**Files**: `update.go`, `view.go`

**Scope**: ~30-50 lines

### 1.4 External Editor Compose

For long prompts, specs, or review instructions, in-place editing is suboptimal.

- `/compose` or `Ctrl+E` opens `$EDITOR` with a temp file
- On save and exit, contents loaded back into textarea (or submitted directly)
- Reuse patterns from how tools like `git commit` invoke `$EDITOR`

**Files**: `update.go`, `app.go`

**Scope**: ~100-150 lines

---

## Phase 2: Command & Navigation

*Make the app faster and more discoverable to operate.*

### 2.1 Enhanced Command Palette

Upgrade the existing slash palette into a unified fuzzy command launcher.

**Activation**:
- `Ctrl+P` opens the palette (universal, like VS Code) — searches everything
- `/` in empty textarea still opens command-filtered palette (existing behavior)
- `@` opens file-filtered palette (from Phase 1.1)
- `Esc` closes; `Enter` executes; `Tab` accepts without executing

**Fuzzy filtering**: Replace `filterPaletteCommands()` with `sahilm/fuzzy`. Score-based ranking with highlighted match characters.

**Multi-source search** with category headers:
```
  Commands
    /clear                  Clear conversation
    /export                 Export session as JSON
  Files
    internal/tui/view.go
    internal/tui/model.go
  Sessions
    polybot session (3h ago, 12 exchanges)
  Themes
    Tokyo Night
```

**MRU (most recently used)**: Track command frequency. Show recent commands at top when palette opens with no filter.

**Grouped display**: Navigation, Sessions, MCP, Workflows, Project Actions.

**Custom user commands** (config.yaml):
```yaml
commands:
  - name: "Run tests"
    description: "Execute the test suite"
    prompt: "run go test ./... and fix any failures"
  - name: "Review staged"
    description: "Review staged changes"
    prompt: "review the changes in git diff --staged"
```

**Files**: `update.go`, `view.go`, `model.go`, `config/config.go`

**Scope**: ~300-500 lines

### 2.2 Vim-Style Scroll Keys

When input is empty (or tab bar focused):
- `j`/`k` → scroll 1 line
- `d`/`u` → half-page (alias for existing Ctrl+D/U)
- `g`/`G` → top/bottom (alias for Home/End)

Only active with empty textarea to avoid text entry conflicts. Reuse existing `chatScrollBy()`.

**Files**: `update.go`

**Scope**: ~30 lines

### 2.3 Copy-to-Clipboard

- `y` keybinding (input empty): copy last consensus response to clipboard
- `/copy` command: copy current consensus or selected provider's response
- Toast feedback: "Copied to clipboard" (requires Phase 0.2)
- Use `atotto/clipboard` (already a transitive dependency)

**Files**: `clipboard.go` (new), `update.go`, `model.go`

**Scope**: ~80 lines

---

## Phase 3: Tool Execution UX

*Make tool execution more readable, trustworthy, and less noisy. This is the single biggest polish win for daily usage.*

### 3.1 Conceal/Collapse Tool Calls

Toggle (`Ctrl+H` or `/conceal`) that collapses tool call details in chat:

**Expanded (default)**:
```
🔧 Reading internal/tui/view.go
   ┌─────────────────────────────────────┐
   │ func renderChat(m Model) string {   │
   │   ...                               │
   └─────────────────────────────────────┘

🔧 Running `go test ./...` (2.3s)
   ┌─────────────────────────────────────┐
   │ ok  ./internal/tui  0.45s           │
   └─────────────────────────────────────┘
```

**Concealed**:
```
⚙ 2 tool calls (file_read, shell_exec) — 2.5s total
```

- State: `m.concealTools bool`
- Per-turn tool count, names, and total duration computed during rendering
- Individual tool calls expandable with cursor in future iteration

### 3.2 Friendly Action Descriptions

Standardize human-readable verbs (partially exists in `ToolCallMsg.Description` — make consistent):

| Tool | Display |
|------|---------|
| `file_read` | "Reading {path}" |
| `file_write` | "Writing {path}" |
| `file_edit` | "Editing {path}" |
| `file_delete` | "Deleting {path}" |
| `file_rename` | "Renaming {old} → {new}" |
| `shell_exec` | "Running \`{command}\`" |
| `grep_search` | "Searching for '{pattern}'" |
| `find_files` | "Finding files matching '{pattern}'" |
| `list_directory` | "Listing {path}" |
| `file_info` | "Inspecting {path}" |
| `mcp_*` | "MCP: {server} → {tool}" |

Strip working directory prefix from paths for brevity.

### 3.3 Elapsed Time Per Tool Call

- Start timer on `ToolCallMsg` receipt
- While running: `"Running go test ./... ⠋ 1.2s"`
- On completion: `"Running go test ./... (2.3s)"`

### 3.4 Result Truncation with Expand

Tool results > 10 lines show truncated: `[+42 more lines]`. Future: key press expands inline.

### 3.5 Syntax-Highlighted Diff Rendering

The biggest visual gap. Currently `unifiedDiff()` in `action/file_ops.go` produces plain text.

**Approach**:
- Add `github.com/alecthomas/chroma/v2` as direct dependency (glamour already uses it internally)
- Create `internal/tui/diff.go`:
  - Unified mode (default, < 120 cols)
  - Split mode (≥ 120 cols, auto-switch like OpenCode)
  - Toggle with `d` key during confirm prompt
- Chroma syntax highlighting from filename extension
- Theme-aware: use `Theme.DiffAdded`, `DiffRemoved`, `DiffContext` colors
- Cache rendered output keyed by content hash + theme
- Apply to: confirm prompts, `/diff` command, edit tool results in chat history
- Also detect diffs in chat output (by `@@` / `+++` / `---` markers) and colorize inline

### 3.6 Editable Approval Gates

Instead of strict `y/n` for mutating tools, add `e` to edit the command or content before execution:

- On `e` press: swap view to textarea containing the tool's command string or proposed file content
- User edits, then submits → modified version sent to executor
- If the bot writes an amazing command but makes one flag mistake, the user can fix it without re-prompting

### 3.7 Error Recovery UI

Replace raw `[ERROR: ...]` text with a structured error panel:
- Red-bordered collapsible panel
- Header: `✕ Error: <summary>` (1 line)
- Expanded: full message, stack trace, timestamp
- Actions: `r` to retry last query, `c` to copy error to clipboard, `a` to auto-fix (sends error to model)
- Toast notification on error occurrence

**Files** (all of Phase 3): `diff.go` (new), `error_panel.go` (new), `update.go`, `view.go`, `model.go`

**New dependency**: `github.com/alecthomas/chroma/v2`

**Scope**: ~600-900 lines total

---

## Phase 4: Observability

*Make runtime state legible at a glance. Users should always know: what is running, which providers are participating, what tools ran, what files changed, and whether verification passed.*

### 4.1 Persistent Status Bar

A bottom bar always visible in chat mode:

```
[Context: 42%] [12.4K/200K tokens] [$0.38] [claude-sonnet via Anthropic] [balanced]
```

- Context percentage with color transitions: green < 60%, yellow 60-80%, orange 80-95%, red > 95%
- Progress bar variant: `████████████░░░░░░░░ 42%`
- Single line, left-aligned mode/session, right-aligned metrics
- Subtle styling: dimmed text, thin top border
- Updates on `TokenUpdateMsg` and mode changes

### 4.2 Context Pressure Meter & Auto-Compact

Surface existing token tracking as a first-class UI element:

- Context usage percentage in status bar (4.1)
- Pre-compaction warning toast at 80%: "Context at 80% — consider /compact"
- Auto-compact trigger at 95%:
  1. Pause input
  2. Show status: "Compacting context..."
  3. Summarize conversation via primary model
  4. Create new session with summary as system context
  5. Resume with fresh window
- `/compact` command for manual trigger
- Compaction event visible in timeline (4.4)

### 4.3 Collapsible Thinking / Trace Sections

Trace content (`── Fan-out ──`, `── Synthesis ──`, `── Tool Execution ──`) consumes space and is rarely useful after a query completes.

- During streaming: always expanded
- After completion: auto-collapse to single line:
  ```
  ▶ Trace: 3 sections, 47 lines (press t to expand)
  ```
- Toggle with `t` key when chat viewport focused
- Per-exchange state persisted in `Exchange.expandedTrace bool`

### 4.4 Turn Artifact Timeline & Changes Drawer

This is likely the single biggest polish win for Polycode specifically, because it surfaces the unique multi-model workflow.

**Turn timeline** (displayed as a structured block per exchange):
```
  ❯ "refactor the auth middleware"
  ├─ Fan-out: 3 providers (Claude ✓ 1.2s, GPT-4 ✓ 2.1s, Gemini ✕ timeout)
  ├─ Synthesis: Claude (primary) 3.4s
  ├─ Tools: file_edit ×2, shell_exec ×1 (4.1s total)
  ├─ Verification: go test ./... ✓
  └─ Files: +45 -12 across 3 files
```

**Changes drawer** (toggled with `c` key or `/changes`):
```
  Changes ─────────────────────────────────────
  src/internal/auth/middleware.go    +32  -8
  src/internal/auth/middleware_test.go  +13  -4
  src/cmd/polycode/app.go           +0   -0  (read only)

  d:diff  r:revert  Esc:close
```

- Per-file add/remove counts (git-style)
- Expandable diff hunks per file
- Verification badge per turn: not run / running / passed / failed
- While a turn runs, timeline items appear live
- After completion, remains inspectable from history

**Architectural note**: This warrants an explicit turn-artifact model:

```go
type TurnArtifact struct {
    Prompt        string
    Providers     []ProviderResult  // name, status, latency
    SynthesisTime time.Duration
    ToolCalls     []ToolCallRecord  // name, target, duration, status
    Verification  VerificationResult
    FilesChanged  []FileChange      // path, additions, deletions
}
```

### 4.5 Live Task HUD for Tool Loops

During multi-step tool execution, show a dynamic checklist:
```
  [✓] Read files (3 files)
  [➤] Modifying server.go
  [ ] Running tests
```

Uses `bubbles/spinner` and updates via existing `ToolCallMsg` messages. Replaces the ambiguous "Thinking..." during long tool loops.

**Files** (all of Phase 4): `status_bar.go` (new), `timeline.go` (new), `changes.go` (new), `model.go`, `view.go`, `update.go`, `app.go`, `tokens/tracker.go`

**Scope**: ~800-1200 lines total

---

## Phase 5: Session Management

*Make sessions a first-class workflow object, not a slash-command afterthought.*

### 5.1 Session Picker Overlay

`Ctrl+S` or `/sessions` opens a visual session browser:

```
  Sessions ─────────────────────────────────────────

  > polybot (current)            12 exchanges    2h ago
    polybot — refactor auth       8 exchanges    1d ago
    polybot — fix mcp bug         3 exchanges    3d ago

  enter:open  n:new  d:delete  r:rename  /:filter  Esc:close
```

- Fuzzy search over session names and first-prompt content
- Shows exchange count, relative time, providers used
- Inline rename with text input
- Delete with confirmation
- Wire into existing `onSessions` callbacks

### 5.2 Session Auto-Naming

After the first exchange, auto-generate a descriptive session name:
- Ask the primary model for a 3-5 word summary of the conversation topic
- Display in status bar and session picker
- User can override with `/sessions name <name>`

### 5.3 Session Branching

Polycode-specific: users need to compare alternate lines of inquiry.

- `/branch` creates a new session forked from the current exchange
- Branch metadata: parent session ID, source exchange index
- Session picker shows branch relationships
- Use cases: compare consensus vs. single-model, try alternate approaches, separate review from implementation

### 5.4 Session Export Improvements

- `/export md` — readable markdown conversation format
- `/export json` — existing JSON export
- `/share` — copy session as markdown to clipboard

### 5.5 Git-Backed Undo/Redo

File changes tracked via git snapshots for surgical reversal:

- Before each mutating tool call: create a lightweight git snapshot (stash or tag)
- `/undo` — restore files to state before last modifying tool call
- `/redo` — re-apply undone change
- Stack-based: multiple undos supported up to session start
- Visual feedback in status area: `⟲ Undone: file_write view.go (3 more undoable)`
- Requires git repo (check via `git rev-parse --git-dir`), disabled silently if not
- Session-scoped snapshots cleaned up on `/clear` or quit

**Files** (all of Phase 5): `session_picker.go` (new), `model.go`, `view.go`, `update.go`, `config/session.go`, `action/executor.go`, `app.go`

**Scope**: ~600-900 lines total

---

## Phase 6: Notifications & Long-Running Tasks

*Make long-running operations feel supervised rather than stalled.*

### 6.1 Desktop Notifications

Notify users when the terminal is unfocused and polycode needs attention.

**Terminal focus detection**: Send `\x1b[?1004h` on startup → terminal reports `\x1b[I` (focus in) / `\x1b[O` (focus out). Track in `m.terminalFocused bool`.

**Triggers** (only when unfocused):
- Tool confirmation needed — "polycode needs approval: shell_exec \`rm -rf build/\`"
- Query complete — "polycode: consensus ready (3 providers, 4.2s)"
- Verification failed — "polycode: tests failed"
- `/plan` complete — "polycode: plan execution finished"

**Platform**: macOS `osascript`, Linux `notify-send`. Gate behind `--notify` flag or config option (opt-in).

### 6.2 Auto-Scroll Toggle

During streaming, auto-scroll to bottom (current behavior). Allow disabling:

- `Ctrl+G` toggles auto-scroll
- When disabled: subtle indicator in tab bar `[scroll locked]`
- Re-enabling jumps to bottom immediately
- Useful for reading explanations while streaming continues

### 6.3 Enhanced Spinner with Phase Labels

Replace uniform `bubbles/spinner` dot style with phase-aware feedback:

- Character set: `⠋⠙⠹⠸⠼⠴⠦⠧⠇` (braille, more visually interesting)
- Phase labels: "Dispatching...", "Thinking...", "Synthesizing...", "Executing tools...", "Verifying..."
- Color: cycle through theme Primary → Secondary gradient
- Model-aware during fan-out: "Claude thinking...", "GPT-4 thinking..."
- Per-provider label in tab bar (not just spinner icon)

**Files** (all of Phase 6): `notify/notify.go` (new), `spinner.go` (new), `model.go`, `update.go`, `view.go`, `config/config.go`

**Scope**: ~400-500 lines total

---

## Phase 7: Visual Polish

*Small touches that collectively elevate perceived quality. Depends on Phase 0 theme system.*

### 7.1 Overlay Drop Shadows

Add `░` character shadows to modal overlays (help, mode picker, MCP dashboard, provenance, session picker):

```go
shadow := strings.Repeat("░", width+2)
overlay = lipgloss.JoinVertical(lipgloss.Left,
    lipgloss.JoinHorizontal(lipgloss.Top, content, "░"),
    " "+shadow,
)
```

### 7.2 Gradient Header

Use `go-colorful` for a subtle gradient on the "polycode" title in the tab bar:
- Interpolate between `theme.Primary` and `theme.Secondary` across characters
- Only when terminal supports true color (detect via `COLORTERM` env var)
- Fall back to solid color on 256-color terminals

### 7.3 Scrollbar Indicators

Thin scrollbar track for chat viewport and provider panels:
- `┃` for thumb, `│` for track
- Theme-colored (muted)
- Only shown when content exceeds viewport

### 7.4 Splash Screen Enhancement

- Show version + last session info: "Resuming session from 2h ago, 5 exchanges"
- Auto-dismiss after 1.5s (current behavior) or on any keypress

### 7.5 Streaming Markdown Improvements

Render cache optimization during streaming:
- Cache keyed by content hash (avoid full re-render when content unchanged)
- Incremental append when delta is small (< 100 chars since last render)
- Full re-render only when accumulated delta is large (> 1000 chars)
- Result: streaming feels equally responsive with ~60-70% less CPU

**Files**: various across `view.go`, `model.go`, `update.go`, `splash.go`, `markdown.go`

**New dependency**: `github.com/lucasb-eyer/go-colorful`

**Scope**: ~300-500 lines total

---

## Phase 8: Layout & Advanced

*Larger features that transform the product. Ship after core polish is stable.*

### 8.1 Split Pane Layout

At wide terminals (≥ 140 cols), show consensus + provider simultaneously:
- Left panel (60%): consensus/chat view
- Right panel (40%): selected provider's response, diff view, or file preview
- Resizable with `Ctrl+←`/`Ctrl+→`
- Below 140 cols: current single-panel behavior (graceful degradation)
- Right panel content toggles: `1` = provider response, `2` = diff view, `3` = file preview, `Esc` = hide

### 8.2 Settings Redesign

Current settings read like admin tables. Should feel like a control center:

- Two-pane layout: left = provider/MCP list, right = selected-item details and actions
- Stronger status badges: connected, auth missing, unhealthy, primary, read-only
- Inline model metadata: context window, reasoning support, pricing
- Better wizard progress and validation messaging
- MCP server details: tools, read-only hints, env metadata, registry source

### 8.3 Approval UX Overhaul

Current confirmation is binary y/n. Should be explicit, informative, and controllable:

**Richer approval dialog showing**:
- Tool name and target files/command
- Risk level indicator (read-only vs. mutating vs. destructive)
- Reason from the model
- Which provider requested the action
- Whether the action emerged from consensus

**Expanded actions**:
- Allow once (current `y`)
- Deny once (current `n`)
- Allow for session (pattern-based, e.g., `git *`)
- Deny for session
- Always allow read-only tools

**Approval history** visible in turn timeline (Phase 4.4).

### 8.4 Configurable Keybindings

All keybindings currently hardcoded. Add config-driven bindings with optional leader key:

```yaml
keybindings:
  leader: "ctrl+x"
  quit: "ctrl+c"
  settings: "ctrl+s"
  sessions: "<leader>s"
  theme: "<leader>t"
  help: "?"
```

- Parse `<leader>` at load time, expand to actual key sequence
- Leader key: arms prefix state for 500ms, show subtle `^X` indicator
- Default bindings match current behavior (backward compatible)
- Validation: warn on conflicts

### 8.5 Mouse Support Enhancement

`tea.WithMouseCellMotion()` is enabled but unused:
- Click on tab bar → switch tab
- Click on chat area → focus viewport
- Click on input → focus textarea
- Mouse wheel → scroll active viewport
- Click on overlay items → select

### 8.6 Headless / Non-Interactive Mode

`polycode run "prompt"` for scripting, CI, and automation:
- No TUI — streams output to stdout
- Reads config from `~/.config/polycode/config.yaml`
- Tool execution: yolo by default, `--confirm` flag for prompts
- Exit codes: 0 success, 1 error, 2 rejected tool call

**Files** (all of Phase 8): `split.go` (new), `keys.go` (new), `session_picker.go`, `settings.go`, `wizard.go`, `update.go`, `view.go`, `model.go`, `config/config.go`, `cmd/polycode/run.go` (new)

**Scope**: ~1500-2500 lines total

---

## Dependency Map

```
Phase 0: Foundation
├── 0.1 Theme System ──────────────────── blocks ──┐
├── 0.2 Toast System ──────────────────── blocks ──┤
│                                                   │
Phase 1: Context & Input                            │
├── 1.1 @ File Completion ◄──────────────────────── │ (independent, adds fuzzy dep)
├── 1.2 ! Shell Context ───────────────────────────  │ (independent)
├── 1.3 Dynamic Textarea ──────────────────────────  │ (independent)
├── 1.4 External Editor ───────────────────────────  │ (independent)
│                                                   │
Phase 2: Command & Navigation                       │
├── 2.1 Enhanced Palette ◄─── requires ── 1.1 fuzzy │
├── 2.2 Vim Scroll Keys ──────────────────────────   │ (independent)
├── 2.3 Copy-to-Clipboard ◄── requires ────────────┘ (toast)
│
Phase 3: Tool Execution UX
├── 3.1-3.4 Tool Call Polish ◄── requires ── theme
├── 3.5 Diff Rendering ◄──────── requires ── theme + chroma
├── 3.6 Editable Approvals ───── (independent)
├── 3.7 Error Recovery UI ◄───── requires ── theme + toast
│
Phase 4: Observability
├── 4.1 Status Bar ◄──────────── requires ── theme
├── 4.2 Context Pressure ◄────── requires ── status bar
├── 4.3 Collapsible Traces ◄──── requires ── theme
├── 4.4 Turn Timeline ────────── (independent, but benefits from theme)
├── 4.5 Task HUD ─────────────── (independent)
│
Phase 5: Sessions ─────────────── (independent, benefits from theme)
Phase 6: Notifications ────────── (independent)
Phase 7: Visual Polish ◄──────── requires ── theme
Phase 8: Layout & Advanced ◄──── requires ── theme, most prior phases
```

### Critical Path

1. **Theme System (0.1)** → blocks all visual work
2. **Toast System (0.2)** → blocks clipboard, error recovery, notification feedback
3. **@ File Completion (1.1)** → adds fuzzy dep needed by enhanced palette (2.1)
4. **Status Bar (4.1)** → enables context pressure meter (4.2) and progress bars

### Parallel Tracks After Phase 0

- **Track A**: 1.1 → 2.1 (file completion → enhanced palette)
- **Track B**: 3.1-3.7 (tool execution UX — can be done incrementally)
- **Track C**: 4.1 → 4.2 → 4.3 → 4.4 (observability chain)
- **Track D**: 5.1-5.5 (session management)
- **Track E**: 6.1-6.3 (notifications)
- **Track F**: 7.1-7.5 (visual polish)

---

## Effort Estimates

| Phase | Feature | Effort | Priority |
|-------|---------|--------|----------|
| 0.1 | Theme System | 2-3 days | P0 — blocks everything |
| 0.2 | Toast System | 1 day | P0 — blocks feedback |
| 1.1 | @ File Completion | 2-3 days | P0 — highest functional impact |
| 1.2 | ! Shell Context | 0.5 day | P1 |
| 1.3 | Dynamic Textarea | 0.5 day | P1 |
| 1.4 | External Editor | 1 day | P2 |
| 2.1 | Enhanced Palette | 2-3 days | P1 |
| 2.2 | Vim Scroll Keys | 0.5 day | P1 |
| 2.3 | Copy-to-Clipboard | 0.5 day | P1 |
| 3.1-3.4 | Tool Call Polish | 2 days | P1 |
| 3.5 | Diff Rendering | 3-4 days | P0 — biggest visual gap |
| 3.6 | Editable Approvals | 1 day | P2 |
| 3.7 | Error Recovery UI | 1-2 days | P2 |
| 4.1 | Status Bar | 1 day | P1 |
| 4.2 | Context Pressure | 1-2 days | P1 |
| 4.3 | Collapsible Traces | 1 day | P1 |
| 4.4 | Turn Timeline + Changes | 3-4 days | P1 |
| 4.5 | Task HUD | 1 day | P2 |
| 5.1 | Session Picker | 2-3 days | P1 |
| 5.2 | Session Auto-Naming | 0.5 day | P2 |
| 5.3 | Session Branching | 2-3 days | P2 |
| 5.4 | Export Improvements | 0.5 day | P2 |
| 5.5 | Git-Backed Undo | 2-3 days | P2 |
| 6.1 | Desktop Notifications | 1-2 days | P2 |
| 6.2 | Auto-Scroll Toggle | 0.5 day | P2 |
| 6.3 | Enhanced Spinner | 0.5 day | P1 |
| 7.1-7.5 | Visual Polish Bundle | 2-3 days | P2 |
| 8.1 | Split Panes | 2-3 days | P3 |
| 8.2 | Settings Redesign | 2-3 days | P3 |
| 8.3 | Approval UX Overhaul | 2-3 days | P2 |
| 8.4 | Configurable Keys | 2-3 days | P3 |
| 8.5 | Mouse Support | 1-2 days | P3 |
| 8.6 | Headless Mode | 2-3 days | P3 |

**Total: ~45-60 days** across all phases

### Recommended Sprint Sequence

**Sprint 1 — Foundation (4 days)**: Theme system + Toast system + Enhanced spinner
**Sprint 2 — Core Input (4 days)**: @ File completion + ! Shell context + Dynamic textarea
**Sprint 3 — Core Visual (6 days)**: Diff rendering + Status bar + Collapsible traces + Tool call polish
**Sprint 4 — Navigation (3 days)**: Enhanced palette + Vim scroll + Copy-to-clipboard
**Sprint 5 — Observability (5 days)**: Turn timeline + Changes drawer + Context pressure
**Sprint 6 — Sessions (4 days)**: Session picker + Auto-naming + Export improvements
**Sprint 7 — Polish & Advanced (ongoing)**: Everything else from P2/P3 backlog

---

## New Dependencies

| Package | Purpose | Phase |
|---------|---------|-------|
| `github.com/sahilm/fuzzy` | Fuzzy matching for file picker + command palette | 1.1, 2.1 |
| `github.com/alecthomas/chroma/v2` | Syntax highlighting for diffs and file previews | 3.5 |
| `github.com/lucasb-eyer/go-colorful` | Perceptual color interpolation for gradients | 7.2 |

All are small, well-maintained, and used within the Charm ecosystem.

---

## Testing Strategy

Polish work often regresses interaction quality even when tests stay green.

### Automated
- TUI update tests for focus transitions and key routing
- Rendering tests for theme application (no hardcoded colors leak through)
- Session metadata persistence and restoration tests
- Turn-artifact persistence tests
- Approval-state transition tests
- Diff rendering correctness tests

### Manual
- Long multi-turn coding session with compaction
- Tool-heavy file editing flow (confirm, edit, undo)
- MCP-heavy session with dashboard interactions
- Session branch/restore/export flow
- Narrow terminal (< 80 cols) — no clipping or panic
- Wide terminal (> 160 cols) — split pane activates
- Theme switching mid-session — all cached renders invalidated
- Approval flow under stress (rapid tool calls)

---

## Anti-Goals

- Do not convert the app into a mouse-first pseudo-GUI
- Do not bury the transcript under decorative chrome
- Do not add visual complexity without adding decision clarity
- Do not copy Crush or OpenCode interactions verbatim — adapt to consensus-first product needs
- Do not let polish work obscure the multi-model differentiator
- Do not prematurely add: LSP integration, image rendering (Kitty/Sixel), plugin ecosystem, custom theme JSON files, transparent mode

---

## What NOT to Copy

Features from competitors that don't fit Polycode's design:

| Feature | Why skip |
|---------|----------|
| LSP integration | Heavy dependency, out of scope for a consensus-focused tool |
| Image rendering (Kitty/Sixel) | Niche terminal support, not worth complexity |
| Plugin ecosystem | Premature; stabilize core UX first |
| Transparent mode | Nice but low impact relative to effort |
| Custom theme JSON files | Start with built-in themes; add later if requested |
| Watch mode (Aider-style) | Doesn't align with interactive consensus workflow |
| Agent switching (Crush-style) | Polycode's multi-model approach is fundamentally different |

---

## Success Criteria

### Qualitative
- Users can explain why consensus landed where it did (provenance is obvious)
- Users can inspect turns without reading raw transcript noise (timeline + changes)
- Users can navigate major features without memorizing commands (palette + overlays)
- Settings and session flows feel like product surfaces, not implementation leftovers

### Quantitative Proxies
- Fewer slash-command-only flows for major actions
- Fewer help-overlay roundtrips for common tasks
- Lower friction around approvals (fewer yolo-mode users)
- Faster recovery when returning to a saved session

---

## Bottom Line

Polycode does not primarily need more features. It needs stronger presentation of the features it already has.

The best polish program makes existing strengths obvious:
- Consensus is inspectable
- Sessions are operable
- Tools are supervised
- Changes are reviewable
- Runtime state is always legible

Executed well, this gives Polycode a sharper identity than simply becoming "another terminal coding agent with nicer colors."
