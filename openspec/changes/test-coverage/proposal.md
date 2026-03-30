## Why

The project has 32 test files across 17 tested packages, all passing with the race detector. However, several areas have significant coverage gaps: `internal/notify` has zero tests (the only untested package), the TUI has 807 lines of tests for 9,200+ lines of production code, and provider adapters have ~100 lines of tests for ~1,300 lines of code. As the project grows, these gaps increase the risk of regressions in error paths, edge cases, and platform-specific behavior.

## What Changes

- Add tests for `internal/notify` (desktop notification dispatch)
- Add TUI update handler tests covering chat, settings, and approval flows
- Add provider adapter error path tests (malformed SSE, auth failures, timeout handling)
- Add CI mode integration tests

## Capabilities

### New Capabilities
<!-- None — test-only changes -->

### Modified Capabilities
<!-- No production code changes -->

## Impact

- **Files created** (~4): `internal/notify/notify_test.go`, `internal/tui/update_chat_test.go`, `internal/provider/anthropic_test.go` (or expand existing), `cmd/polycode/ci_test.go`
- **Files modified** (0): No production code changes
- **Dependencies**: None
- **Config schema**: No changes
- **Scope**: ~500-800 lines of test code
