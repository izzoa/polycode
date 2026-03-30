## 1. Confirm Channel Rework

- [x] 1.1 Define `ConfirmResult` struct in `action/executor.go` with `Approved bool` and `EditedContent *string`
- [x] 1.2 Change confirm channel type from `chan bool` to `chan ConfirmResult` in executor
- [x] 1.3 Update all executor confirm call sites to read ConfirmResult and apply EditedContent when non-nil
- [x] 1.4 Update TUI-side confirm send to emit ConfirmResult instead of bool

## 2. Edit Mode State

- [x] 2.1 Add `confirmEditing bool` and `confirmTextarea textarea.Model` fields to TUI Model
- [x] 2.2 Add `confirmEditTarget string` field to track which tool field is being edited
- [x] 2.3 Initialize textarea with appropriate defaults (line numbers off, max height 60% viewport)

## 3. Edit Mode Input Handling

- [x] 3.1 In updateChat confirmation key handling, add `e` key to enter edit mode
- [x] 3.2 Populate textarea with current tool content based on tool type (command/content/new_string)
- [x] 3.3 Route key events to textarea when confirmEditing is true
- [x] 3.4 Handle Enter/Ctrl+S: build ConfirmResult with edited content, send to channel, exit edit mode
- [x] 3.5 Handle Esc: exit edit mode, return to normal confirmation view

## 4. Edit Mode Rendering

- [x] 4.1 In confirmation view rendering, show textarea when confirmEditing is true
- [x] 4.2 Show hint text: "Enter to execute, Esc to cancel"
- [x] 4.3 Show [e]dit hint in normal confirmation prompt alongside [y]es/[n]o

## 5. Verification

- [x] 5.1 Update any existing confirm-related test helpers for new ConfirmResult type
- [x] 5.2 Run `go build ./...` and `go test ./...` -- all pass
- [x] 5.3 Manual: test edit flow for shell_exec, file_write, and file_edit tool calls
- [x] 5.4 Manual: verify Esc returns to normal confirm, Enter submits edited version
