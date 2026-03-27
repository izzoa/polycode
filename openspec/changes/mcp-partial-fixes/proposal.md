## Why

The first three phases of MCP enhancements landed core functionality (safety, TUI visibility, resilience, protocol support), but Codex code review identified 8 items as PARTIAL — working at the feature level but with correctness gaps under runtime mutation, reconnection, and observability paths. These gaps stem from three shared root causes: no live mcpClient rebuild on config changes, reconnect only refreshing tools (not resources/prompts), and debug logging not covering the full transport surface.

## What Changes

- **Runtime MCP rebuild**: Add `RebuildMCP()` to `MCPClient` that re-reads configs, connects new servers, disconnects removed ones, and refreshes all metadata. Wire into the config-change handler so wizard add/edit/delete and `/mcp remove` take effect immediately without restart.
- **Wizard test with staged config**: Replace the wizard test step's `Reconnect(name)` call with a new `TestConnection(cfg MCPServerConfig)` method that spawns a temporary connection from the in-progress config, validates it, and tears it down.
- **Full reconnect refresh**: Extend `reconnectServer()` to re-discover resources and prompts alongside tools, and update `DisconnectServer()` to clean up resource/prompt state.
- **Centralized debug logging**: Move request/response logging into the `sendRequest()` methods (both stdio and HTTP) so all MCP traffic — initialize, tools/list, resources/list, prompts/list, tools/call, resources/read, prompts/get — is captured automatically.
- **SSE streaming support**: Extend `httpConn` to detect and handle `text/event-stream` responses, parsing SSE frames and delivering JSON-RPC results through them.

## Capabilities

### New Capabilities
- `mcp-runtime-rebuild`: Live MCPClient rebuild when config changes at runtime (wizard save, /mcp remove)
- `mcp-staged-test`: Wizard connection test using staged (unsaved) MCPServerConfig
- `mcp-full-reconnect`: Reconnect refreshes tools, resources, and prompts together
- `mcp-centralized-logging`: Debug logging covers all JSON-RPC traffic at transport layer
- `mcp-sse-streaming`: HTTP transport supports SSE response parsing

### Modified Capabilities

## Impact

- `internal/mcp/client.go` — `RebuildMCP()`, `TestConnection()`, reconnect resource/prompt refresh, `DisconnectServer()` cleanup
- `internal/mcp/http_transport.go` — SSE response parsing
- `internal/mcp/debug_log.go` — No changes (already supports the interface); logging calls move into transport
- `internal/tui/mcp_wizard.go` — Test step callback passes `MCPServerConfig` instead of server name
- `cmd/polycode/app.go` — Config-change handler rebuilds mcpClient, wizard test uses `TestConnection`
