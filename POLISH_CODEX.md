# Polycode UX/UI Polish Plan

## Purpose

This document turns the current UX/UI polish discussion into an execution plan for Polycode.

The goal is not to make Polycode look more like every other coding TUI. The goal is to make Polycode feel more finished, more legible, and more confidently multi-model than single-model tools like Crush or the earlier OpenCode lineage.

## Product Thesis

Polycode already has the hard part:

- parallel fan-out
- consensus synthesis
- provider traces
- provenance
- session persistence
- MCP integration
- tool execution
- verification

The polish gap is that too much of that state is hidden, command-driven, or visually under-explained.

The strongest UX direction for Polycode is:

1. Make consensus visible, not magical.
2. Make runtime state legible at a glance.
3. Replace chat-output-as-control-surface with navigable UI.
4. Preserve terminal-native speed and density.
5. Borrow proven ergonomics from Crush/OpenCode, but adapt them to a consensus-first product.

## Benchmark-Inspired Opportunities

The useful features to borrow from tools like Crush and OpenCode are not their exact layouts. They are their operational ergonomics.

### Crush-style strengths worth adapting

- session-based workflow as a first-class concept
- compact/dense terminal presentation
- long-running task ergonomics
- desktop notifications for approvals and completion
- stronger sense of application state beyond the current message buffer

### OpenCode-style strengths worth adapting

- command launcher as a primary interaction surface
- custom workflows and parameterized commands
- file-change tracking during a session
- richer permission handling
- external-editor compose flow
- explicit compaction/session-management affordances

### Polycode-specific twist

Polycode should not simply replicate those features. Each borrowed feature should answer:

- how does this improve trust in consensus?
- how does this help users compare model behavior?
- how does this reduce friction around multi-provider workflows?

## Current Product Strengths

Polycode already has a better substrate than a lot of terminal tools.

### Existing strengths

- Provider tabs and phase-aware traces already exist.
- Consensus provenance already exists.
- Sessions are already persisted and restored automatically.
- Tool approvals already exist.
- Token tracking already exists.
- Auto-compaction already exists.
- MCP status, dashboards, and wizards already exist.
- Agent-team progress already exists for `/plan`.

### Existing UX liabilities

- Important state is scattered across overlays, slash commands, and chat text.
- Settings are functional but visually flat.
- Session management exists in storage and commands, but not in a first-class browser.
- Tool execution is visible as text, not as a structured change workflow.
- Permission UX is binary and under-informative.
- The command palette is a filter shortcut, not a real launcher.
- The app has no strong compact mode or information-density modes.
- Turn-level artifacts are not grouped into a clear timeline.

## Core UX Principles

These should govern every polish decision.

### 1. State Should Be Observable

Users should always be able to answer:

- what is running?
- which providers are participating?
- what tools ran?
- what files changed?
- whether verification passed?
- what the consensus trusted or ignored?

### 2. Consensus Should Be Inspectable

Consensus should feel earned, not opaque.

Users should be able to inspect:

- provider participation
- areas of agreement
- minority viewpoints
- skipped providers
- synthesis timing
- evidence and confidence

### 3. Actions Should Form a Workflow

Tool use should not appear as stray lines in the transcript. It should feel like a structured workflow with:

- pending action
- approval
- execution
- changed files
- verification
- final result

### 4. Navigation Should Beat Memory

Anything important enough to expose via slash command should also be discoverable through the UI.

### 5. Density Should Be Intentional

Polycode is a terminal app. It should support both:

- a comfortable mode for new users
- a dense mode for daily operators

## Strategic Initiatives

The following initiatives are ordered by product leverage, not by implementation ease.

---

## Initiative 1: Information Architecture Refresh

### Why

The current chat view is structurally correct but visually thin. The main composition in `internal/tui/view.go` is a vertical stack of status bar, content, optional overlays, and input. That works, but it underuses the terminal as a dashboard.

### Outcome

Polycode should feel like a workspace, not just a transcript with tabs.

### Deliverables

- introduce layout modes: `comfortable` and `compact`
- create a persistent runtime status strip for:
  - mode
  - active providers
  - approval mode
  - MCP health
  - current session name
  - context pressure
- add a contextual footer that changes by focus state
- visually separate transcript content from runtime metadata
- give provider/comparison views a stronger identity than simple tab swaps

### UX shape

- top bar: app state
- center: transcript, comparison pane, or provider pane
- bottom auxiliary area: timeline, changes, approvals, or input
- footer: active shortcuts for the current mode

### Code touch points

