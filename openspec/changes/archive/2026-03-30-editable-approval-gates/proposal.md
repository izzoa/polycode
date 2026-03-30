## Why

When the primary model proposes a tool call (shell command, file write, file edit), the user currently faces a binary choice: approve or reject. If the command is almost right but needs a small tweak (different flag, path correction, content fix), the user must reject and hope the model retries correctly -- or manually run the corrected version themselves. This creates friction in the most security-sensitive part of the workflow.

## What Changes

- During the tool confirmation prompt, pressing `e` enters edit mode
- The confirmation view swaps to a textarea pre-filled with the tool's command string (shell_exec) or proposed content (file_write/file_edit)
- User edits the content, then presses Enter to execute the modified version or Esc to cancel
- The edited content is sent to the executor in place of the original
- The synchronous confirm channel is reworked to carry an optional edited payload alongside the approve/reject boolean

## Capabilities

### New Capabilities
- `editable-confirmation`: Edit-before-execute capability for tool confirmation prompts

### Modified Capabilities
<!-- No existing spec-level changes beyond the confirmation UX -->

## Impact

- **Files modified** (4): `internal/tui/update.go`, `internal/tui/view.go`, `internal/tui/model.go`, `internal/action/executor.go`
- **Files created**: None
- **Dependencies**: None new (textarea already available via Bubble Tea bubbletea/textarea)
- **Scope**: ~200-300 lines
