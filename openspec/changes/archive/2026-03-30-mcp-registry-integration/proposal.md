## Why

The MCP wizard currently uses a hardcoded list of 8 popular servers (`PopularMCPServers` in `mcp_wizard.go`). The official MCP Registry at `registry.modelcontextprotocol.io` launched in 2025 as a community-driven, open catalog with a free REST API — hundreds of servers with structured metadata including npm/pip package identifiers, required env vars, transport types, and descriptions. Integrating it replaces stale hardcoded data with live, searchable, continuously-growing server discovery.

## What Changes

- **Registry client** (`internal/mcp/registry.go`) — HTTP client for the MCP Registry REST API with search, pagination, and in-memory cache with TTL. Parses the server JSON schema into Go structs.
- **TUI wizard browse step** — Replace hardcoded `PopularMCPServers` with live registry fetch. Show search input, paginated results grouped by category. Fall back to hardcoded list if registry is unreachable.
- **`/mcp search <query>` slash command** — Search the registry from the TUI and display results in the chat area.
- **`polycode mcp search <query>` CLI command** — Search the registry from the command line, display name/description/package/transport.
- **`polycode mcp browse` CLI command** — Interactive browser: search, pick a server, auto-populate `mcp add` with the selected server's config.

## Capabilities

### New Capabilities
- `mcp-registry-client`: HTTP client for registry.modelcontextprotocol.io REST API
- `mcp-registry-browse`: TUI wizard pulls from live registry instead of hardcoded list
- `mcp-registry-search`: Search registry via /mcp search and polycode mcp search CLI
- `mcp-registry-install`: Auto-populate server config from registry metadata during add

### Modified Capabilities

## Impact

- `internal/mcp/registry.go` — new file: registry client, types, cache
- `internal/mcp/registry_test.go` — new file: tests with mock HTTP server
- `internal/tui/mcp_wizard.go` — wizard browse step fetches from registry
- `internal/tui/update.go` — handle `/mcp search` command
- `cmd/polycode/main.go` — register `mcp search` and `mcp browse` subcommands
- `cmd/polycode/mcp.go` — implement search/browse handlers
- `CLAUDE.md` — document registry integration
- `README.md` — document search/browse commands
