## 1. Config Validation

- [x] 1.1 Add MCP server validation to `Config.Validate()` in `config.go`: name required, no duplicates, command or URL required, non-negative timeout
- [x] 1.2 Add unit tests for MCP validation: empty name, duplicate names, no command/URL, negative timeout, valid config passes

## 2. Tool Name Collision Detection

- [x] 2.1 Add collision detection in `Connect()` when building toolIndex: if prefixed name exists from a different server, log warning and skip the duplicate
- [x] 2.2 Add same collision detection in `reconnectServer()` when rebuilding toolIndex
- [x] 2.3 Add unit test: two servers producing same prefixed name, verify first wins and second is skipped

## 3. Extend Mock Server for Resources/Prompts

- [x] 3.1 Extend the mock server in `client_test.go` to handle `resources/list` and `prompts/list` methods with configurable response data
- [x] 3.2 Extend `newTestClient` to accept optional resources and prompts data

## 4. Unit Tests — Core MCP

- [x] 4.1 Add `TestReconfigure` — test adding, removing, and changing servers via Reconfigure() with mock servers
- [x] 4.2 Add `TestMcpConfigChanged` — test helper detects changes in Command, Args, URL, Env, ReadOnly, Timeout
- [x] 4.3 Add `TestDiscoverResources` — test resource discovery against mock server
- [x] 4.4 Add `TestDiscoverPrompts` — test prompt discovery against mock server with arguments

## 5. Unit Tests — HTTP Transport + SSE

- [x] 5.1 Create `internal/mcp/http_transport_test.go`
- [x] 5.2 Add `TestParseSSEResponse_SingleEvent` — single data line, blank separator
- [x] 5.3 Add `TestParseSSEResponse_MultiLineData` — multiple data lines joined with newline
- [x] 5.4 Add `TestParseSSEResponse_CommentsAndPrefixes` — event:, id:, : comment lines skipped
- [x] 5.5 Add `TestParseSSEResponse_NoTrailingBlankLine` — final event without blank separator
- [x] 5.6 Add `TestParseSSEResponse_ErrorResponse` — SSE stream with JSON-RPC error
- [x] 5.7 Add `TestHTTPTransport_JSONResponse` — mock HTTP server returning application/json
- [x] 5.8 Add `TestHTTPTransport_SSEResponse` — mock HTTP server returning text/event-stream

## 6. Unit Tests — Multiplexed Reader

- [x] 6.1 Add `TestMultiplexedReader_ConcurrentRequests` — send two requests concurrently, verify each gets correct response by ID
- [x] 6.2 Add `TestMultiplexedReader_NotificationDispatch` — verify notifications are delivered to onNotify callback

## 7. CLAUDE.md Updates

- [x] 7.1 Add `internal/mcp/` to Architecture section
- [x] 7.2 Add MCP key patterns: MCPClient, tool naming, transports, multiplexed reader
- [x] 7.3 Add "When Modifying" MCP guidance: discoverTools changes, Reconfigure field additions, new MCP methods, testing patterns

## 8. Final Verification

- [x] 8.1 Run `go test ./... -count=1` — all packages pass
- [x] 8.2 Run `go test -race ./internal/mcp -count=1` — no races
- [x] 8.3 Run `go build ./...` — clean compile
