## Context

Polycode's tool execution uses a confirmation gate: before running mutating tools (shell_exec, file_write, file_edit, file_delete, file_rename), the TUI shows the proposed action and waits for the user to press `y` (approve) or `n` (reject). The confirm flow uses a synchronous channel (`chan bool`) between the executor goroutine and the TUI's Update loop. The current channel type carries only a boolean -- no mechanism for modified content.

## Goals / Non-Goals

**Goals:**
- Allow editing the command/content of any confirmable tool call before execution
- Single keypress (`e`) to enter edit mode from the confirmation prompt
- Edited content replaces the original in the executor's tool call
- Works for shell_exec (command string), file_write (full content), and file_edit (old/new strings)
- Esc from edit mode returns to the normal confirm prompt without changes

**Non-Goals:**
- Editing tool call parameters other than the primary content (e.g., cannot change file path)
- Syntax highlighting or validation in the edit textarea
- Undo/redo within the edit textarea (Bubble Tea textarea handles basic editing)
- Editing read-only tool calls (they bypass confirmation entirely)

## Decisions

### 1. Rework confirm channel to carry optional edited content

**Decision**: Change the confirm channel from `chan bool` to `chan ConfirmResult` where `ConfirmResult{Approved bool, EditedContent *string}`. A nil EditedContent means "use original."

**Rationale**: Minimal change to the executor side. The executor checks EditedContent and substitutes it into the tool arguments if present. The channel type change is the only breaking change.

### 2. Textarea component for editing

**Decision**: Use Bubble Tea's `textarea.Model` embedded in the TUI Model. When `e` is pressed during confirmation, populate the textarea with the current tool content and switch to an `editingConfirmation` sub-state.

**Rationale**: The textarea component handles cursor movement, selection, scrolling, and multi-line editing out of the box. No need for a custom editor.

### 3. Edit applies to the primary content field only

**Decision**: For shell_exec, edit the `command` field. For file_write, edit the `content` field. For file_edit, edit the `new_string` field.

**Rationale**: These are the fields the user most often wants to tweak. Editing other fields (path, old_string) would require a multi-field form, which is out of scope.

## Risks / Trade-offs

- **[Risk] Large file content overflows textarea** -> Mitigation: Textarea scrolls; set max height to 60% of viewport.
- **[Risk] User accidentally submits empty content** -> Mitigation: Warn if content is empty; require explicit confirmation.
- **[Risk] ConfirmResult channel change breaks tests** -> Mitigation: Update mock confirm functions in test helpers.
