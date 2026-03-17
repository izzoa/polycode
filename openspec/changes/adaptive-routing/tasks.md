## 1. Operating Modes

- [x] 1.1 Define `Mode` type (ModeQuick, ModeBalanced, ModeThorough) in `internal/routing/mode.go`
- [x] 1.2 Add `DefaultMode string` field to Config (yaml: `mode`, default "balanced")
- [x] 1.3 Add `currentMode Mode` to `conversationState` in app.go
- [x] 1.4 Detect `/mode <name>` in `updateChat()` — validate mode name, send `ModeChangedMsg` to TUI
- [x] 1.5 Add `ModeChangedMsg` handler in TUI update — store mode, update status bar
- [x] 1.6 Display current mode in the status bar (e.g., "polycode [balanced] | claude ...")
- [x] 1.7 In the submit handler, use mode to determine provider selection: quick → [primary], balanced → [primary, best secondary], thorough → all healthy

## 2. Heuristic Router

- [x] 2.1 Create `internal/routing/router.go` with `Router` struct holding cached provider scores
- [x] 2.2 Implement `LoadTelemetryStats(path string) map[string]ProviderStats` — read telemetry JSONL, aggregate per-provider
- [x] 2.3 Implement `ScoreProvider(stats ProviderStats) float64` — scoring formula
- [x] 2.4 Implement `Router.SelectProviders(mode, allHealthy, primaryID) []provider.Provider` — returns subset based on mode and scores
- [x] 2.5 Cache stats in memory, refresh from disk every 5 minutes
- [x] 2.6 Create the router in `startTUI`, pass to the submit handler

## 3. Instruction Hierarchy

- [x] 3.1 Create `internal/memory/instructions.go` with `LoadInstructions(workDir string) string`
- [x] 3.2 Check for `.polycode/instructions.md` in workDir (repo-level)
- [x] 3.3 Check for `~/.config/polycode/instructions.md` (user-level)
- [x] 3.4 Concatenate: repo-level + user-level + built-in default, separated by newlines
- [x] 3.5 Use the loaded instructions as the system prompt in `conversationState` instead of the hardcoded string
- [x] 3.6 Reload instructions when config changes (after settings modifications)

## 4. Repo Memory

- [x] 4.1 Create `internal/memory/memory.go` with `MemoryStore` struct
- [x] 4.2 Implement `LoadMemory(memDir string) map[string]string` — reads all .md files
- [x] 4.3 Implement `SaveMemoryFile(memDir, name, content string) error` — writes a single memory file
- [x] 4.4 Append loaded memory content to the system prompt (after instructions)
- [x] 4.5 Detect `/memory` command in `updateChat()` — display all memory file contents in the chat
- [x] 4.6 Detect `/memory edit <name>` — open editor for named memory file

## 5. Calibration

- [x] 5.1 Add `CalibrationInterval int` to a new `RoutingConfig` in config.go
- [x] 5.2 Track query count on `conversationState`
- [x] 5.3 Every N queries (when in quick/balanced), run a full-consensus query in a background goroutine
- [x] 5.4 Handle `calibration_interval: 0` to disable calibration

## 6. Testing

- [x] 6.1 Unit test: `LoadTelemetryStats` parses JSONL and aggregates correctly
- [x] 6.2 Unit test: `ScoreProvider` returns expected scores for known inputs
- [x] 6.3 Unit test: `SelectProviders` returns correct subsets for each mode
- [x] 6.4 Unit test: `LoadInstructions` loads and concatenates repo + user + default
- [x] 6.5 Unit test: `LoadMemory` reads .md files from directory
- [x] 6.6 Unit test: mode switching via /mode command
