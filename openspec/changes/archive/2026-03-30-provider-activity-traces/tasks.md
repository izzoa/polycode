## 1. Provider Trace Message Model

- [x] 1.1 Add a phase-aware provider trace message type and phase enum in `internal/tui/update.go`
- [x] 1.2 Update `ProviderPanel` state in `internal/tui/model.go` to accumulate phase-ordered trace content and track per-provider completion accurately
- [x] 1.3 Update provider panel rendering in `internal/tui/view.go` to show phase labels and remove the misleading fan-out-only placeholder behavior

## 2. App Orchestration

- [x] 2.1 Route fan-out callbacks in `cmd/polycode/app.go` into `fanout` provider trace events for all queried providers
- [x] 2.2 Mirror primary-provider synthesis chunks into the primary tab as `synthesis` trace events while leaving consensus streaming unchanged
- [x] 2.3 Mirror tool-loop status, tool output, and follow-up model output into the primary tab as `tool` trace events
- [x] 2.4 Mirror verification messages into the primary tab as `verify` trace events and only mark the primary provider done after its final phase completes

## 3. Session Persistence And Export

- [x] 3.1 Extend `internal/config/session.go` with backward-compatible provider trace records on each `SessionExchange`
- [x] 3.2 Save provider trace data during auto-save in `cmd/polycode/app.go` and restore that data on session load paths
- [x] 3.3 Update export formatting in `cmd/polycode/sharing*.go` to prefer provider traces and fall back to legacy `Individual` summaries

## 4. Test Coverage

- [x] 4.1 Add or update TUI tests for phase-aware accumulation, phase headers, and provider completion timing
- [x] 4.2 Add session round-trip tests for provider trace persistence and legacy-session fallback
- [x] 4.3 Add orchestration or export tests covering primary synthesis, tool, and verification trace output
- [x] 4.4 Verify `go test ./... -count=1` passes
