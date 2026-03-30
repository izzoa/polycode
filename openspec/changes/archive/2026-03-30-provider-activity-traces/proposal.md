## Why

Polycode currently presents provider tabs as if they show what each model did for a turn, but in practice they only capture the initial fan-out response. The primary model's synthesis, tool execution, follow-up generations, and verification stream only to the consensus panel, so tabs can show a provider as "finished" even though most of that provider's work happened elsewhere.

## What Changes

- **Phase-aware provider traces**: Each provider tab will become a trace of that provider's full participation in a turn instead of a buffer of fan-out text only
- **Primary-provider execution visibility**: The primary provider tab will include synthesis output, tool execution status, tool output, follow-up model output, and verification results in the order they occurred
- **Accurate provider status lifecycle**: Provider tabs will remain active until that provider's full trace is complete instead of flipping to done at the end of fan-out
- **Structured trace rendering**: The TUI will label major phases such as fan-out, synthesis, tool execution, and verification so users can distinguish what happened
- **Trace persistence**: Session save/load and export will persist provider activity traces so reopened sessions retain the same provider-trace data
- **Backward-compatible history**: Existing `Individual` response summaries can remain for compact display, but they will no longer be the only stored representation of provider work

## Capabilities

### New Capabilities
- `provider-activity-tabs`: Provider tabs show the full activity trace for each provider for the current turn, including every runtime phase that provider participated in
- `provider-trace-persistence`: Provider activity traces are saved, restored, and exported alongside the turn history

### Modified Capabilities
_(none — there are no existing top-level specs for this behavior yet)_

## Impact

- **`cmd/polycode/app.go`**: Route fan-out, synthesis, tool-loop, and verification events into provider-specific traces instead of only the consensus panel
- **`internal/tui/update.go`, `model.go`, `view.go`**: Add phase-aware provider trace messages, panel state, and rendering
- **`internal/config/session.go`**: Extend session exchange persistence to store full provider traces
- **`cmd/polycode/sharing*.go`**: Update export/share formatting to include persisted provider traces where appropriate
- **Tests**: TUI update tests, session round-trip tests, and orchestration tests need coverage for multi-phase provider traces
