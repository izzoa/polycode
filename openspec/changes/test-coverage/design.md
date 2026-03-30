## Context

Current test coverage by package (lines of test code / lines of production code):

| Package | Test LoC | Prod LoC | Ratio | Notes |
|---------|----------|----------|-------|-------|
| `internal/notify` | 0 | 36 | 0% | Only untested package |
| `internal/tui` | ~807 | ~9,200 | ~9% | Covers model init, some view rendering |
| `internal/provider` | ~100 | ~1,300 | ~8% | Basic adapter tests |
| `cmd/polycode` | ~200 | ~3,500 | ~6% | CLI flag tests |

All 18 test suites pass including with `-race`. The issue is breadth, not reliability.

## Goals / Non-Goals

**Goals:**
- Bring every package to at least one test file
- Cover error paths in provider adapters (malformed responses, network errors)
- Cover key TUI update flows (message dispatch, mode transitions)
- Test CI mode's non-interactive pipeline

**Non-Goals:**
- 100% line coverage (diminishing returns)
- Visual/screenshot TUI testing
- Integration tests requiring live API keys
- Refactoring production code for testability (that's god-file-refactoring scope)

## Decisions

### 1. notify tests: mock exec.Command

**Decision**: Test `Notify()` by verifying it constructs the correct OS command (osascript on macOS, notify-send on Linux) without actually executing it. Use a test helper that replaces the command runner.

**Rationale**: Desktop notifications are platform-specific and can't be meaningfully asserted in CI. Testing command construction is sufficient.

### 2. TUI tests: use Bubble Tea's test helpers

**Decision**: Test update handlers by constructing a Model, sending messages via `Update()`, and asserting state changes on the returned model. Focus on message routing and state transitions, not rendered output.

**Rationale**: Bubble Tea's Elm architecture makes handler testing straightforward — send a message, check the model. View rendering tests are fragile and low-value.

### 3. Provider tests: table-driven error scenarios

**Decision**: For each provider adapter, add table-driven tests with malformed SSE payloads, HTTP error codes, and timeout scenarios using `httptest.Server`.

**Rationale**: Provider adapters do their own SSE parsing. Error paths (truncated events, non-200 responses, connection drops) are where bugs hide.

## Risks / Trade-offs

- **[Risk] TUI tests become brittle** — Mitigation: Test state transitions, not rendered strings.
- **[Risk] Provider error paths vary by upstream API** — Mitigation: Use real-world error payloads captured from actual API failures where possible.
