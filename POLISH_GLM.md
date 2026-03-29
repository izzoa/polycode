# Polycode UX/UI Polish Plan

Based on features from OpenCode, Crush, and modern TUI best practices, this document outlines concrete UX/UI improvements to enhance the polycode developer experience.

---

## Overview

This plan focuses on **developer productivity polish** — features that make the tool feel more like an integrated IDE companion and less like a simple chat interface. Each feature is designed to build upon existing polycode patterns (command palette, tab system, message types) while adding powerful new interaction models.

**Priority Framework:**
- **P0** - High impact, low complexity (quick wins)
- **P1** - High impact, medium complexity (strategic investments)
- **P2** - Medium impact, high complexity (future enhancements)
- **P3** - Nice-to-have, exploratory (research items)

---

## Feature Specifications

### 1. Prompt Stashing

**Priority:** P0
**Effort:** Low (2-3 days)
**Inspiration:** OpenCode's prompt stashing system

#### User Value
- **Context preservation:** Save partially-formed queries without losing them
- **Iterative refinement:** Work on complex prompts across multiple sessions
- **Experimentation:** Save and swap between different prompt approaches
- **Reduced cognitive load:** Don't need to retype or copy/paste between terminals

#### UX Design

**Stash Management (New overlay: `viewStash`)**
```
╔══════════════════════════════════════════╗
║ Saved Prompts                              ║
╠══════════════════════════════════════════╣
║ ▸ add-auth-flow-to-app        2h ago     ║
║   "Add OAuth2 authentication flow to       ║
║    the existing app using..."              ║
║                                           ║
║ ▸ refactor-user-service       1d ago     ║
║   "Refactor the user service to use      ║
║    repository pattern and..."              ║
║                                           ║
║ ▸ debug-api-latency         5d ago     ║
║   "Investigate why the API responses       ║
║    are taking >2s on average..."         ║
╚══════════════════════════════════════════╝

Actions: a:load  e:edit  d:delete  Enter:view  Esc:close
```

**Interaction Patterns:**
- `/stash save <name>` - Save current textarea content as a named stash
- `/stash list` - Open stash overlay (sorted by recency)
- `/stash load <name>` - Load a stash into textarea
- `/stash edit <name>` - Open a stash in textarea for editing (updates on save)
- `/stash delete <name>` - Remove a stash
- Arrow keys navigate, Enter loads selected stash

**Visual Feedback:**
- Show stash count in status bar when non-zero: `Stash: 3`
- Highlight recently modified stashes
- Show preview snippet (first 2-3 lines) in list view

#### Implementation

**Data Model (add to `model.go`):**
```go
type PromptStash struct {
    Name      string
    Content   string
    CreatedAt time.Time
    UpdatedAt time.Time
}

// Add to Model struct
stashes      []PromptStash
stashOverlay  bool
stashCursor  int
```

**New View Mode:**
```go
const (
    // ... existing modes
    viewStash viewMode = iota
)
```

**Storage:**
- Persist to `~/.config/polycode/stashes.json`
- Use file-based storage (simple, no schema changes)
- Auto-save on create/update/delete

**Slash Commands Integration:**
- Add to `slashCommands` list:
  - `/stash [save|list|load|edit|delete] <name>`
- Command palette filtering works naturally
- Tab-complete stash names when typing `/stash load <tab>`

**Key Handling (in `updateChat`):**
- New keys for stash overlay navigation
- Support `/stash name` pattern from chat input

**Persistence:**
- Load stashes on `Init()`
- Save to disk on every mutation
- Use `atomic` write pattern to avoid corruption

**File Structure:**
```
~/.config/polycode/
├── config.yaml
├── stashes.json        # NEW: prompt stashes
└── sessions/
    └── ...
```

---

### 2. Fuzzy File Search Autocomplete

**Priority:** P0
**Effort:** Low-Medium (3-4 days)
**Inspiration:** OpenCode's fuzzy file search

#### User Value
- **Faster file referencing:** Don't need to type full paths or remember exact names
- **Discoverability:** See available files while typing, not before
- **Reduced errors:** Avoid typos in file paths
- **Workflow integration:** Matches how developers think about files

#### UX Design

**Trigger Pattern:**
- Type `@` in textarea → shows fuzzy file matcher overlay
- Partial text after `@` filters the list (fuzzy matching)
- Tab or Enter accepts the selected file

**File Matcher Overlay:**
```
╔══════════════════════════════════════════╗
║ @ma                                       ║
╠══════════════════════════════════════════╣
║ internal/main.go              42 lines   ║
║ internal/middleware.go        128 lines  ║
║ internal/auth.go             215 lines  ║
║ api/main.go                 156 lines  ║
║ docs/api_main.go.md          45 lines   ║
╚══════════════════════════════════════════╝
Tab:accept  Enter:insert ↑↓:navigate  Esc:close
```

**Integration with Prompt:**
- After selection, file is inserted as `@path/to/file.go`
- Existing action executor already handles `@file` syntax
- Virtual text rendering shows compact attachment in textarea

