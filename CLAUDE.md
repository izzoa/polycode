# CLAUDE.md

## Project Overview

Polycode is a multi-model consensus coding assistant TUI. It queries multiple LLMs in parallel and synthesizes their responses into a single answer via a designated primary model. Built in Go with Bubble Tea.

## Build & Test

```bash
go build ./cmd/polycode/       # Build the binary
go test ./... -count=1         # Run all tests
go build ./...                 # Verify all packages compile
```

No special environment variables needed for building. Tests use mock providers — no API keys required.

## Architecture

```
cmd/polycode/          → CLI entry point (Cobra), app wiring, setup wizard
internal/
  config/              → YAML config loading/validation/saving (~/.config/polycode/config.yaml)
  provider/            → Provider interface + adapters (Anthropic, OpenAI, Gemini, OpenAI-compatible)
  consensus/           → Fan-out dispatcher, consensus engine, pipeline orchestration, truncation
  tokens/              → Token tracking, model limits registry, litellm metadata fetcher
  auth/                → Keyring storage, file fallback, OAuth device flow
  action/              → Tool execution (file_read, file_write, shell_exec), safety guardrails
  tui/                 → Bubble Tea TUI (model, update, view, splash, settings, wizard)
openspec/              → OpenSpec change artifacts (proposals, designs, specs, tasks)
```

## Key Patterns

- **Provider interface**: All LLM adapters implement `provider.Provider` (ID, Query, Authenticate, Validate). Query returns `<-chan StreamChunk` for streaming.
- **Bubble Tea architecture**: TUI uses Elm pattern — Model struct, Update handles messages, View renders. View modes: `viewChat`, `viewSettings`, `viewAddProvider`, `viewEditProvider`.
- **Fan-out pipeline**: `consensus.Pipeline.Run()` dispatches to all providers, collects responses, synthesizes via primary model. Three phases: dispatch → collect → synthesize.
- **Config is the source of truth**: All provider setup flows through `config.Config`. TUI settings and YAML both write to the same config file.
- **Token tracking**: `tokens.TokenTracker` accumulates per-provider usage. `tokens.MetadataStore` fetches model limits from litellm's JSON database with local cache + TTL.

## Code Conventions

- Standard Go project layout (`cmd/`, `internal/`)
- No external HTTP framework — all API calls use `net/http` + `bufio` for SSE parsing
- Provider adapters handle their own streaming SSE parsing (no shared SSE library)
- Errors are wrapped with `fmt.Errorf("context: %w", err)` pattern
- Thread safety via `sync.Mutex` / `sync.RWMutex` where needed (TokenTracker, conversationState)
- Config validation happens in `Config.Validate()` — enforces exactly one primary provider

## TUI Message Types

All pipeline → TUI communication uses typed messages sent via `program.Send()`:
- `QueryStartMsg` / `QueryDoneMsg` — query lifecycle
- `ProviderChunkMsg` — streaming chunks from individual providers
- `ConsensusChunkMsg` — streaming consensus output
- `TokenUpdateMsg` — token usage snapshot after each turn
- `ConfigChangedMsg` — triggers registry/pipeline rebuild
- `TestResultMsg` — provider connection test result

## When Modifying

- After adding a new provider type: implement the `Provider` interface, add to `registry.go`'s `newProvider()` switch, add to the wizard's type list in `wizard.go`
- After adding new config fields: update the struct + YAML tags, add validation in `Validate()`, update the setup wizard if applicable
- After changing the TUI: keep view mode dispatch in `View()`, key routing in `Update()` via mode-specific handler functions (`updateChat`, `updateSettings`, `updateWizard`)
- After changing consensus logic: update the integration tests in `consensus/integration_test.go`
