## 1. Data Types + Model State

- [x] 1.1 Add `MCPDashboardServer` struct to `mcp_wizard.go`: Name, Transport, Status, ToolCount, ReadOnly, Error, Tools []string (prefixed names), ResourceCount, PromptCount
- [x] 1.2 Add `MCPDashboardDataMsg` to `mcp_wizard.go`: Servers []MCPDashboardServer, TotalTools int, TotalCalls int64
- [x] 1.3 Add fields to Model: `showMCPDashboard bool`, `mcpDashboardData []MCPDashboardServer`, `mcpDashboardTotalTools int`, `mcpDashboardTotalCalls int64`, `mcpDashboardCursor int`
- [x] 1.4 Add `onMCPDashboardRefresh func()` callback to Model + setter method

## 2. App Wiring — Dashboard Data

- [x] 2.1 Add `sendMCPDashboardData(program, mcpH)` helper in app.go that builds MCPDashboardDataMsg from MCPClient: Status(), Tools(), Resources(), Prompts(), CallCount(), IsServerReadOnly()
- [x] 2.2 Wire `SetMCPDashboardRefreshHandler` in app.go: async call to `sendMCPDashboardData`
- [x] 2.3 Also send dashboard data after reconnect/test results so the dashboard updates live

## 3. Dashboard Toggle

- [x] 3.1 In `updateChat` key handler: `m` key (when textarea empty and not in other overlay) toggles `showMCPDashboard` and fires `onMCPDashboardRefresh`
- [x] 3.2 Add `m` key to help overlay hints
- [x] 3.3 In `View()` dispatch: if `showMCPDashboard`, return `renderMCPDashboard()` (before chat, after help)

## 4. Dashboard Rendering

- [x] 4.1 Create `renderMCPDashboard()` in `view.go`: title, server table, tool lists, stats footer, action hints
- [x] 4.2 Server table: columns NAME, TRANSPORT, STATUS, TOOLS, READ-ONLY with color-coded status (green=connected, red=failed, gray=disconnected)
- [x] 4.3 Cursor: `>` indicator on selected row (j/k to navigate)
- [x] 4.4 Error detail: show error message indented below failed servers
- [x] 4.5 Per-server tool list: grouped under server name, showing prefixed tool names (truncate if >5 tools per server)
- [x] 4.6 Stats footer: "N tools across M servers | K calls this session"
- [x] 4.7 Action hints bar: "j/k:navigate  r:reconnect  t:test  /settings:manage  Esc:close"

## 5. Dashboard Key Handling

- [x] 5.1 Handle `MCPDashboardDataMsg` in Update(): populate dashboard fields
- [x] 5.2 When `showMCPDashboard` is true, route keys to `updateMCPDashboard(msg)`
- [x] 5.3 `updateMCPDashboard`: j/k navigate cursor, Esc/m closes, r reconnects selected, t tests selected
- [x] 5.4 `r` key: call `onReconnectMCP(serverName)` for selected server, show spinner
- [x] 5.5 `t` key: call `onTestMCP(cfg)` for selected server, show spinner

## 6. Tab Bar Integration

- [x] 6.1 Make the MCP indicator in the tab bar selectable when tab bar is focused: navigate past last provider tab to reach MCP
- [x] 6.2 Enter on MCP indicator opens the dashboard

## 7. Documentation

- [x] 7.1 Add `m` key to help overlay items
- [x] 7.2 Update CLAUDE.md: mention MCP dashboard overlay
- [x] 7.3 Update README.md: mention `m` key for MCP dashboard in keyboard shortcuts if documented

## 8. Final Verification

- [x] 8.1 Run `go build ./...` — clean compile
- [x] 8.2 Run `go test ./... -count=1` — all packages pass
- [x] 8.3 Manual test: press `m` → dashboard shows → j/k navigates → Esc closes
- [x] 8.4 Manual test: `r` reconnects, `t` tests, dashboard updates with results