**Fuzzy Matching Algorithm:**
- Case-insensitive substring match
- Prefer files in working directory over subdirectories
- Prefer `.go`, `.ts`, `.js` over other extensions
- Score by: match position, file type, recency (git status if available)

**Advanced Features (Phase 2):**
- Symbol search: `@main.go:processAuth` → shows functions/definitions
- Directory search: `@internal/` → shows all files in directory
- Git-aware: `@main.go:~10` → shows git diff context

#### Implementation

**New Message Types:**
```go
type FileMatch struct {
    Path    string
    RelPath string // relative to project root
    Size    int64
    Lines   int
    Type    string // "go", "ts", "js", etc.
}

type FileSearchMsg struct {
    Query     string // text after @
    Matches   []FileMatch
    Cursor    int
    IsVisible bool
}
```

**Model State:**
```go
fileMatcherVisible bool
fileMatcherQuery  string
fileMatcherMatches []FileMatch
fileMatcherCursor int
```

**File Discovery:**
- Use existing `list_directory` tool infrastructure
- Walk project directory on first `@` trigger
- Cache file tree (invalidate on 30s TTL or file system watcher)
- Respect `.gitignore`, `.polycodeignore` patterns

**Fuzzy Search Algorithm:**
```go
// Simple implementation for MVP
func fuzzyMatch(query string, candidates []FileMatch) []FileMatch {
    lowerQ := strings.ToLower(query)
    var scored []FileMatch
    for _, f := range candidates {
        if strings.Contains(strings.ToLower(f.RelPath), lowerQ) {
            scored = append(scored, f)
        }
    }
    // Sort by: exact prefix > start > anywhere
    return scored[:min(20, len(scored))]
}
```

**Textarea Integration:**
- Detect `@` character and trigger file search
- Show overlay on next render
- On Tab/Enter, insert file path and close overlay
- Handle backspace: if deleting `@`, close overlay

