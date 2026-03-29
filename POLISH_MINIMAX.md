# UX Polish for Polycode — opencode/crush-inspired features

## Why

Polycode's core consensus architecture is solid, but the TUI UX lags behind tools like opencode and crush in several areas that materially affect daily usability:

- **Context injection** — referencing files in prompts requires copy-paste; opencode's `@` syntax and crush's file picker are far faster
- **Keyboard ergonomics** — `ctrl+` chords feel heavy; opencode's leader-key bindings and `/` commands feel immediate
- **Trust and preview** — file-modifying tools execute without showing intent; opencode's plan mode gives users a chance to course-correct before wrong changes land
- **Session hygiene** — undo is limited to the last exchange; git-backed tracking would enable surgical reversal
- **Viewport performance** — markdown re-rendering during streaming is CPU-heavy; render caching and viewport-based dirty tracking would help
- **Theming and customization** — a single hardcoded style means no room for personal preference or accessibility needs

This document lays out a phased plan to close these gaps, prioritized by impact vs. effort.

---

## Phase 1: High-Impact, Low-Effort

Features that leverage existing infrastructure and deliver immediate quality-of-life improvements.

### 1. `@` File Reference Fuzzy Search

**What it is:** Inline file references in prompts using `@filename` or `@path/`. Triggered when `@` appears anywhere in the input (not just at the start). Does a fuzzy match across the project tree and injects file content into the prompt context before sending.

**How it works:**
- A new `fileRefDialog` overlay (dialog stack, reusing the overlay pattern from view.go)
- Typing `@` anywhere in the textarea opens the dialog
- Fuzzy filters project files (uses `action.listDirectory` + `find_files` or walks `os.ReadDir`)
- Enter or Tab inserts the selected file's path as `@path/to/file`
- On submit: the `@path` token is replaced with the file's contents before being sent to providers
- Syntax: `@foo.go`, `@src/utils/`, `@*.go` (glob patterns)
- If no file matches, show "No matches" and allow closing with Esc

**Model changes:**
- `fileRefOpen bool`
- `fileRefFilter string`
- `fileRefMatches []string` — matched file paths
- `fileRefCursor int`
- `onFileRefSelect func(path string)` — inserts `@path` into textarea

**New file:** `internal/tui/file_ref.go` — dialog implementation

**Key bindings:**
- `@` character → open file ref dialog
- `j/k` → navigate matches
- `Enter/Tab` → insert selected path and close
- `Esc` → close without inserting

**Prompt injection format:**
```
@path/to/file:
```go
<file contents>
```
```

### 2. `!` Shell Command Execution

**What it is:** Prefix input with `!` to run a shell command and feed its stdout into the prompt as context.

**How it works:**
- If input starts with `!`, treat the rest as a shell command to execute via `os/exec`
- Show a "Running..." indicator in the consensus panel
- Capture stdout (and optionally stderr) and append to the prompt before sending
- Syntax: `!ls -la`, `!git diff HEAD~1`
- Result is injected as:

```
$ <command>
<output>
```

**Model changes:**
- Detect `!` prefix in `updateChat` enter handler, before slash command check
- `toolStatus string` already exists — reuse for "Running shell command..."
- After execution, prepend output to prompt and continue normal submit flow

**Error handling:** If command fails (non-zero exit), append stderr to the output block and still submit (user should decide how to handle).

### 3. Leader-Key Keybindings

**What it is:** A `ctrl+x` leader key that prefixes a second key for common actions. Reduces chord fatigue and makes the UI feel more高手 (pro).

**Design:**
- `ctrl+x` alone does nothing — it arms the prefix state
- Second key must follow within 500ms, otherwise state resets
- If second key is unrecognized, state resets silently (no error shown)

**Initial bindings:**

