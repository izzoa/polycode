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
  action/              → Tool execution, safety guardrails, project context for system prompts
  tui/                 → Bubble Tea TUI (model, update, view, splash, settings, wizard, mcp_wizard)
  mcp/                 → MCP client — server connections, tool/resource/prompt discovery, JSON-RPC transport (stdio + HTTP)
openspec/              → OpenSpec change artifacts (proposals, designs, specs, tasks)
```

## Key Patterns

- **Provider interface**: All LLM adapters implement `provider.Provider` (ID, Query, Authenticate, Validate). Query returns `<-chan StreamChunk` for streaming.
- **Bubble Tea architecture**: TUI uses Elm pattern — Model struct, Update handles messages, View renders. View modes: `viewChat`, `viewSettings`, `viewAddProvider`, `viewEditProvider`.
- **Fan-out pipeline**: `consensus.Pipeline.Run()` dispatches to all providers, collects responses, synthesizes via primary model. Three phases: dispatch → collect → synthesize.
- **Config is the source of truth**: All provider setup flows through `config.Config`. TUI settings and YAML both write to the same config file.
- **Token tracking**: `tokens.TokenTracker` accumulates per-provider usage. `tokens.MetadataStore` fetches model limits from litellm's JSON database with local cache + TTL.
- **MCP client**: `mcp.MCPClient` manages connections to external MCP servers. Tool names are prefixed `mcp_{serverName}_{toolName}` and resolved via a lookup map (`toolIndex`) to avoid underscore-parsing ambiguity. Supports stdio (subprocess) and HTTP transports. A single multiplexed reader goroutine per stdio connection routes responses by request ID and dispatches notifications (e.g. `tools/list_changed`). Config changes apply at runtime via `Reconfigure()` without restart.
- **MCP Registry**: `mcp.RegistryClient` queries `registry.modelcontextprotocol.io/v0/servers` for server discovery. Results are cached in-memory with 15-minute TTL. `ToMCPServerConfig()` maps registry metadata to config (npm→npx, pip/pypi→uvx, oci→docker, remote→URL). The wizard browse step and `/mcp search` both use the registry, falling back to the hardcoded `PopularMCPServers` list when offline.

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
- `ConfigChangedMsg` — triggers registry/pipeline rebuild + MCP reconfigure
- `TestResultMsg` — provider connection test result
- `MCPStatusMsg` — MCP server connection status update
- `MCPTestResultMsg` — MCP server connection test result
- `MCPToolsChangedMsg` — dynamic tool refresh notification
- `MCPCallCountMsg` — MCP tool call count update
- `MCPDashboardDataMsg` — full dashboard data (servers, tools, stats)

## When Modifying

- After adding a new provider type: implement the `Provider` interface, add to `registry.go`'s `newProvider()` switch, add to the wizard's type list in `wizard.go`
- After adding new config fields: update the struct + YAML tags, add validation in `Validate()`, update the setup wizard if applicable
- After changing the TUI: keep view mode dispatch in `View()`, key routing in `Update()` via mode-specific handler functions (`updateChat`, `updateSettings`, `updateWizard`)
- After changing consensus logic: update the integration tests in `consensus/integration_test.go`
- After adding/modifying tools: update `AllTools()` and/or `ReadOnlyTools()` in `tools.go`, add executor dispatch in `executor.go`, update `ToolUsageHints()` in `project_context.go`. Read-only tools go in both sets; mutating tools go in `AllTools()` only and require `e.confirm()`.
- After modifying MCP client: tool metadata changes go in `discoverTools()`. New config fields need: YAML tags in `MCPServerConfig`, validation in `Validate()`, change detection in `mcpConfigChanged()`, and handling in `Reconfigure()`. New MCP methods route through `sendRequest()` (auto-logged when debug enabled). Test with the `newTestClientFull` mock pattern in `client_test.go`. The MCP wizard lives in `tui/mcp_wizard.go`; wizard test uses `TestConnection()` with staged config.

### Tool Sets

**`AllTools()` (primary model — 10 tools):** `file_read`, `file_write`, `file_edit`, `file_delete`, `file_rename`, `shell_exec`, `list_directory`, `grep_search`, `find_files`, `file_info`

**`ReadOnlyTools()` (fan-out providers — 5 tools):** `file_read`, `file_info`, `list_directory`, `grep_search`, `find_files`

**MCP tools (discovered at runtime):** Prefixed `mcp_{server}_{tool}`, resolved via `MCPClient.ResolveToolCall()`. All MCP tools go to the primary model; read-only MCP tools (server `read_only: true` or tool `readOnlyHint` annotation) also go to fan-out. MCP tools route through the confirmation gate unless the server is marked read-only.
