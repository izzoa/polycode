## 1. Split app.go

- [x] 1.1 Create `cmd/polycode/app_query.go` — move `SetSubmitHandler` closure and `summarizeConversation()` helper
- [x] 1.2 Create `cmd/polycode/app_handlers.go` — move non-query, non-MCP, non-session handler closures (test provider, plan, skill, mode change, undo, redo, yolo, shell context, cancel, clear, save, export, share)
- [x] 1.3 Create `cmd/polycode/app_mcp.go` — move MCP handler closure, `sendMCPStatus()`, `sendMCPDashboardData()`, `wireMCPNotifications()`, MCP test/reconnect/dashboard/registry handlers
- [x] 1.4 Create `cmd/polycode/app_session.go` — move session handler closures, `toSessionMessages()`, `fromSessionMessages()`, auto-name handler, session picker, `/sessions` handler
- [x] 1.5 Verify `app.go` is ≤500 lines containing only initialization and `startTUI()` orchestration
- [x] 1.6 Run `go build ./...` and `go test ./... -count=1` — all pass

## 2. Split update.go

- [x] 2.1 Create `internal/tui/update_chat.go` — move `updateChat()` and chat-mode helpers
- [x] 2.2 Create `internal/tui/update_approval.go` — move approval prompt handling
- [x] 2.3 Create `internal/tui/update_settings.go` — move `updateSettings()`, `updateAddProvider()`, `updateEditProvider()`
- [x] 2.4 Create `internal/tui/update_palette.go` — move command palette and file picker handling
- [x] 2.5 Verify `update.go` is ≤500 lines containing only `Update()` dispatcher and shared message handling
- [x] 2.6 Run `go build ./...` and `go test ./... -count=1` — all pass

## 3. Verification

- [x] 3.1 Run `go test ./... -count=1 -race` — all pass with race detector
- [x] 3.2 Verify no exported API changes: `go doc` output for affected packages is identical
- [ ] 3.3 Manual: launch TUI, test basic chat, settings, MCP, session flows
