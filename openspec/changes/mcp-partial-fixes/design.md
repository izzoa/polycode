## Context

Polycode's MCP integration has all 18 planned features implemented at the feature level, but 8 items were flagged as PARTIAL by Codex code review. The gaps share three root causes:

1. **No runtime rebuild**: `mcpClient` is created once at startup. Wizard add/edit/delete and `/mcp remove` save config but tell the user to restart. The live client drifts from config.
2. **Partial reconnect**: `reconnectServer()` only re-discovers tools. Resources and prompts become stale after auto-reconnect or manual `/mcp reconnect`.
3. **Spotty observability**: Debug logging only covers `tools/call` and notification wiring — not initialize, discovery, resources, or prompts.

Additionally, the wizard test step calls `mcpClient.Reconnect(name)` which can't test new servers (not yet in `c.servers`), and HTTP transport only does basic POST/JSON without SSE streaming.

## Goals / Non-Goals

**Goals:**
- Config changes take effect immediately without restart (wizard save, `/mcp remove`, `/mcp add`)
- Reconnect refreshes tools, resources, and prompts atomically
- Wizard test validates the staged config, not the saved config
- All MCP JSON-RPC traffic is captured by debug logging when enabled
- HTTP transport can handle SSE (`text/event-stream`) responses

**Non-Goals:**
- Full MCP resource template injection into system prompts (future work)
- Registering MCP prompts as slash commands in the palette (future work)
- Supporting MCP sampling/completion requests from servers (out of scope)

## Decisions

### D1: Runtime rebuild via `MCPClient.Reconfigure(newConfigs)`

**Choice**: Add a `Reconfigure(configs []MCPServerConfig)` method that diffs old vs new configs, connects new servers, disconnects removed ones, and reconnects changed ones — all without replacing the `MCPClient` pointer.

**Alternative considered**: Replace the entire `mcpClient` pointer on config change. Rejected because all closures in `app.go` capture `mcpClient` — replacing it would require an extra layer of indirection (`*atomic.Pointer[*MCPClient]`) and careful coordination across goroutines.

**Rationale**: Mutating the existing client is simpler and the `mu` mutex already serializes access.

### D2: `TestConnection(cfg MCPServerConfig)` spawns a temporary connection

**Choice**: Add a standalone `TestConnection(ctx, cfg)` function (not a method on MCPClient) that creates a temporary `serverConn`, performs the initialize handshake, calls `tools/list`, and tears down. Returns tool count and error.

**Alternative considered**: Add the server to the live client temporarily. Rejected because it mutates shared state and requires cleanup on failure.

**Rationale**: Stateless test function is safe to call from the wizard without affecting the live client.

### D3: `reconnectServer` calls `discoverResources` and `discoverPrompts`

**Choice**: Extend the existing `reconnectServer()` to call `discoverResources()` and `discoverPrompts()` after `discoverTools()`, replacing per-server entries in the `resources` and `prompts` slices.

**Alternative considered**: Store resources/prompts in per-server maps instead of flat slices. Rejected as over-engineering — the flat slice with server-name filtering is consistent with the tools pattern and simple to maintain.

### D4: Centralize logging in `sendRequest()` methods

**Choice**: Add `LogRequest`/`LogResponse` calls inside `serverConn.sendRequest()` (stdio path) and `httpConn.sendRequest()` (HTTP path). The server name is stored on the conn, and method/params are available at the call site.

**Alternative considered**: Instrument at the `MCPClient` level (wrapping each public method). Rejected because it would miss internal calls like `discoverTools` and `connectStdio`'s initialize handshake.

**Rationale**: Transport-layer logging captures everything automatically.

### D5: SSE response handling in HTTP transport

**Choice**: After receiving an HTTP response, check `Content-Type`. If `text/event-stream`, parse SSE frames (`data:` lines) and extract the JSON-RPC response from them. Otherwise, parse as plain JSON (existing behavior).

**Rationale**: The MCP spec allows servers to respond with either plain JSON or SSE streams. Supporting both makes the HTTP transport spec-compliant.

## Risks / Trade-offs

- **[Reconnect races]** → `Reconfigure()` holds `c.mu` during the diff phase and releases it during connection attempts. A concurrent `CallTool` could see a partially-updated state. Mitigation: mark servers as "reconnecting" in `serverErrors` during the transition so callers get a clear error.
- **[SSE complexity]** → SSE parsing adds code to the HTTP path that's hard to test without a real SSE server. Mitigation: unit test with a mock HTTP server returning `text/event-stream` content.
- **[Wizard test latency]** → `TestConnection` blocks the TUI update loop during the test. Mitigation: run in a goroutine (existing pattern from provider test handler).