**Performance Considerations:**
- Cache file tree (don't walk on every keystroke)
- Debounce search (wait 150ms after keystroke before matching)
- Limit matches to top 20 results
- Use goroutine for file tree building (non-blocking)

**File Types Prioritization:**
```go
var preferredTypes = []string{"go", "ts", "tsx", "js", "jsx", "py", "rs", "java"}
func fileTypeScore(ext string) int {
    for i, t := range preferredTypes {
        if ext == t {
            return len(preferredTypes) - i // higher score for preferred
        }
    }
    return 0
}
```

---

### 3. Virtual Text Attachments

**Priority:** P1
**Effort:** Medium (4-5 days)
**Inspiration:** OpenCode's extmark attachment rendering

#### User Value
- **Clean prompts:** Show attachments as visual elements, not inline text
- **Rich metadata:** Display file size, line count, or snippet preview
- **Context awareness:** Always see what files are attached
- **Professional polish:** Makes the tool feel like an IDE

#### UX Design

**Attachment Representation:**
```
Prompt: @main.go @config/auth.yaml
──────────────────────────────────────────────────────────────────
┌─ 📄 main.go ───────────────────────────────────────────────┐
│ @internal/main.go                              156 lines   │
│ "Main application entry point with router setup..."         │
└──────────────────────────────────────────────────────────────┘

┌─ 📄 config/auth.yaml ───────────────────────────────────────┐
│ @config/auth.yaml                                 42 lines   │
│ "OAuth2 provider configuration and secrets..."              │
└──────────────────────────────────────────────────────────────┘

Add more files or press Enter to submit
```

**Attachment Parsing:**
- Detect `@filepath` patterns in textarea
- Extract file metadata using `file_info` tool
- Render as compact extmarks above textarea
- Support removal with `x` button on attachment

**Attachment Overlay (toggleable):**
```
Press `a` to show/hide attachment details
──────────────────────────────────────────────────────────────────
main.go (internal/)
  Size: 4.2 KB
  Lines: 156
  Modified: 2h ago
  Preview:
    package main

    import (
        "github.com/gin-gonic/gin"
    )

    func main() {
        r := gin.Default()
        r.Run(":8080")
    }
```

#### Implementation

**Attachment Model:**
```go
type Attachment struct {
    Path       string
    Metadata   struct {
        Size      int64
        Lines     int
        Modified  time.Time
        Language  string
    }
    Preview    string // first 10 lines
    PreviewErr error
}

// Add to Model
attachments   []Attachment
attachmentsVisible bool
```

**Attachment Detection:**
```go
func parseAttachments(text string) []string {
    // Find all @filepath patterns
    // Returns list of file paths
}
```

**Metadata Fetching:**
- Use existing `file_info` action tool
- Fetch on-demand (when attachment is added)
- Cache metadata (invalidate on 5min TTL)
- Show loading state while fetching

**Rendering (new view method):**
```go
func (m Model) renderAttachments() string {
    // Render compact extmarks for each attachment
    // Use lipgloss for nice borders and colors
    // Show file type icons based on extension
}
```

**Key Interactions:**
- Type `@` → trigger file matcher (see feature #2)
- After selection → add attachment to list
- Press `a` → toggle detailed attachment view
- `x` on attachment → remove it
- Backspace on `@` in textarea → remove last attachment

**Error Handling:**
- Show error icon for files that don't exist
- Show warning for files > 100KB (triggers summarization)
- Show lock icon for files without read permission

**Styling:**
```go
// Add to Styles
AttachmentBorder  lipgloss.Style
AttachmentHeader  lipgloss.Style
AttachmentPreview lipgloss.Style
AttachmentError  lipgloss.Style
```

---

### 4. TODO Extraction & Tracking

**Priority:** P1
**Effort:** Medium (3-4 days)
**Inspiration:** OpenCode's TODO sidebar, IDE task lists

#### User Value
- **Action tracking:** Don't lose action items in long agent responses
- **Clear next steps:** See what needs to be done at a glance
- **Progress tracking:** Mark items as done and see progress
- **Searchability:** Find specific TODOs across sessions

#### UX Design

**TODO Panel (toggleable with `t` key):**
```
╔══════════════════════════════════════════╗
║ Action Items (5 pending, 2 complete)       ║
╠══════════════════════════════════════════╣
║ Pending:                                   ║
║ ☐ [Response #3] Add authentication flow   ║
║ ☐ [Response #3] Add unit tests for auth   ║
║ ☐ [Response #5] Refactor user service     ║
║ ☐ [Response #5] Update API docs          ║
║ ☐ [Response #7] Fix timezone bug         ║
║                                           ║
║ Complete:                                  ║
║ ☑ [Response #1] Setup project structure   ║
║ ☑ [Response #2] Configure MCP servers     ║
╚══════════════════════════════════════════╝

Space:toggle  j/k:navigate  d:delete  Enter:view source
```

**TODO Extraction Patterns:**
- `TODO: ...` - Standard TODO comment
- `FIXME: ...` - High-priority fix
- `- [ ] ...` - Markdown task list
- `Action: ...` - Action item in agent response
- `Next: ...` - Next step instructions
- `Implement: ...` - Implementation task

**Slash Commands:**
- `/todo` - Toggle TODO panel
- `/todo list` - Show all TODOs
- `/todo clear <all|complete>` - Clear TODOs
- `/todo export` - Export TODOs to markdown

#### Implementation

**TODO Model:**
```go
type TodoItem struct {
    ID          string    // unique ID
    Text        string    // extracted text
    Source      string    // response/agent that created it
    ResponseIdx int       // which exchange
    Priority    string    // "high", "medium", "low"
    Status      string    // "pending", "complete"
    CreatedAt   time.Time
    CompletedAt time.Time
}

// Add to Model
todos       []TodoItem
todoPanelOpen bool
todoCursor   int
```

**Extraction Logic:**
```go
func extractTodos(response string, responseIdx int) []TodoItem {
    var todos []TodoItem

    // Pattern 1: TODO/FIXME comments
    todoRegex := regexp.MustCompile(`(?i)(TODO|FIXME|ACTION|NEXT|IMPLEMENT):\s*(.+)`)
    matches := todoRegex.FindAllStringSubmatch(response, -1)
    for _, m := range matches {
        priority := "medium"
        if strings.Contains(m[1], "FIXME") {
            priority = "high"
        }
        todos = append(todos, TodoItem{
            Text:     strings.TrimSpace(m[2]),
            Priority: priority,
            Status:   "pending",
        })
    }

    // Pattern 2: Markdown task lists
    taskListRegex := regexp.MustCompile(`- \[([ x])\]\s*(.+)`)
    // ... similar extraction

    return todos
}
```

**Integration with Conversation:**
- Extract TODOs from every `ConsensusChunkMsg.Done`
- Associate with response index for source linking
- Auto-extract on query completion

**TODO Panel View:**
```go
func (m Model) renderTodoPanel() string {
    // Group by status (pending/complete)
    // Show source reference (e.g., "[Response #3]")
    // Checkbox style for visual feedback
}
```

**Key Handling:**
- `t` - Toggle TODO panel (when tab bar focused)
- `Space` - Toggle complete/incomplete status
- `j/k` or `↑/↓` - Navigate TODOs
- `Enter` - Jump to source response in chat
- `d` - Delete TODO

**Persistence:**
- Save to `~/.config/polycode/todos.json`
- Per-session or global? Global is more useful
- Auto-save on status change
- Export to markdown for sharing

**TODO Export Format:**
```markdown
# Action Items

## High Priority
- [ ] Add authentication flow

## Medium Priority
- [ ] Add unit tests for auth
- [ ] Refactor user service

## Completed
- [x] Setup project structure
- [x] Configure MCP servers
```

---

### 5. Diff Summary Sidebar

**Priority:** P1
**Effort:** Low-Medium (2-3 days)
**Inspiration:** OpenCode's diff summary panel

#### User Value
- **Quick change overview:** See what files were modified at a glance
- **Validation:** Verify the tool didn't make unexpected changes
- **Workflow awareness:** Understand the scope of modifications
- **Professional integration:** Matches IDE diff patterns

#### UX Design

**Diff Summary Panel (auto-shows after tool actions):**
```
╔══════════════════════════════════════════╗
║ Changes (3 files, +42 -12 lines)           ║
╠══════════════════════════════════════════╣
║  internal/auth.go                            ║
║    +15 -2                                    ║
║    Added JWT token validation                  ║
║                                             ║
║  api/routes.go                               ║
║    +20 -5                                    ║
║    Added authentication middleware             ║
║                                             ║
║  config/auth.yaml                            ║
║    +7 -5                                     ║
║    Updated OAuth configuration                ║
╚══════════════════════════════════════════╝

Enter:view diff  d:dismiss  Esc:close
```

**Detailed Diff View (on Enter):**
```
internal/auth.go
────────────────────────────────────────────────────
@@ -25,6 +25,21 @@ func AuthMiddleware() gin.HandlerFunc {
     return func(c *gin.Context) {
         token := c.GetHeader("Authorization")
+        if token == "" {
+            c.JSON(401, gin.H{"error": "missing token"})
+            c.Abort()
+            return
+        }
+
+        // Validate JWT token
+        claims, err := validateToken(token)
+        if err != nil {
+            c.JSON(401, gin.H{"error": "invalid token"})
+            c.Abort()
+            return
+        }
+
+        c.Set("user_id", claims.UserID)
         c.Next()
     }
 }
────────────────────────────────────────────────────
↑/↓:navigate files  q:close
```

#### Implementation

**Diff Tracking Model:**
```go
type FileChange struct {
    Path        string
    Additions   int    // +lines
    Deletions   int    // -lines
    Summary     string // one-line description
    FullDiff    string // unified diff
    Timestamp   time.Time
}

// Add to Model
recentChanges []FileChange
diffPanelOpen bool
diffCursor   int
```

**Integration with Tool Execution:**
- After `ToolCallMsg` completes, generate diff for modified files
- Use existing `file_edit` and `file_write` action infrastructure
- Track changes per query/turn

**Diff Generation:**
```go
func generateDiff(oldContent, newContent, path string) FileChange {
    // Use github.com/sergi/go-diff/diffmatchpatch or similar
    // Calculate additions/deletions
    // Generate unified diff
    return FileChange{...}
}
```

**Storage:**
- Keep recent changes in memory (last 50 actions)
- Persist to session metadata for export
- Clear on `/clear` command

**Slash Commands:**
- `/diff` - Show diff summary panel
- `/diff <file>` - Show detailed diff for specific file
- `/diff export` - Export all changes as patch file

**UI Integration:**
- Auto-show diff panel after tool actions (fade after 10s or dismissible)
- Add diff badge to status bar: `Changes: +15 -8`
- Integrate with existing `ToolCallMsg` flow

---

### 6. Agent/Skill Mentions

**Priority:** P2
**Effort:** Medium-High (5-7 days)
**Inspiration:** OpenCode's agent mentions

#### User Value
- **Specialized routing:** Send specific tasks to specialized agents/skills
- **Flexibility:** Mix and match agents in a single query
- **Explicit control:** Know which agent is handling what
- **Discovery:** Learn about available agents through autocomplete

#### UX Design

**Mention Syntax:**
- `@agent-name:task description` - Route specific task to agent
- `@gpt4:review this code` - Use GPT-4 for code review
- `@claude:write tests` - Use Claude for test generation
- `@skill-name:invoke skill` - Invoke a specific skill

**Mention Autocomplete:**
```
Prompt: @g
──────────────────────────────────────────────────────────────────
Available Agents & Skills:
──────────────────────────────────────────────────────────────────
@gpt4           - Primary model (high reasoning)
@claude         - Anthropic Claude (great for code)
@gemini         - Google Gemini (fast, cost-effective)
@code-review    - Skill: Code review specialist
@test-gen      - Skill: Test generation
@doc-gen       - Skill: Documentation generator
@security      - Skill: Security analysis
──────────────────────────────────────────────────────────────────
Tab:complete  Enter:accept  ↑↓:navigate  Esc:close
```

**Routing Display:**
```
Querying...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
→ @claude:write unit tests for auth service
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Claude: "I'll write comprehensive unit tests for the auth service..."
```

#### Implementation

**Mention Parser:**
```go
type Mention struct {
    Name string        // "claude" or "code-review"
    Type string        // "agent" or "skill"
    Task string        // task description after colon
    Range [2]int      // start/end positions in prompt
}

func parseMentions(prompt string) []Mention {
    // Find all @name:task patterns
    // Return structured mentions
}
```

**Agent/Skill Registry:**
```go
type AgentMetadata struct {
    Name        string
    Description string
    Type        string // "provider" or "skill"
    ProviderID  string // for providers
    SkillID     string // for skills
}

// Add to Model
agents []AgentMetadata
```

**Autocomplete Integration:**
- Type `@` → shows agent/skill matcher (similar to file matcher)
- Filter by name/description
- Tab/Enter to insert mention

**Routing Logic:**
- Existing consensus engine already supports provider routing
- For skills: invoke skill before/after consensus
- For agents: route specific mention to designated provider

**UI Updates:**
- Show mention routing in query status
- Highlight which provider is handling which mention
- Display agent name in provider panel when routed

**Storage:**
- Mention metadata in Exchange struct
- Track which agent/skill handled each part of response
- Useful for provenance and debugging

---

### 7. Shareable Session Export

**Priority:** P2
**Effort:** Medium (3-4 days)
**Inspiration:** OpenCode's share links

#### User Value
- **Collaboration:** Share sessions with team members
- **Documentation:** Export sessions for docs/blogs
- **Debugging:** Share problematic sessions for support
- **Archiving:** Keep session snapshots for future reference

#### UX Design

**Export Formats:**
- JSON (machine-readable, re-importable)
- Markdown (human-readable, documentation)
- HTML (styled, shareable via web)
- Gist (upload to GitHub Gist API)

**Export Options:**
```
╔══════════════════════════════════════════╗
║ Export Session                              ║
╠══════════════════════════════════════════╣
║ Format:                                    ║
║ ▸ JSON      (re-importable)                ║
║   Markdown  (documentation)                ║
║   HTML      (web-viewable)                 ║
║   Gist      (GitHub sharing)               ║
║                                           ║
║ Options:                                   ║
║ [x] Include provider responses              ║
║ [x] Include provenance data               ║
║ [ ] Include file attachments              ║
║                                           ║
║ Destination:                               ║
║ ~/Desktop/polycode_session_2024-03-28.md   ║
║                                           ║
╚══════════════════════════════════════════╝

Enter:export  Esc:cancel
```

**Gist Integration:**
- Prompt for GitHub token (if not stored)
- Upload to GitHub Gists API
- Return gist URL for sharing
- Show success/error status

#### Implementation

**Export Formats:**

**JSON Export:**
```json
{
  "version": "1.0",
  "session": {
    "id": "uuid",
    "name": "Session Name",
    "created_at": "2024-03-28T10:30:00Z",
    "exchanges": [
      {
        "prompt": "...",
        "consensus_response": "...",
        "provider_responses": {
          "gpt4": "...",
          "claude": "..."
        },
        "provenance": {
          "confidence": "high",
          "agreements": [...],
          "minorities": [...]
        }
      }
    ]
  }
}
```

**Markdown Export:**
```markdown
# Polycode Session

**Date:** 2024-03-28 10:30:00
**Session:** Session Name

---

## Turn 1

### Prompt
Add OAuth2 authentication to the app

### Response
I'll add OAuth2 authentication using GitHub OAuth provider...

---

### Provenance
**Confidence:** high
**Providers:** gpt4, claude, gemini
**Agreements:**
- Use GitHub OAuth
- Implement middleware pattern

---

## Turn 2
...
```

**HTML Export:**
- Styled with CSS for nice rendering
- Collapsible sections for provider responses
- Syntax highlighting for code blocks
- Responsive design

**Export Wizard:**
- New view mode: `viewExport`
- Format selection (radio buttons)
- Options (checkboxes)
- Destination path or Gist upload
- Progress indicator during export

**Gist API Integration:**
```go
func uploadToGist(sessionData []byte, token string) (string, error) {
    // Upload to GitHub Gists API
    // Return gist URL
}
```

**Storage:**
- Save export history in `~/.config/polycode/exports/`
- Track exported sessions for re-export

**Slash Commands:**
- `/export` - Open export wizard
- `/export <path>` - Quick export as markdown to path
- `/export --gist` - Upload as gist
- `/export --json <path>` - Export as JSON

---

### 8. Enhanced Theme Support

**Priority:** P3
**Effort:** Medium (3-4 days)
**Inspiration:** OpenCode's theme improvements

#### User Value
- **Accessibility:** Better contrast for readability
- **Personalization:** Match user preferences
- **Comfort:** Dark/light mode for different environments
- **Visual polish:** More professional appearance

#### UX Design

**Theme Switcher:**
```
╔══════════════════════════════════════════╗
║ Themes                                     ║
╠══════════════════════════════════════════╣
║ ▸ Default Dark                     (current) ║
║ ▸ High Contrast Dark                          ║
║ ▸ Light Mode                                  ║
║ ▸ Dracula                                     ║
║ ▸ Catppuccin Mocha                            ║
║ ▸ Nord                                       ║
╚══════════════════════════════════════════╝

Enter:select  Esc:cancel
```

**Theme Configuration:**
```yaml
# config.yaml
ui:
  theme: "default-dark"
  # Or custom color overrides
  colors:
    primary: "#86b042"
    secondary: "#458588"
    accent: "#d3869b"
    background: "#282c34"
    foreground: "#abb2bf"
```

#### Implementation

**Theme Model:**
```go
type Theme struct {
    Name       string
    Background lipgloss.Color
    Foreground lipgloss.Color
    Primary    lipgloss.Color
    Secondary  lipgloss.Color
    Accent     lipgloss.Color
    Error      lipgloss.Color
    Success    lipgloss.Color
    Warning    lipgloss.Color
    Muted      lipgloss.Color
}

func (t Theme) ToStyles() Styles {
    return Styles{
        App:       lipgloss.NewStyle().Background(t.Background).Foreground(t.Foreground),
        StatusBar: lipgloss.NewStyle().Background(t.Muted).Foreground(t.Foreground),
        // ... map all colors to Styles struct
    }
}
```

**Built-in Themes:**
```go
var builtinThemes = map[string]Theme{
    "default-dark": Theme{
        Name:       "Default Dark",
        Background: "#1e1e1e",
        Foreground: "#d4d4d4",
        Primary:    "#4fc1ff",
        // ...
    },
    "high-contrast": Theme{
        Name:       "High Contrast",
        Background: "#000000",
        Foreground: "#ffffff",
        // Higher contrast colors
    },
    "light-mode": Theme{
        Name:       "Light Mode",
        Background: "#ffffff",
        Foreground: "#000000",
        // Light theme colors
    },
    // ... dracula, catppuccin, nord
}
```

**Theme Switching:**
```go
func (m *Model) SetTheme(name string) {
    if theme, ok := builtinThemes[name]; ok {
        m.styles = theme.ToStyles()
        m.cfg.UI.Theme = name
        m.cfg.Save()
    }
}
```

**Slash Commands:**
- `/theme` - Open theme switcher
- `/theme <name>` - Switch to specific theme
- `/theme list` - List available themes

**Auto-Detection:**
- Detect system light/dark mode (if possible)
- Prompt on first launch to select theme
- Respect `polycode.yaml` config

---

### 9. Keyboard Shortcut Hints in Status Bar

**Priority:** P3
**Effort:** Low (1-2 days)
**Inspiration:** IDE keyboard shortcut hints

#### User Value
- **Discoverability:** Learn shortcuts without opening help
- **Context awareness:** See relevant shortcuts for current mode
- **Reduced friction:** No need to memorize all shortcuts

#### UX Design

**Dynamic Status Bar Hints:**
```
Chat mode:  Enter:send  /:commands  Ctrl+S:settings  ?:help
──────────────────────────────────────────────────────────────────
Settings mode:  j/k:nav  a:add  e:edit  d:delete  Esc:close
──────────────────────────────────────────────────────────────────
Stash mode:  Enter:load  e:edit  d:delete  Esc:close
──────────────────────────────────────────────────────────────────
```

**Context-Sensitive Hints:**
- Change based on current view mode
- Show most relevant actions for current context
- Prioritize frequently used shortcuts

#### Implementation

**Hint Manager:**
```go
type HintSet struct {
    Hints []Hint
}

type Hint struct {
    Key    string // "Enter", "Ctrl+S", etc.
    Action string // "send", "settings", etc.
}

func (m Model) getCurrentHints() HintSet {
    switch m.mode {
    case viewChat:
        return HintSet{
            Hints: []Hint{
                {"Enter", "send"},
                {"/", "commands"},
                {"Ctrl+S", "settings"},
            },
        }
    case viewSettings:
        return HintSet{...}
    // ... per mode
    }
}
```

**Status Bar Update:**
- Add hints to existing status bar rendering
- Show as right-aligned text
- Dynamic based on mode/context
- Shorten hints if space is limited

**Configuration:**
- Allow user to customize hints in config
- Option to disable hints entirely

---

### 10. Multi-Session Parallelism

**Priority:** P3 (Exploratory)
**Effort:** High (10-15 days)
**Inspiration:** OpenCode's multi-session support

#### User Value
- **Context switching:** Work on multiple tasks simultaneously
- **Comparison:** Compare different approaches side-by-side
- **Flexibility:** Don't lose context when switching tasks
- **Parallel workflows:** Run multiple agents concurrently

#### UX Design

**Tab Interface for Sessions:**
```
╔══════════════════════════════════════════╗
║ [Session 1★] [Session 2] [Session 3] [+] ║
╠══════════════════════════════════════════╣
║                                           ║
║ (Session 1 content)                       ║
║                                           ║
╚══════════════════════════════════════════╝
```

**Session Management:**
- `Ctrl+T` - New session
- `Ctrl+W` - Close current session
- `Ctrl+Tab` - Switch between sessions
- Right-click/long-press - Session menu (rename, duplicate, export)

#### Implementation

**Session Manager:**
```go
type Session struct {
    ID       string
    Name     string
    History  []Exchange
    State    SessionState // model snapshot
    Active   bool
}

type SessionManager struct {
    Sessions []Session
    Active   int // index of active session
}

// Add to Model
sessionManager *SessionManager
```

**Tab Bar Rendering:**
- New tab bar component
- Shows all sessions
- Star indicates primary/favorite
- Close button (x) on non-active sessions

**State Preservation:**
- Each session maintains its own model state
- Query status per session
- Independent provider panels per session

**Storage:**
- Multi-session file structure:
  ```
  ~/.config/polycode/sessions/
  ├── session-1.json
  ├── session-2.json
  └── session-3.json
  ```
- Load/save individual sessions

**Architectural Considerations:**
- **Pipeline isolation:** Separate consensus pipeline per session
- **Resource sharing:** Providers can be shared (same connections)
- **UI updates:** Session-specific message routing
- **Memory/session management:** Independent per session

**Slash Commands:**
- `/session new <name>` - Create new session
- `/session switch <name>` - Switch to session
- `/session close` - Close current session
- `/session list` - Show all sessions

**Note:** This is a significant architectural change. Consider as future enhancement after core polish features are implemented.

---

## Implementation Roadmap

### Phase 1: Quick Wins (Week 1-2)
**Goal:** High-impact features with low complexity

1. **Prompt Stashing** (P0, 2-3 days)
   - Data model + storage
   - Stash overlay view
   - Slash commands
   - File persistence

2. **Keyboard Shortcut Hints** (P3, 1-2 days)
   - Hint manager
   - Status bar integration
   - Context-sensitive hints

**Deliverables:**
- Stashed prompts save/load workflow
- Status bar shows context-sensitive shortcuts

---

### Phase 2: File Interaction (Week 2-3)
**Goal:** Better file handling and attachments

3. **Fuzzy File Search Autocomplete** (P0, 3-4 days)
   - File matcher overlay
   - Fuzzy search algorithm
   - File tree caching
   - Trigger detection

4. **Virtual Text Attachments** (P1, 4-5 days)
   - Attachment model
   - Metadata fetching
   - Extmark rendering
   - Attachment panel

**Deliverables:**
- Type `@` to search files
- Attachments show as compact extmarks
- File metadata displayed

---

### Phase 3: Action Tracking (Week 3-4)
**Goal:** Track and visualize actions

5. **TODO Extraction & Tracking** (P1, 3-4 days)
   - TODO extraction patterns
   - TODO panel
   - Persistence
   - Status toggling

6. **Diff Summary Sidebar** (P1, 2-3 days)
   - File change tracking
   - Diff generation
   - Summary panel
   - Detailed diff view

**Deliverables:**
- Auto-extract TODOs from responses
- Show diff summaries after tool actions
- Manage TODOs through panel

---

### Phase 4: Advanced Features (Week 5-6)
**Goal:** Specialized routing and sharing

7. **Agent/Skill Mentions** (P2, 5-7 days)
   - Mention parsing
   - Agent registry
   - Autocomplete
   - Routing logic

8. **Shareable Session Export** (P2, 3-4 days)
   - Export wizard
   - Multiple formats (JSON, Markdown, HTML, Gist)
   - Styling
   - Gist API integration

**Deliverables:**
- `@agent:task` syntax for routing
- Export sessions in multiple formats
- Share sessions via Gist

---

### Phase 5: Polish & Exploration (Week 7-8)
**Goal:** Visual polish and exploration

9. **Enhanced Theme Support** (P3, 3-4 days)
   - Theme model
   - Built-in themes
   - Theme switcher
   - Custom theme config

10. **Multi-Session Parallelism** (P3, 10-15 days)
    - Session manager
    - Tab interface
    - State preservation
    - Storage refactor

**Deliverables:**
- Multiple built-in themes
- Theme customization
- Multi-session support (if time permits)

---

## Architecture Considerations

### Existing Patterns to Leverage

1. **Message Types:**
   - All existing TUI features use message-based communication
   - New features should follow `Model.Update(msg)` pattern
   - Define new message types for feature-specific events

2. **View Modes:**
   - Already have `viewSettings`, `viewAddProvider`, etc.
   - Add new modes: `viewStash`, `viewExport`, `viewThemes`
   - Keep mode switching logic consistent

3. **Command Palette:**
   - All slash commands go through existing palette
   - Add new commands to `slashCommands` list
   - Palette filtering works automatically

4. **File Storage:**
   - Config already uses YAML for persistence
   - JSON is simpler for stashes/TODOs (no schema validation needed)
   - Use atomic writes to avoid corruption

5. **Overlay Pattern:**
   - Help, MCP dashboard, mode picker use overlays
   - Stash list, TODO panel, export wizard should follow same pattern
   - Overlay intercepts all key events when open

### New Components Needed

1. **Fuzzy Search Engine:**
   - File matcher
   - Agent/skill matcher
   - Consider `github.com/sahilm/fuzzy` or custom implementation

2. **Diff Engine:**
   - Unified diff generation
   - Consider `github.com/sergi/go-diff/diffmatchpatch`

3. **Markdown/HTML Renderer:**
   - For export formatting
   - Consider `github.com/alecthomas/chroma` for syntax highlighting

4. **Gist API Client:**
   - For GitHub Gist integration
   - Simple HTTP client with OAuth token auth

### Performance Considerations

1. **File Tree Caching:**
   - Walk project directory once, cache for 30s
   - Invalidate on file system change (if implementing watcher)
   - Don't block UI with file I/O

2. **TODO Extraction:**
   - Regex matching is fast enough for typical responses
   - Consider limiting extraction to last N turns

3. **Diff Generation:**
   - Only generate diffs for files actually modified
   - Cache old content in memory per query
   - Generate diffs in goroutine to avoid blocking

4. **Attachment Metadata:**
   - Fetch on-demand (don't pre-fetch all attachments)
   - Cache metadata (5min TTL)
   - Show loading state while fetching

### Storage Schema

**Stashes (`stashes.json`):**
```json
{
  "version": "1.0",
  "stashes": [
    {
      "name": "add-auth-flow",
      "content": "Add OAuth2 authentication to the app...",
      "created_at": "2024-03-28T10:30:00Z",
      "updated_at": "2024-03-28T10:35:00Z"
    }
  ]
}
```

**TODOs (`todos.json`):**
```json
{
  "version": "1.0",
  "todos": [
    {
      "id": "uuid",
      "text": "Add authentication flow",
      "source": "Response #3",
      "response_idx": 3,
      "priority": "medium",
      "status": "pending",
      "created_at": "2024-03-28T10:30:00Z"
    }
  ]
}
```

**Themes (optional `themes.yaml`):**
```yaml
themes:
  custom-theme:
    primary: "#86b042"
    secondary: "#458588"
    accent: "#d3869b"
```

---

## Testing Strategy

### Unit Tests
- Fuzzy search algorithm
- TODO extraction patterns
- Mention parsing
- Diff generation
- Theme color mapping

### Integration Tests
- Stash save/load/persistence
- Attachment detection and metadata fetching
- TODO tracking across multiple exchanges
- Export formats (JSON, Markdown, HTML)
- Gist API upload

### UI/UX Tests
- Manual testing of all overlays
- Keyboard shortcut discovery
- Visual polish in different terminal sizes
- Theme rendering verification
- Accessibility (contrast, readability)

### Performance Tests
- File search on large projects (1000+ files)
- TODO extraction on long conversations (100+ turns)
- Diff generation on large files (10K+ lines)
- Stash/TODO persistence under load

---

## Dependencies

### External Libraries

**Optional (consider adding):**
- `github.com/sahilm/fuzzy` - Fuzzy search (or implement custom)
- `github.com/sergi/go-diff/diffmatchpatch` - Diff generation
- `github.com/alecthomas/chroma` - Syntax highlighting for exports
- `github.com/google/uuid` - UUID generation for TODO/session IDs

**Already Used:**
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/microcosm-cc/bluemonday` - HTML sanitization

### Go Version
- Minimum: Go 1.20 (current polycode version)
- Consider Go 1.21+ for improved stdlib

---

## Risks & Mitigations

### Technical Risks

1. **Performance on Large Projects**
   - Risk: File search slow with 10K+ files
   - Mitigation: Implement caching, debouncing, pagination

2. **Memory Usage**
   - Risk: Storing many TODOs/stashes increases memory
   - Mitigation: Limit in-memory cache, lazy-load from disk

3. **Terminal Compatibility**
   - Risk: Complex rendering breaks on old terminals
   - Mitigation: Fallback to simple rendering, test on various terminals

4. **File System Watching**
   - Risk: Watching file system is platform-dependent
   - Mitigation: Use `fsnotify` library, graceful degradation

### UX Risks

1. **Feature Bloat**
   - Risk: Too many features overwhelm users
   - Mitigation: Prioritize P0/P1 features, hide P2/P3 behind opt-in

2. **Keyboard Shortcut Conflicts**
   - Risk: New shortcuts conflict with existing ones
   - Mitigation: Audit all shortcuts, document conflicts

3. **Discovery Issues**
   - Risk: Users don't know new features exist
   - Mitigation: Add hints in status bar, show onboarding tips on first use

### Data Risks

1. **Data Loss**
   - Risk: Stash/TODO corruption on crash
   - Mitigation: Atomic writes, backup strategy

2. **Privacy**
   - Risk: Gist export exposes sensitive data
   - Mitigation: Warning prompt, option to exclude sensitive content, encrypt locally

3. **Export Security**
   - Risk: HTML export vulnerable to XSS
   - Mitigation: Use `bluemonday` for sanitization

---

## Success Metrics

### Adoption
- **Feature Usage:** Track how often stashing, TODO tracking, file search are used
- **Session Duration:** Compare before/after polish features (expect increase)
- **User Feedback:** Qualitative feedback on UX improvements

### Performance
- **Startup Time:** Should remain <500ms (excluding session load)
- **Search Latency:** File search <100ms for 1000 files
- **Memory Usage:** Should not increase significantly (>10MB) per feature

### Quality
- **Test Coverage:** Maintain >80% coverage for new features
- **Bug Reports:** Track bugs introduced by new features
- **User Satisfaction:** Survey users on polish features (goal: 4.5/5 stars)

---

## Conclusion

This plan provides a comprehensive roadmap for UX/UI polish in polycode, building on existing patterns while adding powerful new interaction models. By implementing features in priority order (P0 → P3), we can deliver maximum value with minimal risk.

**Key Principles:**
- Build on existing patterns (message types, view modes, command palette)
- Prioritize developer productivity (stashing, file search, TODO tracking)
- Maintain performance and compatibility
- Provide clear upgrade path (no breaking changes)

**Next Steps:**
1. Review and prioritize features based on team bandwidth
2. Estimate effort more precisely for Phase 1 features
3. Set up branch strategy for incremental delivery
4. Begin implementation with Prompt Stashing (P0)

---

*Document Version: 1.0*
*Last Updated: 2024-03-28*
*Author: Generated via Crush AI Assistant*
