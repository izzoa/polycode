## Why

Polycode currently queries every configured provider on every prompt — blind fan-out regardless of task type, cost, or historical performance. A user asking a simple syntax question pays the same token cost as a complex architecture review. Meanwhile, there's no way to persist repo-specific knowledge (build commands, project conventions, file layout) across sessions — users repeat the same instructions every time. Phase 4 makes multi-model usage affordable through operating modes and heuristic routing, and cumulative through repo memory and an instruction hierarchy.

## What Changes

- **Operating modes**: Three modes the user can switch between — `quick` (primary only, no consensus), `balanced` (primary + one secondary), `thorough` (all providers, full consensus + verifier)
- **Heuristic router**: Uses telemetry data to pick the best provider subset for a given task type (debug, review, implement, explain) based on historical latency, quality, and error rate
- **Repo memory**: Persistent memory at `~/.config/polycode/memory/` storing build commands, test commands, architecture notes, preferred patterns, and per-provider performance signals
- **Instruction hierarchy**: Three-tier precedence for system instructions — repo-level (`.polycode/instructions.md`) > user-level (`~/.config/polycode/instructions.md`) > built-in default
- **`/memory` command**: View and edit repo memory from within the TUI
- **`/mode` command**: Switch between quick/balanced/thorough modes mid-session
- **Periodic full-consensus fallback**: Even in `quick` mode, occasionally run full consensus to recalibrate routing heuristics

## Capabilities

### New Capabilities
- `operating-modes`: Quick/balanced/thorough mode switching with different provider selection strategies
- `repo-memory`: Persistent repo-specific knowledge across sessions with /memory command
- `instruction-hierarchy`: Three-tier instruction precedence (repo > user > default)
- `heuristic-router`: Provider selection based on telemetry history

### Modified Capabilities
_(none — the consensus pipeline and fan-out system are reused; routing just changes which providers participate)_

## Impact

- **New `internal/routing/`**: Router, mode logic, telemetry aggregation
- **New `internal/memory/`**: Repo memory store, instruction loader
- **`internal/consensus/pipeline.go`**: Pipeline accepts a provider subset from the router instead of all healthy providers
- **`internal/tui/`**: Mode indicator in status bar, /memory and /mode command handling
- **`cmd/polycode/app.go`**: Instruction hierarchy loading, router integration, mode state
- **`internal/config/`**: New `mode` field, memory config
