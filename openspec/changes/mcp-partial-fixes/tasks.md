## 1. Full Reconnect (resources + prompts)

- [x] 1.1 Extend `reconnectServer()` in `client.go` to call `discoverResources()` and `discoverPrompts()` after `discoverTools()`, replacing per-server entries in the `resources` and `prompts` slices
- [x] 1.2 Extend `DisconnectServer()` to remove resources and prompts for the disconnected server alongside tools and index entries
- [x] 1.3 Add unit test: verify reconnect refreshes resources and prompts

## 2. Centralized Debug Logging

- [x] 2.1 Add `serverName` field to `serverConn` struct (set during connect) so transport-layer logging knows which server it's logging for
- [x] 2.2 Move `LogRequest`/`LogResponse` calls into `serverConn.sendRequest()` (stdio path) — log method and truncated params/result for every request
- [x] 2.3 Add `LogRequest`/`LogResponse` calls into `httpConn.sendRequest()` (HTTP path) with server name
- [x] 2.4 Remove the per-callsite `c.debug.LogRequest`/`LogResponse` calls from `CallTool()` (now redundant)
- [x] 2.5 Verify: enable `mcp.debug: true`, connect a server, confirm initialize + tools/list + tools/call all appear in mcp-debug.log

## 3. Staged Config Test

- [x] 3.1 Add standalone `TestConnection(ctx context.Context, cfg MCPServerConfig) (int, error)` function in `client.go` that creates a temp connection, performs initialize handshake, calls tools/list, returns tool count, and tears down
- [x] 3.2 Change `onTestMCP` callback signature in `model.go` from `func(serverName string)` to `func(cfg config.MCPServerConfig)`
- [x] 3.3 Update `mcpStepTest` handler in `mcp_wizard.go` to pass `m.mcpWizardData` (the staged config) instead of just the server name
- [x] 3.4 Update `SetTestMCPHandler` in `app.go` to call `mcp.TestConnection(ctx, cfg)` instead of `mcpClient.Reconnect(name)`
- [x] 3.5 Make the test step auto-trigger on entry: fire the test callback in `nextMCPWizardStep()` when advancing to `mcpStepTest`, returning a spinner tick command

## 4. Runtime Reconfiguration

- [x] 4.1 Add `Reconfigure(configs []MCPServerConfig)` method to `MCPClient` that diffs old vs new configs: connect new, disconnect removed, reconnect changed
- [x] 4.2 Update `c.configs` inside `Reconfigure` so `Status()` and `Reconnect()` use the new config set
- [x] 4.3 Wire `Reconfigure` into the config-change handler in `app.go`: when `ConfigChangedMsg` fires and `cfg.MCP.Servers` has changed, call `mcpClient.Reconfigure(cfg.MCP.Servers)` and send `MCPStatusMsg`
- [x] 4.4 Update `saveMCPWizard()` in `mcp_wizard.go` to remove the "restart to connect" message and rely on the config-change handler
- [x] 4.5 Update `/mcp remove` in `app.go` to call `mcpClient.Reconfigure(cfg.MCP.Servers)` after removing from config, instead of just `DisconnectServer`
- [x] 4.6 Add unit test: verify `Reconfigure` with added/removed/changed servers produces correct `Status()` output

## 5. SSE Streaming

- [x] 5.1 Add `parseSSEResponse(body io.Reader) (json.RawMessage, error)` helper in `http_transport.go` that reads `data:` lines from an SSE stream and extracts the JSON-RPC response
- [x] 5.2 Update `httpConn.sendRequest()` to check response `Content-Type`: if `text/event-stream`, use `parseSSEResponse`; otherwise parse as plain JSON (existing path)
- [x] 5.3 Add unit test: mock HTTP server returning `text/event-stream` with a JSON-RPC result in `data:` frames, verify correct response extraction

## 6. Integration + Verification

- [x] 6.1 Run `go test ./... -count=1` — all 17 packages must pass
- [x] 6.2 Run `go test -race ./internal/mcp -count=1` — no race conditions
- [x] 6.3 Run `go build ./...` — clean compile
- [x] 6.4 Update MCP_ENHANCEMENTS.md implementation tracker: mark all 8 PARTIAL items as Done