- `internal/tui/view.go`
- `internal/tui/model.go`
- `internal/tui/update.go`

### Acceptance criteria

- a user can identify the current operating mode and approval mode without opening help
- a user can tell whether MCP is healthy without opening the MCP dashboard
- dense mode fits materially more information without unreadable clipping

---

## Initiative 2: Session Browser And Branching UX

### Why

Polycode has solid session persistence in `internal/config/session.go` and resume logic in `cmd/polycode/app.go`, but the UX is still command-oriented. The user experience is closer to "dump session info into chat" than "work across sessions."

### Outcome

Sessions should become an explicit workflow object.

### Deliverables

- session browser overlay or dedicated panel
- session preview:
  - name
  - last updated
  - exchange count
  - providers used
  - current project marker
- quick actions:
  - open
  - rename
  - duplicate
  - delete
  - export
  - branch from current turn
- optional "recent sessions" launcher
- visible current-session label in the main UI

### Important extension

Branching matters more for Polycode than for single-model tools because users will want to compare:

- a consensus line of inquiry
- a tool-heavy implementation branch
- a review-only branch
- alternate provider mixes or operating modes

### Data model additions

- optional session metadata for:
  - summary
  - provider set
  - last mode
  - branch parent
  - branch source exchange index

### Code touch points

- `internal/config/session.go`
- `cmd/polycode/app.go`
- `internal/tui/model.go`
- `internal/tui/view.go`
- `internal/tui/update.go`

### Acceptance criteria

- session switching is possible without slash commands
- branching a session does not feel like manual export/import work
- users can understand the relationship between current session and historical branches

---

## Initiative 3: Changes Drawer And Turn Artifact Timeline

### Why

This is likely the single biggest polish win.

Polycode already knows about tool calls, tool results, provider traces, and verification. Today that information is mostly serialized into chat or provider tabs. That is technically sufficient and experientially weak.

### Outcome

Each turn should produce a structured artifact set that can be inspected after the fact.

### Deliverables

- turn timeline showing:
  - prompt submitted
  - fan-out started
  - providers completed/failed/skipped
  - synthesis started
  - tools requested
  - approvals granted/denied
  - files changed
  - verification result
  - final answer
- changes drawer showing:
  - touched files
  - add/remove counts
  - per-file status
  - expandable diff hunks
- verification badge per turn:
  - not run
  - running
  - passed
  - failed

### UX behavior

- while a turn runs, timeline items appear live
- after completion, the turn remains inspectable from history
- selecting an old exchange restores not only transcript state but also artifacts

### Architectural note

This likely warrants an explicit turn-artifact model instead of relying on ad hoc message reconstruction.

### Code touch points

- `cmd/polycode/app.go`
- `internal/action/executor.go`
- `internal/action/file_ops.go`
- `internal/action/shell.go`
- `internal/tui/model.go`
- `internal/tui/update.go`
- `internal/config/session.go`

### Acceptance criteria

- users can answer "what changed?" without re-reading the transcript
- users can answer "what happened during this turn?" from a single screen
- tool-heavy turns no longer feel like noisy chat transcripts

---

## Initiative 4: Permission And Approval UX Overhaul

### Why

The current confirmation UI is safe but primitive. It asks for approval without enough context and only supports approve or reject.

That is below the quality bar of serious coding assistants.

### Outcome

Approvals should feel explicit, informative, and controllable.

### Deliverables

- richer approval dialog that shows:
  - tool name
  - target files or command
  - risk level
  - reason from the model
  - whether policy or yolo mode affects it
- approval actions:
  - allow once
  - deny once
  - allow for session
  - deny for session
  - always allow read-only tools
  - always deny this tool
- command preview and file-change preview where possible
- approval history in the timeline

### Product note

Polycode has an opportunity to do this better than peer tools because it can show:

- which provider requested the action
- whether the action emerged from consensus
- what minority or provenance context existed at the time

### Code touch points

- `internal/tui/view.go`
- `internal/tui/update.go`
- `cmd/polycode/app.go`
- permissions policy integration already wired in app setup

### Acceptance criteria

- a user can distinguish between low-risk read-only and high-risk mutating actions
- policy state is understandable from the approval UI
- fewer users need to rely on yolo mode to reduce friction

---

## Initiative 5: Command Launcher 2.0 And Workflow Templates

### Why

Polycode's current slash palette is useful but shallow. It filters command names and inserts text. That is not the same thing as a command launcher.

### Outcome

The command surface should feel fast, discoverable, and composable.

### Deliverables

