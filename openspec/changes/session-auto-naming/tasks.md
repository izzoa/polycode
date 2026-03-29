## 1. Session Metadata

- [x] 1.1 Add `Name string` and `UserNamed bool` fields to session metadata struct in `internal/config/session.go`
- [x] 1.2 Ensure Name and UserNamed persist to/from session metadata file (YAML/JSON tags)
- [x] 1.3 Add helper `SetName(name string, userSet bool)` on session metadata

## 2. Auto-Naming Logic

- [x] 2.1 Add `SessionNameMsg{Name string}` message type to TUI
- [x] 2.2 After first QueryDoneMsg (exchange index 0), check if session is unnamed and UserNamed is false
- [x] 2.3 Fire background goroutine: send naming prompt to primary model, parse response, emit SessionNameMsg
- [x] 2.4 In Update handler for SessionNameMsg: sanitize name (truncate 40 chars, strip quotes/punctuation), save to session metadata

## 3. Display Integration

- [x] 3.1 Show session name in status bar (replace or augment timestamp display)
- [x] 3.2 Show session name in session picker list items
- [x] 3.3 Fall back to timestamp display when name is empty

## 4. User Override

- [x] 4.1 Add `/sessions name <text>` slash command parsing in updateChat
- [x] 4.2 Set session Name and UserNamed=true on command execution
- [x] 4.3 Display confirmation (or toast if available)

## 5. Verification

- [x] 5.1 Run `go build ./...` and `go test ./...` -- all pass
- [x] 5.2 Manual: start new session, send first message, verify name appears in status bar
- [x] 5.3 Manual: use `/sessions name` to override, verify auto-naming does not overwrite
- [x] 5.4 Manual: restart app, verify session name persists
