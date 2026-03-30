## Context

`cmd/polycode/app.go` is a 2,428-line file dominated by `startTUI()`, which creates ~26 `model.Set*Handler` closures that capture local variables (state, mcpH, conv, program, etc.). The closures fall into natural groups: config/provider management, MCP commands, session management, and the query pipeline. Helper functions at the bottom (`sendMCPStatus`, `summarizeConversation`, `toSessionMessages`, etc.) also cluster by domain.

`internal/tui/update.go` is 2,927 lines. The top-level `Update()` method dispatches on message type, then delegates to mode-specific handlers (`updateChat`, `updateSettings`, `updateWizard`, etc.) which are all defined in the same file. Each handler is 200-600 lines.

## Goals / Non-Goals

**Goals:**
- Reduce each file to ≤500 lines by splitting into focused siblings
- Keep all functions in their current packages (no new packages)
- Group by domain/concern, not alphabetically
- Maintain identical behavior — this is a zero-change refactor
- Make `app.go` ready for pipeline extraction (headless-mode prerequisite)

**Non-Goals:**
- Extracting the query pipeline into a new package (that's headless-mode scope)
- Changing function signatures or interfaces
- Adding new tests (existing tests must continue to pass as-is)
- Refactoring internal logic within any function

## Decisions

### 1. app.go split into 5 files

**Decision**: Split `cmd/polycode/` into:
- `app.go` — `startTUI()` orchestration: initialization, model creation, `program.Run()`. Target ~300 lines.
- `app_handlers.go` — All `model.Set*Handler` closures for provider test, plan, skill, mode change, undo, redo, yolo, shell context, cancel, clear, save, export, share.
- `app_mcp.go` — MCP handler closure, `sendMCPStatus()`, `sendMCPDashboardData()`, `wireMCPNotifications()`, MCP test/reconnect/dashboard/registry handlers.
- `app_session.go` — Session handler closures, `toSessionMessages()`, `fromSessionMessages()`, auto-name handler, session picker, `/sessions` command.
- `app_query.go` — `SetSubmitHandler` closure (the query pipeline), `summarizeConversation()`.

**Rationale**: These groups have minimal cross-references. Each file captures one domain. The query pipeline is the largest single closure (~800 lines) and gets its own file to set up headless-mode extraction.

### 2. update.go split into 5 files

**Decision**: Split `internal/tui/` handlers into:
- `update.go` — Top-level `Update()` dispatcher and shared message type handling. Target ~400 lines.
- `update_chat.go` — `updateChat()` and chat-mode key handling.
- `update_approval.go` — Approval prompt key handling and confirmation flow.
- `update_settings.go` — `updateSettings()`, `updateAddProvider()`, `updateEditProvider()`.
- `update_palette.go` — Command palette and file picker key handling.

**Rationale**: Each mode handler is self-contained. The existing `updateChat`, `updateSettings`, etc. function boundaries are the natural split points.

### 3. Shared state stays as closure captures

**Decision**: Handler closures in the split `app_*.go` files still close over the same local variables (`state`, `mcpH`, `conv`, `program`, etc.) from `startTUI()`. This means the closures must be defined inside `startTUI()` — the split files contain helper functions called from within `startTUI()`, not standalone wiring.

**Alternative considered**: Pass dependencies explicitly. Rejected because it would change function signatures and expand scope beyond a pure refactor.

**Practical approach**: Define named functions like `wireHandlers(model, state, ...)` in each `app_*.go` file, called from `startTUI()`. This moves the bulk of the code out while keeping the closure wiring in one place.

## Risks / Trade-offs

- **[Risk] Closures reference local variables** — Handler closures capture `state`, `mcpH`, `conv`, `program`, etc. Moving them to separate files requires either keeping them inside `startTUI()` (awkward split) or extracting setup functions that take these as parameters. Mitigation: use setup functions with explicit parameters.
- **[Risk] Merge conflicts with in-flight work** — This touches the two most-edited files. Mitigation: do this early, before new feature branches diverge.
- **[Risk] IDE navigation changes** — Developers used to finding code in `app.go` will need to adjust. Mitigation: consistent naming makes `grep`/`gf` reliable.
