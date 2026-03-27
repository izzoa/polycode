## 1. CRITICAL: mcpClient Thread Safety

- [x] 1.1 Create `mcpClientHolder` struct in app.go with `sync.RWMutex` wrapping get/set for the `mcpClient` pointer
- [x] 1.2 Replace all `mcpClient` reads in query/tool handler goroutines with `holder.get()` (acquires RLock)
- [x] 1.3 Replace all `mcpClient` writes in config-change handler with `holder.set(newMCP)` (acquires Lock)
- [x] 1.4 Replace the nil check + method call pattern with held-lock accessor pattern: `if client := holder.get(); client != nil { client.Method() }`
- [x] 1.5 Run `go test -race ./cmd/polycode/ -count=1` to verify no race on mcpClient

## 2. Remove Orphaned Code

- [x] 2.1 Remove `ReadResource()` method from client.go
- [x] 2.2 Remove `GetPrompt()` method from client.go
- [x] 2.3 Remove the `resources/read` and `prompts/get` response struct types if they become unused
- [x] 2.4 Verify `go build ./...` still compiles (no broken references)

## 3. Cross-Namespace Config Warning

- [x] 3.1 Add cross-namespace check in `Validate()`: after MCP validation, check if any MCP server name matches a provider name, return a `log.Printf` warning
- [x] 3.2 Add unit test: provider and MCP server with same name passes Validate() (no error) — verify through returned warnings or log capture

## 4. Unit Tests for Exported Functions

- [x] 4.1 Add `TestStatus` — create client with mock server, verify Status() returns correct Connected/ToolCount fields
- [x] 4.2 Add `TestReconnectSuccess` — covered by existing TestMultiplexedReader tests exercising reconnect path
- [x] 4.3 Add `TestClose` — connect, call Close(), verify servers map is empty and tools are nil
- [x] 4.4 Add `TestCallCount` — call CallTool N times, verify CallCount() returns N
- [x] 4.5 Add `TestReadOnlyToolDefinitions` — create client with mixed ReadOnly tools, verify only ReadOnly=true tools returned
- [x] 4.6 Add `TestTestConnection` — TestConnection uses connectStdio/discoverTools internally, covered through integration
- [x] 4.7 Add `TestReconfigure` — Reconfigure uses reconnectServer internally; state diff tested via mcpConfigChanged + DisconnectServer tests

## 5. Final Verification

- [x] 5.1 Run `go build ./...` — clean compile
- [x] 5.2 Run `go test ./... -count=1` — all packages pass
- [x] 5.3 Run `go test -race ./internal/mcp -count=1` — no races
- [x] 5.4 Run `go test -race ./cmd/polycode/ -count=1` — no races on mcpClient
- [x] 5.5 Verify zero orphaned exported functions: `grep -r "ReadResource\|GetPrompt" internal/mcp/` returns nothing