| Key | Action |
|-----|--------|
| `ctrl+x s` | Open settings (`/settings`) |
| `ctrl+x m` | Open mode picker (`/mode`) |
| `ctrl+x p` | Open commands palette |
| `ctrl+x c` | Clear conversation (`/clear`) |
| `ctrl+x h` | Toggle help (`/help`) |
| `ctrl+x y` | Toggle yolo mode (`/yolo`) |
| `ctrl+x m` | Toggle MCP dashboard (`m`) |
| `ctrl+x .` | Repeat last prompt |

**Implementation:**
- Add `leaderKeyActive bool` and `leaderKeyDeadline time.Time` to Model
- In `updateChat`: on `ctrl+x`, set `leaderKeyActive = true` and `leaderKeyDeadline = now + 500ms`
- In `updateChat`: on any other key while `leaderKeyActive`:
  - If past deadline, reset state and pass key normally
  - Otherwise, look up binding, reset state, execute action
- Add `tea.SingleKey` timeout command: after 500ms, reset `leaderKeyActive`
- Show subtle `^X` indicator in tab bar when leader key is armed (briefly, like vim's leader indicator)

### 4. Commands Palette via `ctrl+p`

**What it is:** A fast-access palette (distinct from the slash command palette) triggered by `ctrl+p`, showing:
- Built-in commands (`/settings`, `/clear`, `/mode`, etc.)
- Custom user-defined commands (stored in config)
- MCP prompts (from connected servers' `prompts/list` results)

**How it differs from existing slash palette:**
- Slash palette (`/`) requires starting with `/`; `ctrl+p` can be triggered from anywhere
- Shows MCP prompts that aren't slash commands
- Supports custom user commands (arbitrary name + prompt template)
- Fuzzy filtering on name and description

**Model changes:**
- `paletteOpen bool` (repurposed — extend to support type: `slash` | `commands`)
- `paletteType string` — "slash" or "commands"
- `paletteCursor int`
- `customCommands []CustomCommand` — loaded from config
- `CustomCommand struct`: Name, Description, PromptTemplate string
- `mcpPrompts []MCPPrompt` — cached from `prompts/list` on MCP connect

**Config field (in config.yaml):**
```yaml
commands:
  - name: "Run tests"
    description: "Execute the test suite"
    prompt: "/plan run go test ./..."
  - name: "Git status"
    description: "Show working tree status"
    prompt: "!git status"
```

---

## Phase 2: Medium-Impact, Moderate-Effort

Features that require more implementation work but meaningfully improve trust and usability.

### 5. Plan Mode (Intent Preview)

**What it is:** Before executing any file-modifying tools (`file_write`, `file_edit`, `file_delete`), the consensus output shows a structured summary of what *will* change — e.g., "Will modify 3 files, delete 1 file, run 2 shell commands" — and pauses for user confirmation.

**How it works:**
- Primary model returns a `tool_calls` response with a special marker or the response includes structured intent
- TUI intercepts and renders the intent summary in the consensus panel
- User can `y` to approve, `n` to reject, or `e` to edit the proposed changes inline
- If approved, tool calls are executed; if rejected, the turn ends without modification

**Implementation approach (two options):**

**Option A — Structured intent from model (preferred):**
- System prompt instructs primary model to emit a structured "intent block" before tool calls:

```
[INTENT]
modify: src/main.go (replace function X)
modify: src/utils.go (add helper Y)
delete: src/old.go
run: go test ./...
[/INTENT]
```

- Action executor detects `[INTENT]...[/INTENT]` in model output before executing
- Renders as a diff-like preview panel
- Requires prompt engineering and model cooperation

**Option B — Tool use policy (simpler):**
- When a tool call is detected, pause and render: `Tool: file_write → path/to/file\n[diff or new content]\ny/n/e`
- Less elegant but more reliable across models

**Recommended:** Start with Option B as a v1, evolve to Option A once the intent detection is stable.

**Model changes:**
- `intentPending bool` — true when waiting for plan confirmation
- `intentContent string` — structured description of pending changes
- `intentToolCalls []ToolCall` — the tool calls to execute if confirmed
- `renderIntent()` method in view.go

**User flow:**
1. User submits prompt
2. Model returns with file-modifying tool calls
3. Instead of executing immediately, TUI shows intent summary
4. User approves (`y`) → tools execute normally
5. User rejects (`n`) → turn ends, no changes
6. User edits (`e`) → inline editor for the proposed changes, then approve/reject

### 6. Git-Backed Undo

**What it is:** File changes made during a session are tracked via `git stash`-style snapshots, allowing users to undo specific changes without clearing the whole conversation.

**How it works:**
- When tools modify files (`file_write`, `file_edit`, `file_delete`), call `git add <path>` after each change
- Maintain a session-scoped git branch: `polycode/<session-id>` created from HEAD on session start
- After each tool-modifying action, `git stash` the changes (creates a stash entry)
- On undo: `git stash pop` to revert the most recent change
- Maintain a stack of stash entries per session
- Multiple undos pop stash entries in LIFO order

**Model changes:**
- `undoStack []UndoEntry` — {path, description, stashIndex}
- `onUndo() func()` — pops last stash and restores file
- `gitBranch string` — session-specific branch name

**Commands:**
- `/undo` — revert last file-modifying action
- Show in provenance panel: "Undo stack: 3 changes — /undo to revert last"

**Constraints:**
- Requires the project to be a git repo (check with `git rev-parse --git-dir`)
- Requires `git` CLI available on PATH
- If not a git repo, disable undo silently (no error to user)
- Stash entries are session-scoped and can be dropped on session end

### 7. Render Caching + Viewport Optimization

**What it is:** Reduce CPU overhead during streaming by caching rendered markdown and only re-rendering when content actually changes in the visible viewport.

**Problems with current approach:**
- `renderMarkdown()` is called every ~500ms during streaming (update.go:531)
- Full markdown render on every tick is expensive for long responses
- No dirty tracking — the entire view re-renders regardless of what changed

**Changes:**
- Add a render cache keyed by content hash (md5 of raw text)
- On each `ConsensusChunkMsg` delta:
  - Compute new content hash
  - If hash unchanged, skip render (no-op)
  - If hash changed but content delta is small (< 100 chars), append to cached rendered output instead of full re-render
  - If content changed significantly (> 1000 chars since last full render), do a full render
- Track `lastRenderedLen int` to detect when accumulated deltas warrant a fresh render
- Result: streaming feels equally responsive but uses ~60-70% less CPU

**Implementation:**
- `markdownCache map[string]string` — hash → rendered HTML
- `markdownCacheMu sync.Mutex`
- Eviction: keep last 50 entries, LRU-style (or just cap at 100)
- Viewport optimization: only `GotoBottom()` if user is already at the bottom (don't fight manual scroll during streaming)

### 8. Sessions Manager Dialog

**What it is:** A dedicated overlay dialog (like the MCP dashboard) for browsing, selecting, renaming, and deleting sessions — with fuzzy filtering and a visible preview of the selected session's first message.

**How it works:**
- Triggered by `/sessions` with no args, or by `ctrl+p` → "Sessions"
- Shows a scrollable list of all sessions (from `~/.config/polycode/sessions/`)
- Each row: session name, date, first prompt (truncated)
- `j/k` to navigate, `Enter` to load selected session
- `r` to rename (inline input), `d` to delete (with confirmation)
- `/` to filter sessions by name or first prompt content
- At bottom: "Last active: <date>" and total session count

**Model changes:**
- `sessionsDialogOpen bool`
- `sessionsDialogCursor int`
- `sessionsDialogFilter string`
- `sessionsDialogItems []SessionItem` — loaded lazily on open
- `SessionItem struct`: ID, Name, FirstPrompt, LastModified time.Time

**Reuses existing:** `onSessions` callback is already wired — this is a UI pass over the existing session management flow.

---

## Phase 3: Larger Features, Higher Impact

Features that require significant work but transform the product.

### 9. Theme System

**What it is:** A configurable color palette and typography system with at least 3 built-in themes (dark, light, high-contrast) and support for user-defined themes via config.

**Theme structure:**
```yaml
theme:
  name: "dark"
  colors:
    primary: "#00ff00"        # consensus/success color
    secondary: "#63b3ed"      # links, selections
    background: "#1a1a1a"
    surface: "#2d2d2d"
    text: "#e0e0e0"
    dimmed: "#888888"
    error: "#ff6b6b"
    warning: "#f6e05e"
    border: "#404040"
    tabActive: "#00cc00"
    tabInactive: "#666666"
  fonts:
    mono: "JetBrains Mono"
    regular: "System"
```

**Model changes:**
- `theme Theme` — current active theme
- `Theme struct`: Name, Colors, Fonts
- Load theme from config on startup; fall back to hardcoded defaults
- `Styles` struct in Model recomputed from theme on `ConfigChangedMsg`

**Themes to include:**
1. **Dark** (default, current behavior) — dark background, green primary
2. **Light** — light background, dark text, blue primary (accessibility)
3. **High Contrast** — pure black/white, thick borders, maximum readability

**Implementation notes:**
- Use lipgloss color constants throughout (already the case)
- Replace hardcoded `lipgloss.Color("86")` etc. with `m.theme.Colors.Primary`
- Provide a `/theme <name>` command to switch at runtime

### 10. Non-Interactive / Headless Mode

**What it is:** `polycode run "prompt"` for scripting, CI pipelines, and automation. No TUI — just streams output to stdout and exits when done.

**How it works:**
- New `run` subcommand via Cobra: `polycode run [flags] "<prompt>"`
- Reads config from `~/.config/polycode/config.yaml` (no interactive setup)
- Initializes providers, MCP, and consensus pipeline
- Streams consensus output to stdout (no markdown rendering — raw text)
- Tool execution: runs with `yoloMode=true` by default (use `--confirm` flag to enable prompts)
- Exit codes: 0 on success, 1 on error, 2 on user-rejected tool call

**CLI surface:**
```
polycode run "explain this error" < error.log
polycode run "write a tests for src/*.go" --provider anthropic
polycode run "/plan refactor to use errors.Wrap" --confirm
```

**Implementation:**
- New `cmd/polycode/run.go` with `runCmd` subcommand
- `internal/app/run.go` — headless pipeline runner
- Reuse existing `Pipeline`, `Query`, tool execution logic
- No Bubble Tea model — just `fmt.Print` for output
- Config loading same as TUI mode

### 11. Auto-Scroll Toggle

**What it is:** During streaming, auto-scroll to bottom (current behavior). Add a toggle to disable auto-scroll so users can read ahead without the view jumping.

**How it works:**
- Track `autoScroll bool` — default `true`
- `ctrl+g` toggles auto-scroll mode
- When disabled: show a subtle indicator in the tab bar: `[scroll locked]`
- Re-enabling auto-scroll jumps to bottom immediately
- Useful for reading long explanations while streaming is still going

**Implementation:**
- `autoScroll bool` in Model
- In `ConsensusChunkMsg` handler: only call `m.consensusView.GotoBottom()` if `autoScroll`
- `ctrl+g` key handler in `updateChat` toggles and shows indicator
- Update help overlay with `ctrl+g` hint

### 12. Project Context Seeding

**What it is:** On first run in a project directory, analyze the codebase and generate `AGENTS.md` with discovered build commands, patterns, and conventions — so future sessions have rich context automatically.

**How it works:**
- On session start, check if `AGENTS.md` exists in the project root
- If not, and if project appears to be a code project (has `go.mod`, `package.json`, `Cargo.toml`, etc.), run a discovery pass:
  - Detect language/framework from lock files
  - Find build/test/lint commands from Makefile, package.json scripts, etc.
  - Scan for common patterns (error handling style, test file naming, etc.)
  - Read `.gitignore` and `.editorconfig` if present
- Generate `AGENTS.md` with findings
- User is shown: `"First time here — generated AGENTS.md with project context. Edit to refine."`
- User can regenerate with `/memory --regen`

**Output format (AGENTS.md):**
```markdown
# Project Context

## Language / Framework
Go 1.21+

## Build Commands
- Build: `go build ./...`
- Test: `go test ./... -count=1`
- Lint: `golangci-lint run`

## Conventions
- Errors wrapped with `fmt.Errorf("context: %w", err)`
- Tests colocated with source (`foo_test.go` next to `foo.go`)
- No abbreviations in function names

## Ignores
node_modules/, .git/, vendor/, dist/
```

**Implementation:**
- `internal/action/project_context.go` — already exists and has `ToolUsageHints()`
- Extend to include a `Discover()` method that runs the analysis
- Call from app startup before first prompt
- Gate behind a config flag: `autoSeed: true` (default)

---

## Implementation Phases Summary

| Phase | Feature | Effort | Impact |
|-------|---------|--------|--------|
| 1.1 | `@` file references | Low | High |
| 1.2 | `!` shell commands | Low | High |
| 1.3 | Leader-key bindings | Low | Medium |
| 1.4 | `ctrl+p` commands palette | Medium | High |
| 2.1 | Plan mode (intent preview) | Medium | High |
| 2.2 | Git-backed undo | Medium | High |
| 2.3 | Render caching | Medium | Medium |
| 2.4 | Sessions manager dialog | Medium | Medium |
| 3.1 | Theme system | High | Medium |
| 3.2 | Headless mode | High | High |
| 3.3 | Auto-scroll toggle | Low | Low |
| 3.4 | Project context seeding | Medium | Medium |

**Recommended order:** 1.1 → 1.2 → 1.3 → 2.1 → 1.4 → 2.2 → 2.3 → 2.4 → 3.2 → 3.1 → 3.3 → 3.4

---

## Files Modified Per Feature

### Phase 1

| Feature | Files |
|---------|-------|
| `@` file refs | `internal/tui/file_ref.go` (new), `internal/tui/model.go`, `internal/tui/view.go`, `internal/tui/update.go`, `internal/action/project_context.go` |
| `!` shell | `internal/tui/update.go`, `cmd/polycode/app.go` |
| Leader keys | `internal/tui/model.go`, `internal/tui/update.go`, `internal/tui/view.go` |
| `ctrl+p` palette | `internal/tui/model.go`, `internal/tui/view.go`, `internal/tui/update.go`, `internal/config/config.go` |

### Phase 2

| Feature | Files |
|---------|-------|
| Plan mode | `internal/tui/model.go`, `internal/tui/view.go`, `internal/tui/update.go`, `internal/action/executor.go`, `internal/action/tools.go` |
| Git undo | `internal/action/executor.go` (new), `internal/tui/model.go`, `internal/tui/update.go` |
| Render caching | `internal/tui/view.go` (renderMarkdown), `internal/tui/update.go` |
| Sessions dialog | `internal/tui/model.go`, `internal/tui/view.go`, `internal/tui/update.go`, `internal/tui/sessions.go` (new) |

### Phase 3

| Feature | Files |
|---------|-------|
| Theme system | `internal/tui/model.go`, `internal/tui/view.go`, `internal/tui/styles.go` (new), `internal/config/config.go` |
| Headless mode | `cmd/polycode/run.go` (new), `internal/app/run.go` (new) |
| Auto-scroll | `internal/tui/model.go`, `internal/tui/update.go` |
| Project seeding | `internal/action/project_context.go` |

---

## Open Questions

1. **Plan mode with Option A** requires prompt engineering. Should we invest in building a "plan mode" variant of the system prompt, or start with Option B (simpler, less reliable intent detection)?
2. **Git undo**: session-scoped stash entries could accumulate. Should we clean them up on session end (`/clear` or quit), or keep them for later reference?
3. **`@` file injection**: for large files, should we truncate? If so, what's the limit (token budget awareness vs. full file context)?
4. **`!` shell**: should we support streaming output live into the consensus panel, or wait for completion before injecting?
5. **Custom commands**: should these support variables like `{{selection}}` or `{{buffer}}` (current file content)?
