## Why

Sessions are currently identified by timestamp or sequential number, making it difficult to distinguish them in the session picker. After a few sessions, the list becomes a wall of "Session 2026-03-28 14:32" entries with no indication of topic. Users must open each session to remember what it was about. Auto-generating a short descriptive name after the first exchange gives sessions meaningful identifiers with zero user effort.

## What Changes

- After the first query-response exchange completes, send a lightweight request to the primary model asking for a 3-5 word topic summary
- Display the generated name in the status bar and session picker
- User can override the auto-name with `/sessions name <custom name>`
- Skip auto-naming if the session already has a user-assigned name
- Session names persist in the session metadata file

## Capabilities

### New Capabilities
- `auto-session-naming`: Automatic session name generation from first exchange content

### Modified Capabilities
<!-- No existing spec-level changes -->

## Impact

- **Files modified** (3): `cmd/polycode/app.go`, `internal/tui/model.go`, `internal/config/session.go`
- **Files created**: None
- **Dependencies**: None new
- **Scope**: ~100-150 lines