- dedicated launcher shortcut such as `Ctrl+K`
- cursorable launcher, not just first-match accept
- grouped commands:
  - navigation
  - sessions
  - MCP
  - workflows
  - project actions
- parameter prompts for commands with arguments
- recent commands and favorites
- project-scoped workflow templates
- user-scoped workflow templates

### Template examples

- "Prime repo context"
- "Review staged changes"
- "Plan implementation"
- "Investigate failing test"
- "Summarize current branch"

### Stretch feature

Named parameters for workflows, inspired by OpenCode custom commands, would be high leverage.

### Code touch points

- `internal/tui/model.go`
- `internal/tui/view.go`
- `internal/tui/update.go`
- optional new config storage for command templates

### Acceptance criteria

- important app actions are reachable without remembering exact slash syntax
- commands with required arguments do not force the user back into raw text entry
- workflows can be reused across projects

---

## Initiative 6: Settings, Wizard, And MCP Management Redesign

### Why

Polycode's settings and wizard flows are capable, but they still read like admin tables. They do not yet communicate confidence, health, metadata quality, or relative impact.

### Outcome

Configuration should feel operational, not clerical.

### Deliverables

- two-pane settings view:
  - left: provider/MCP list
  - right: selected-item details and actions
- stronger status badges:
  - connected
  - auth missing
  - unhealthy
  - primary
  - read-only
- inline model metadata:
  - context window
  - reasoning support
  - attachment support
  - pricing if available
- more explicit primary-provider editing flow
- better wizard progress and validation messaging
- better MCP server details:
  - tools
  - read-only hints
  - env metadata
  - registry source

### Important detail

The settings screen should stop behaving like a dead-end list and start behaving like a control center.

### Code touch points

- `internal/tui/settings.go`
- `internal/tui/wizard.go`
- `internal/tui/mcp_wizard.go`
- `internal/tui/model.go`

### Acceptance criteria

- a new user can understand provider health without reading docs
- editing provider or MCP configuration requires less recall
- connection-testing results are visually clearer and more localized

---

## Initiative 7: Context Pressure, Token, And Compaction UX

### Why

Polycode already tracks token usage and auto-compacts when context pressure rises, but that system is mostly invisible unless you inspect the code or infer it from behavior.

### Outcome

Users should understand when the app is nearing context limits and what Polycode is doing about it.

### Deliverables

- visible context-pressure meter
- per-provider token summary with better affordances
- compaction event surfaced in timeline/history
- manual compact action
- pre-compaction warning in long sessions
- summary preview showing what was compressed

### Code touch points

- `cmd/polycode/app.go`
- `internal/tui/model.go`
- `internal/tui/view.go`
- `internal/tokens/*`

### Acceptance criteria

- users are not surprised by context compression
- compaction feels like an assistive feature, not hidden behavior
- token data is useful rather than decorative

---

## Initiative 8: Long-Running Task Ergonomics

### Why

Consensus, tool loops, MCP calls, and `/plan` jobs can all run long enough to warrant a more deliberate UX.

### Outcome

Long-running work should feel supervised rather than stalled.

### Deliverables

- persistent activity rail for current turn
- elapsed time and stage indicators
- background-completion notification support
- optional desktop notifications for:
  - approval needed
  - turn complete
  - verification failed
  - `/plan` complete
- more informative worker-team presentation for `/plan`

### Code touch points

- `internal/tui/view.go`
- `internal/tui/update.go`
- `cmd/polycode/app.go`

### Acceptance criteria

- users can leave a long-running turn and still recover context immediately
- idle waiting is visibly different from active progress

---

## Initiative 9: External Editor Compose And Review Mode

### Why

For long prompts, specs, or review instructions, in-place textarea editing is suboptimal.

### Outcome

Polycode should support a more deliberate compose flow for complex tasks.

### Deliverables

- open-in-editor compose flow using `$EDITOR`
- return edited text to the active input or workflow prompt
- specialized review compose mode for:
  - review instructions
  - implementation plans
  - large `/plan` requests

### Code touch points

- likely new command integration in TUI and app layer
- possible reuse of editor-bridge patterns conceptually, but local editor compose is separate

### Acceptance criteria

- large prompt authoring no longer feels cramped
- editor-based compose does not break chat workflow

---

## Prioritization

### Tier 1: Must-do polish

1. Information architecture refresh
2. Session browser and branching
3. Changes drawer and turn artifact timeline
4. Permission and approval UX overhaul

These four initiatives will change the felt quality of the app the most.

### Tier 2: Strong leverage

5. Command launcher 2.0 and workflow templates
6. Settings, wizard, and MCP redesign
7. Context pressure and compaction UX

### Tier 3: Valuable finishing work

8. Long-running task ergonomics
9. External editor compose and review mode

## Delivery Plan

This should be executed in phases rather than as a single "polish" blob.

---

## Phase 0: Foundation

### Goal

Create the structural primitives needed for the rest of the polish program.

### Work

- add layout mode state
- add reusable status/footer rendering primitives
- define turn-artifact model
- define session metadata extensions
- define richer approval-state model

### Why first

Without these primitives, later features will pile more special cases into `View()` and `Update()`.

### Exit criteria

- layout can support compact mode
- runtime metadata can be rendered outside the transcript
- turn artifacts have a defined in-memory representation

---

## Phase 1: Visibility

### Goal

Make important runtime state visible in the main UI.

### Work

- ship information architecture refresh
- ship compact mode
- ship context-pressure meter
- ship current-session indicator
- improve active shortcut/footer hints

### Exit criteria

- users can understand app state without opening help or using slash commands

---

## Phase 2: Sessions And Artifacts

### Goal

Turn session history into a navigable workspace.

### Work

- session browser
- session preview
- branch/duplicate flow
- changes drawer
- per-turn timeline
- verification badges

### Exit criteria

- history becomes operational, not archival
- "what happened?" and "what changed?" are answered from UI

---

## Phase 3: Safe Action UX

### Goal

Improve trust and control during tool execution.

### Work

- approval modal redesign
- richer approval actions
- command/file previews
- approval timeline logging

### Exit criteria

- approvals are informative enough that users do not have to guess intent

---

## Phase 4: Workflow Acceleration

### Goal

Reduce friction for repeat expert usage.

### Work

- command launcher 2.0
- workflow templates
- favorites and recent commands
- external editor compose

### Exit criteria

- frequent users can operate Polycode faster than raw slash-command recall

---

## Phase 5: Configuration And Finish

### Goal

Make setup and long-running usage feel production-ready.

### Work

- settings redesign
- provider/MCP detail panes
- better wizard messaging
- notifications
- `/plan` progress improvements

### Exit criteria

- onboarding and operations both feel polished rather than merely functional

## Suggested OpenSpec Breakdown

This work should likely be split into focused changes rather than one massive umbrella spec.

Suggested change set:

- `tui-layout-refresh`
- `session-browser`
- `turn-artifacts`
- `approval-ux`
- `command-launcher`
- `settings-redesign`
- `context-visibility`
- `long-running-task-ergonomics`
- `editor-compose`

## Testing Strategy

Polish work often regresses interaction quality even when tests stay green. Testing must include behavior, not just rendering.

### Automated

- TUI update tests for focus transitions
- rendering tests for compact vs comfortable mode
- session metadata persistence tests
- turn-artifact persistence tests
- approval-state transition tests

### Manual

- long multi-turn coding session with compaction
- tool-heavy file editing flow
- MCP-heavy session
- session branch/restore flow
- narrow terminal width
- large terminal width
- approval flow under stress

## Success Metrics

These do not need formal telemetry on day one, but they should inform evaluation.

### Qualitative

- users can explain why consensus landed where it did
- users can inspect turns without reading raw transcript noise
- users can navigate major features without memorizing commands
- settings and session flows feel like product surfaces, not implementation leftovers

### Quantitative proxies

- fewer slash-command-only flows for major actions
- fewer help-overlay roundtrips for common tasks
- lower friction around approvals
- faster recovery when returning to a saved session

## Anti-Goals

To keep the polish program disciplined, avoid these traps.

- do not convert the app into a mouse-first pseudo-GUI
- do not bury the transcript under decorative chrome
- do not add visual complexity without adding decision clarity
- do not copy Crush or OpenCode interactions verbatim where Polycode has different product needs
- do not let polish work obscure the multi-model differentiator

## Recommended Starting Sequence

If only a few items can be started immediately, do them in this order:

1. define turn-artifact model and layout primitives
2. ship compact mode plus stronger runtime status strip
3. ship session browser
4. ship changes drawer and verification badges
5. ship approval modal redesign

That sequence gives Polycode a visibly better shell first, then improves trust, then improves repeat usage.

## Bottom Line

Polycode does not primarily need more features. It needs stronger presentation of the features it already has.

The best polish program is the one that makes these existing strengths obvious:

- consensus is inspectable
- sessions are operable
- tools are supervised
- changes are reviewable
- runtime state is always legible

If executed well, that will give Polycode a sharper identity than simply becoming "another terminal coding agent with nicer colors."
