# MCP Enhancements Plan

Comprehensive plan for enhancing Polycode's MCP integration across safety, resilience, UI/UX, and protocol support.

---

## Phase 1: Safety & Correctness (Priority: Critical)

### 1.1 — Confirmation Gate for MCP Tool Calls

**Problem:** MCP tools bypass the `confirmFunc` gate entirely. The `ExternalToolHandler` in `app.go:1095` dispatches directly to `mcpClient.CallTool()` without checking permissions or prompting the user. An MCP server can expose arbitrary side-effecting tools (database writes, HTTP requests, file mutations) that execute silently.

**Design:**

Route MCP tool calls through the same permission → yolo → user-prompt flow that built-in mutating tools use.

1. **Extend `MCPServerConfig`** with a `ReadOnly bool` field (`yaml:"read_only,omitempty"`). Servers marked `read_only: true` skip confirmation. This is the server-level override — individual tool annotations (1.1b) refine further.

2. **Integrate with `permissions.PolicyManager`** — MCP tool names are already prefixed `mcp_{server}_{tool}`, so existing glob support works out of the box:
   ```yaml
   # permissions.yaml
   tools:
     mcp_filesystem_*: allow      # trust all filesystem server tools
     mcp_database_write: deny     # block specific tool
     mcp_*: ask                   # prompt for everything else
   ```

3. **Modify the external handler** in `app.go` to call `confirmFunc(call.Name, description)` before dispatching MCP tools. Build a human-readable description from the tool name + truncated arguments. Only skip confirmation if:
   - The server is marked `read_only: true`, OR
   - The permission policy returns `PolicyAllow`, OR
   - Yolo mode is enabled

**Files to modify:**
- `internal/config/config.go` — add `ReadOnly` field to `MCPServerConfig`
- `cmd/polycode/app.go` — wrap MCP dispatch in `confirmFunc` call
- No changes needed to `permissions.go` — glob matching already works

**Tests:**
- Unit test: MCP tool with `PolicyAsk` triggers confirmation callback
- Unit test: MCP tool with `PolicyAllow` bypasses confirmation
- Unit test: `read_only: true` server bypasses confirmation
- Unit test: `PolicyDeny` blocks MCP tool execution

---

### 1.2 — Fix Server Name Parsing

**Problem:** The prefix parser in `app.go:1097-1105` splits `mcp_{rest}` on the first `_` to extract `serverName` and `toolName`. A server named `my_db` would parse as server=`my`, tool=`db_actual_tool` — completely wrong.

**Design:**

Replace runtime string parsing with a lookup map built at discovery time.

1. **Add a method to `MCPClient`:**
   ```go
   // ResolveToolCall takes a prefixed tool name (e.g. "mcp_filesystem_read_file")
   // and returns the (serverName, toolName) pair.
   func (c *MCPClient) ResolveToolCall(prefixedName string) (serverName, toolName string, ok bool)
   ```

2. **Build an internal map** during `Connect()` / `discoverTools()`:
   ```go
   toolIndex map[string]struct{ Server, Tool string }  // prefixed name → (server, tool)
   ```
   Populated in `discoverTools()` alongside `c.tools`.

3. **Use a non-ambiguous separator** for the prefixed name. Two options:
   - **Option A (breaking):** Switch to `mcp::{server}::{tool}` — requires all providers to handle `:` in tool names (most do).
   - **Option B (non-breaking):** Keep `mcp_{server}_{tool}` format but resolve via the lookup map instead of parsing. Server names with underscores work because the map key is the exact prefixed string.

   **Recommendation:** Option B. The lookup map is robust regardless of naming and avoids any provider compatibility concerns.

4. **Update external handler** in `app.go` to use `mcpClient.ResolveToolCall(call.Name)` instead of manual parsing.

**Files to modify:**
- `internal/mcp/client.go` — add `toolIndex` map, `ResolveToolCall()` method, populate in `discoverTools()`
- `cmd/polycode/app.go` — replace manual parsing with `ResolveToolCall()`

**Tests:**
- Unit test: server name `my_db` with tool `read` resolves correctly
- Unit test: unknown prefixed name returns `ok=false`
- Unit test: `ToToolDefinitions()` and `ResolveToolCall()` are consistent

---

## Phase 2: TUI Visibility & MCP Wizard (Priority: High)

### 2.1 — MCP Status in Settings View

**Problem:** MCP is completely invisible in the TUI. Users have no way to see connected servers, their status, or what tools were discovered.

**Design:**

Add an "MCP Servers" section below the existing provider table in `renderSettings()`.

**Layout:**
```
Settings — Provider Management

  NAME          TYPE              MODEL                  AUTH           PRIMARY
> claude        anthropic         claude-sonnet-4        configured     ★
  gpt           openai            gpt-4o                 configured

MCP Servers
  NAME          TRANSPORT    STATUS        TOOLS
  filesystem    stdio        connected     3
  github        stdio        failed        —

a:add  e:edit  d:delete  t:test  m:mcp  Esc:back
```

**Implementation:**

1. **Track MCP server status** — Add a new struct to the TUI model:
   ```go
   type mcpServerStatus struct {
       Name      string
       Transport string   // "stdio" or "sse"
       Status    string   // "connected", "failed", "disconnected"
       ToolCount int
       Error     string   // populated on failure
   }
   ```

2. **Add `MCPStatusMsg`** TUI message type — sent from `app.go` after MCP connect attempt:
   ```go
   type MCPStatusMsg struct {
       Servers []mcpServerStatus
   }
   ```

3. **Render in `renderSettings()`** — below the provider table, render MCP server rows with color-coded status (green=connected, red=failed, gray=disconnected).

4. **Add `m` key binding** in settings to enter the MCP wizard (Phase 2.3).

**Files to modify:**
- `internal/tui/model.go` — add `mcpServers []mcpServerStatus` field
- `internal/tui/update.go` — handle `MCPStatusMsg`
- `internal/tui/settings.go` — render MCP section
- `cmd/polycode/app.go` — send `MCPStatusMsg` after connect

---

### 2.2 — `/mcp` Slash Command

**Design:**

Register `/mcp` as a slash command with subcommands, following the `/skill` pattern.

**Subcommands:**
- `/mcp` or `/mcp list` — list servers and their tools
- `/mcp status` — show connection status for all servers
- `/mcp reconnect [name]` — reconnect a specific server (or all if no name)
- `/mcp tools [server]` — list tools from a specific server (or all)
- `/mcp add` — open the MCP wizard (add mode)
- `/mcp remove <name>` — remove a server from config

**Output format for `/mcp list`:**
```
MCP Servers (2 connected)

  filesystem (stdio) — connected, 3 tools
    • mcp_filesystem_read_file — Read a file from the filesystem
    • mcp_filesystem_write_file — Write content to a file
    • mcp_filesystem_list_dir — List directory contents

  github (stdio) — connected, 5 tools
    • mcp_github_search — Search repositories
    ...
```

**Implementation:**

1. **Register `/mcp` command** in `NewModel()` slash command list.
2. **Add `onMCP` callback** to TUI model (same pattern as `onSkill`).
3. **Wire handler in `app.go`** — dispatch subcommands, send results as `ConsensusChunkMsg`.
4. **For `reconnect`:** Add `Reconnect(serverName string)` method to `MCPClient` that closes and re-establishes a single server connection.
5. **For `add`/`remove`:** Delegate to MCP wizard (2.3) or config mutation + save.

**Files to modify:**
- `internal/tui/model.go` — add `onMCP` callback, register slash command
- `internal/tui/update.go` — dispatch `/mcp` command
- `internal/mcp/client.go` — add `Reconnect()`, `ServerStatus()` methods
- `cmd/polycode/app.go` — wire `/mcp` handler

---

### 2.3 — MCP Wizard (Full TUI Flow)

**Problem:** Adding MCP servers currently requires hand-editing `~/.config/polycode/config.yaml`. There's no guided setup, no validation, and no way to browse/install popular servers.

**Design:**

A full wizard flow mirroring the provider wizard pattern — new view modes, step-based navigation, list selection, text input, connection testing, and a curated registry of popular MCP servers.

#### 2.3.1 — View Modes

Add two new view modes:
```go
const (
    viewChat         viewMode = iota // 0
    viewSettings                     // 1
    viewAddProvider                  // 2
    viewEditProvider                 // 3
    viewAddMCP                       // 4  (new)
    viewEditMCP                      // 5  (new)
)
```

#### 2.3.2 — Wizard Steps

```
mcpStepSource    (0)  → Select source: "Popular servers" / "Custom server"
mcpStepBrowse    (1)  → [CONDITIONAL: source=popular] Browse curated registry
mcpStepTransport (2)  → Select transport: "stdio (subprocess)" / "SSE (HTTP)"
mcpStepName      (3)  → Enter server name (auto-suggested from selection)
mcpStepCommand   (4)  → [CONDITIONAL: transport=stdio] Enter command (e.g. "npx")
mcpStepArgs      (5)  → [CONDITIONAL: transport=stdio] Enter arguments (e.g. "-y @modelcontextprotocol/server-filesystem /path")
mcpStepURL       (6)  → [CONDITIONAL: transport=sse] Enter server URL
mcpStepEnv       (7)  → [OPTIONAL] Add environment variables (key=value pairs)
mcpStepReadOnly  (8)  → Mark as read-only? (yes/no)
mcpStepTest      (9)  → Test connection (auto-triggered, shows spinner + result)
mcpStepConfirm   (10) → Review summary, confirm save
```

**Conditional logic (`mcpShouldShowStep`):**
- `mcpStepBrowse`: only if source = "Popular servers"
- `mcpStepCommand`, `mcpStepArgs`: only if transport = stdio
- `mcpStepURL`: only if transport = SSE
- `mcpStepEnv`: always shown, but can be skipped (enter to skip)
- `mcpStepTest`: always shown, auto-triggers connection test

#### 2.3.3 — Curated Server Registry

Embed a list of well-known MCP servers with pre-filled configs:

```go
type MCPServerTemplate struct {
    Name        string
    Description string
    Command     string
    Args        []string
    EnvVars     []string   // required env var names (user fills values)
    ReadOnly    bool
    Category    string     // "filesystem", "search", "database", "dev-tools", "ai"
}

var PopularMCPServers = []MCPServerTemplate{
    {
        Name:        "filesystem",
        Description: "Read/write local files and directories",
        Command:     "npx",
        Args:        []string{"-y", "@modelcontextprotocol/server-filesystem", "{PATH}"},
        ReadOnly:    false,
        Category:    "filesystem",
    },
    {
        Name:        "github",
        Description: "GitHub API — repos, issues, PRs, search",
        Command:     "npx",
        Args:        []string{"-y", "@modelcontextprotocol/server-github"},
        EnvVars:     []string{"GITHUB_TOKEN"},
        ReadOnly:    false,
        Category:    "dev-tools",
    },
    {
        Name:        "postgres",
        Description: "Query PostgreSQL databases (read-only)",
        Command:     "npx",
        Args:        []string{"-y", "@modelcontextprotocol/server-postgres", "{CONNECTION_STRING}"},
        ReadOnly:    true,
        Category:    "database",
    },
    {
        Name:        "brave-search",
        Description: "Web search via Brave Search API",
        Command:     "npx",
        Args:        []string{"-y", "@modelcontextprotocol/server-brave-search"},
        EnvVars:     []string{"BRAVE_API_KEY"},
        ReadOnly:    true,
        Category:    "search",
    },
    {
        Name:        "memory",
        Description: "Persistent knowledge graph memory",
        Command:     "npx",
        Args:        []string{"-y", "@modelcontextprotocol/server-memory"},
        ReadOnly:    false,
        Category:    "ai",
    },
    {
        Name:        "puppeteer",
        Description: "Browser automation and web scraping",
        Command:     "npx",
        Args:        []string{"-y", "@modelcontextprotocol/server-puppeteer"},
        ReadOnly:    false,
        Category:    "dev-tools",
    },
    {
        Name:        "sqlite",
        Description: "Query SQLite databases",
        Command:     "npx",
        Args:        []string{"-y", "@modelcontextprotocol/server-sqlite", "{DB_PATH}"},
        ReadOnly:    true,
        Category:    "database",
    },
    {
        Name:        "slack",
        Description: "Slack workspace integration",
        Command:     "npx",
        Args:        []string{"-y", "@modelcontextprotocol/server-slack"},
        EnvVars:     []string{"SLACK_BOT_TOKEN", "SLACK_TEAM_ID"},
        ReadOnly:    false,
        Category:    "dev-tools",
    },
    // More can be added over time
}
```

#### 2.3.4 — Browse Step UI

When user selects "Popular servers", show a categorized, scrollable list:

```
MCP Wizard — Browse Servers

  FILESYSTEM
  > filesystem         Read/write local files and directories

  SEARCH
    brave-search       Web search via Brave Search API

  DATABASE
    postgres           Query PostgreSQL databases (read-only)
    sqlite             Query SQLite databases

  DEV TOOLS
    github             GitHub API — repos, issues, PRs, search
    puppeteer          Browser automation and web scraping
    slack              Slack workspace integration

  AI
    memory             Persistent knowledge graph memory

  j/k:navigate  Enter:select  Esc:cancel
```

Navigation: `j`/`k` or arrow keys to move cursor, `Enter` to select. Selecting a template pre-fills all subsequent wizard steps (name, command, args, read-only) — user can still edit each field.

**Placeholder resolution:** Templates with `{PATH}`, `{CONNECTION_STRING}`, `{DB_PATH}` placeholders prompt the user to fill them during the args step. The wizard detects `{...}` tokens in args and presents a focused input for each.

#### 2.3.5 — Environment Variable Step

For servers requiring env vars (e.g. `GITHUB_TOKEN`):

```
MCP Wizard — Environment Variables

  This server requires the following environment variables:

  GITHUB_TOKEN: ••••••••••••••••  (configured ✓)

  Add additional env vars? (Enter key=value, or Enter to skip)
  >

  Enter:next  Esc:cancel
```

- Required vars from the template shown first with input fields (password-masked for tokens/keys)
- User can add additional vars
- Env vars stored in `MCPServerConfig.Env map[string]string`
- Values containing secrets stored in keyring via `auth.Store()` (same pattern as provider API keys), with the config storing a reference like `$KEYRING:mcp_github_GITHUB_TOKEN`

#### 2.3.6 — Connection Test Step

Auto-triggered when reaching the test step (same pattern as provider wizard API key test):

```
MCP Wizard — Connection Test

  Testing connection to "github"...  ⠋

  [after success:]
  ✓ Connected successfully
    Server: github-mcp-server v1.2.0
    Tools discovered: 5
      • search_repositories
      • get_issue
      • create_issue
      • list_pull_requests
      • get_file_contents

  Enter:next  Esc:skip
```

**Implementation:**
1. Spawn the server subprocess with provided command/args/env
2. Perform `initialize` handshake
3. Call `tools/list` to discover tools
4. Display results (server info from initialize response, tool list)
5. Close the test connection (the real connection happens at app startup)
6. Send `MCPWizardTestResultMsg` to TUI with results

If the test fails, show the error and allow the user to go back and fix command/args/env, or skip the test and save anyway.

#### 2.3.7 — Confirmation Step

```
MCP Wizard — Review

  Name:       github
  Transport:  stdio
  Command:    npx -y @modelcontextprotocol/server-github
  Env:        GITHUB_TOKEN (configured)
  Read-only:  no
  Test:       ✓ passed (5 tools)

  Enter:save  Esc:cancel
```

#### 2.3.8 — Edit Mode

Entering the wizard in edit mode (`viewEditMCP`) pre-populates all fields from the existing `MCPServerConfig`. Starts at `mcpStepTransport` (skipping source/browse since the server already exists).

#### 2.3.9 — Delete Flow

From the settings view, when the MCP section cursor is active:
- `d` key triggers delete confirmation: `Remove MCP server 'github'? (y/n)`
- On `y`: remove from `cfg.MCP.Servers`, save config, disconnect server, send `ConfigChangedMsg`

#### 2.3.10 — Model State

New fields on TUI `Model`:

```go
// MCP Wizard state
mcpWizardStep        mcpWizardStep
mcpWizardData        config.MCPServerConfig
mcpWizardEnv         map[string]string         // env vars being configured
mcpWizardInput       textinput.Model
mcpWizardListCursor  int
mcpWizardListItems   []string
mcpWizardEditing     bool
mcpWizardEditIndex   int
mcpWizardSource      string                    // "popular" or "custom"
mcpWizardTemplate    *MCPServerTemplate        // selected template (nil if custom)
mcpWizardTestResult  *mcpTestResult            // nil until test completes
mcpWizardTesting     bool
mcpWizardPlaceholders []mcpPlaceholder         // {PATH} etc. to fill

// MCP settings cursor (separate from provider cursor)
mcpSettingsCursor    int
mcpSettingsFocused   bool                      // true = cursor in MCP section
```

#### 2.3.11 — Settings View Navigation

The settings view gains a two-section layout. Tab or a separator key (e.g. `Tab`) switches focus between the provider table and the MCP table. Each section has its own cursor.

```
  Providers (Tab to switch)
  NAME          TYPE              MODEL                  AUTH           PRIMARY
> claude        anthropic         claude-sonnet-4        configured     ★
  gpt           openai            gpt-4o                 configured

  MCP Servers
  NAME          TRANSPORT    STATUS        TOOLS
  filesystem    stdio        connected     3
  github        stdio        connected     5

  a:add  e:edit  d:delete  t:test  Tab:switch section  Esc:back
```

When MCP section is focused:
- `a` → enter MCP wizard (add mode)
- `e` → enter MCP wizard (edit mode) for selected server
- `d` → delete selected server
- `t` → test connection to selected server (reconnect + discover tools)

**Files to create:**
- `internal/tui/mcp_wizard.go` — wizard step rendering, input handling, save logic
- `internal/mcp/registry.go` — `PopularMCPServers` template list

**Files to modify:**
- `internal/tui/model.go` — add MCP wizard state fields, new view modes
- `internal/tui/update.go` — route MCP wizard messages, dispatch key handling
- `internal/tui/view.go` — add `viewAddMCP`/`viewEditMCP` to view dispatch
- `internal/tui/settings.go` — two-section layout, MCP table rendering
- `internal/config/config.go` — add `Env` and `ReadOnly` fields to `MCPServerConfig`
- `cmd/polycode/app.go` — wire MCP wizard callbacks (test, save, reconnect)

---

### 2.4 — Status Bar MCP Indicator

**Design:**

Add an MCP connection indicator to the tab bar / status area in chat view.

```
[consensus] [claude] [gpt-4o]                    MCP: 2/2 ✓  mode: balanced
```

Or if a server is down:
```
[consensus] [claude] [gpt-4o]                    MCP: 1/2 ⚠  mode: balanced
```

**Implementation:**
- Track `mcpConnectedCount` and `mcpTotalCount` on the TUI model
- Update via `MCPStatusMsg`
- Render in the tab bar area of `renderChat()`

---

## Phase 3: Connection Resilience (Priority: Medium)

### 3.1 — Per-Server Failure Isolation

**Problem:** `MCPClient.Connect()` aborts and tears down all servers if any single server fails to connect. A broken `postgres` server prevents `filesystem` from working.

**Design:**

1. Change `Connect()` to continue past individual server failures
2. Track per-server status:
   ```go
   type ServerStatus struct {
       Connected bool
       Error     error
       ToolCount int
   }

   func (c *MCPClient) Status() map[string]ServerStatus
   ```
3. Log warnings for failed servers but continue with the rest
4. Return a multi-error if any servers failed (callers can check but don't have to abort)

---

### 3.2 — Automatic Reconnection

**Problem:** If an MCP server process crashes mid-session, `CallTool` fails with a broken pipe and never recovers.

**Design:**

1. **Detect dead connections** — before sending a request in `sendRequest()`, check if the process has exited:
   ```go
   func (sc *serverConn) isAlive() bool {
       if sc.process == nil {
           return false
       }
       // Check if process has exited (non-blocking)
       return sc.process.ProcessState == nil
   }
   ```

2. **Auto-reconnect in `CallTool()`:**
   ```go
   func (c *MCPClient) CallTool(ctx, serverName, toolName, args) (string, error) {
       conn := c.servers[serverName]
       if !conn.isAlive() {
           // Attempt reconnect
           newConn, err := c.connectStdio(ctx, conn.config)
           if err != nil {
               return "", fmt.Errorf("server %q died and reconnect failed: %w", serverName, err)
           }
           // Re-discover tools
           tools, _ := c.discoverTools(ctx, serverName, newConn)
           c.replaceServer(serverName, newConn, tools)
           conn = newConn
       }
       // proceed with call...
   }
   ```

3. **Notify TUI** — send `MCPReconnectMsg` so the status indicator updates.

---

### 3.3 — Per-Call Timeout

**Problem:** `sendRequest()` blocks forever reading from the scanner. A hung server freezes the entire synthesis loop.

**Design:**

1. Accept a `context.Context` in `sendRequest()` (currently unused)
2. Read in a goroutine with the scanner, select on `ctx.Done()`
3. Default per-call timeout of 30 seconds, configurable via `MCPServerConfig.Timeout`

```go
func (sc *serverConn) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
    // ... send request ...

    type result struct {
        data json.RawMessage
        err  error
    }
    ch := make(chan result, 1)

    go func() {
        // existing scanner loop
        data, err := sc.readResponse(id)
        ch <- result{data, err}
    }()

    select {
    case r := <-ch:
        return r.data, r.err
    case <-ctx.Done():
        return nil, fmt.Errorf("request timed out: %w", ctx.Err())
    }
}
```

**Config addition:**
```go
type MCPServerConfig struct {
    // ... existing fields ...
    Timeout int `yaml:"timeout,omitempty"` // per-call timeout in seconds (default 30)
}
```

---

### 3.4 — Graceful Shutdown

**Problem:** `Close()` kills server processes abruptly. MCP spec defines a shutdown sequence.

**Design:**

Send a `shutdown` notification before killing:
```go
func (sc *serverConn) gracefulClose(timeout time.Duration) error {
    // 1. Send shutdown notification
    sc.sendNotification("notifications/cancelled", nil)

    // 2. Close stdin (signals EOF to well-behaved servers)
    sc.stdin.Close()

    // 3. Wait for process to exit with timeout
    done := make(chan error, 1)
    go func() { done <- sc.process.Wait() }()

    select {
    case err := <-done:
        return err
    case <-time.After(timeout):
        // Force kill after timeout
        sc.process.Process.Kill()
        return sc.process.Wait()
    }
}
```

---

## Phase 4: Protocol Completeness (Priority: Medium)

### 4.1 — Environment Variable Passthrough

**Problem:** MCP servers often need API keys or config via environment variables, but there's no way to configure this.

**Design:**

1. **Extend `MCPServerConfig`:**
   ```go
   type MCPServerConfig struct {
       // ... existing fields ...
       Env      map[string]string `yaml:"env,omitempty"`
       ReadOnly bool              `yaml:"read_only,omitempty"`
   }
   ```

2. **Apply env vars when spawning** in `connectStdio()`:
   ```go
   cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
   cmd.Env = os.Environ()  // inherit parent env
   for k, v := range cfg.Env {
       // Support keyring references: $KEYRING:key_name
       if strings.HasPrefix(v, "$KEYRING:") {
           v, _ = auth.Retrieve(v[9:])
       }
       cmd.Env = append(cmd.Env, k+"="+v)
   }
   ```

3. **Secure storage** — the MCP wizard (2.3) stores sensitive values in keyring and writes `$KEYRING:mcp_{server}_{var}` references in the YAML. Plain values also supported for non-sensitive config.

---

### 4.2 — SSE / Streamable HTTP Transport

**Problem:** SSE transport is stubbed out (`if cfg.URL != "" { continue }`).

**Design:**

Implement the MCP streamable HTTP transport (the spec's recommended remote transport).

1. **New connection type** in `serverConn`:
   ```go
   type serverConn struct {
       // ... existing fields ...
       transport transportType  // "stdio" or "http"
       httpURL   string
       httpClient *http.Client
       sessionID  string        // returned by server in Mcp-Session-Id header
   }
   ```

2. **HTTP transport flow:**
   - `POST` to server URL with JSON-RPC request body
   - Content-Type: `application/json`
   - Server responds with JSON-RPC response (or SSE stream for streaming)
   - Session management via `Mcp-Session-Id` header

3. **`connectHTTP()`:**
   - Send `initialize` as POST
   - Parse response, extract session ID
   - Send `notifications/initialized` as POST

4. **`sendRequestHTTP()`:**
   - POST JSON-RPC request to URL
   - Include `Mcp-Session-Id` header
   - Parse JSON-RPC response from body

5. **Abstract transport** — `sendRequest` dispatches to `sendRequestStdio` or `sendRequestHTTP` based on `transport` field.

**Files to create:**
- `internal/mcp/http_transport.go` — HTTP transport implementation

**Files to modify:**
- `internal/mcp/client.go` — transport abstraction, connect dispatch

---

### 4.3 — Read-Only Tool Annotation

**Problem:** MCP tools are excluded from fan-out because they might have side effects. But many MCP tools are genuinely read-only, and the MCP spec supports a `readOnlyHint` annotation on tool definitions.

**Design:**

1. **Parse `annotations` from `tools/list` response:**
   ```go
   type MCPTool struct {
       // ... existing fields ...
       ReadOnly bool  // derived from annotations.readOnlyHint
   }
   ```

2. **Extend `ToToolDefinitions()`** to tag tools with read-only status (add a field to `provider.ToolDefinition` or return a separate list).

3. **Include read-only MCP tools in fan-out:**
   ```go
   fanOutTools := action.ReadOnlyTools()
   if mcpClient != nil {
       fanOutTools = append(fanOutTools, mcpClient.ReadOnlyToolDefinitions()...)
   }
   ```

4. **Safety fallback:** If the server doesn't provide annotations, treat all its tools as side-effecting (current behavior). The `read_only` server-level flag from 1.1 also applies as an override.

---

### 4.4 — Dynamic Tool Refresh (`notifications/tools/list_changed`)

**Problem:** Tools are discovered once at connect time. If a server's tools change, Polycode won't notice.

**Design:**

1. **Background notification listener** — after initialization, spawn a goroutine that reads from `stdout` looking for notifications (lines with no matching request ID).

2. **Handle `notifications/tools/list_changed`:**
   ```go
   func (sc *serverConn) listenNotifications(ctx context.Context, callback func(method string, params json.RawMessage)) {
       // Runs in background goroutine
       // Reads lines from scanner
       // Filters for notifications (no "id" field)
       // Calls callback with method and params
   }
   ```

3. **Re-discover tools** when notification received:
   - Call `tools/list` again
   - Update `c.tools` and `toolIndex`
   - Send `MCPToolsChangedMsg` to TUI to update status display

4. **Separate request/notification streams** — this requires decoupling the scanner from the request/response flow. Move to a multiplexed reader:
   - Background goroutine reads all lines, dispatches to either the pending-request channel (by ID) or the notification callback.
   - `sendRequest` sends and waits on a per-ID channel instead of reading directly.

This is a meaningful refactor of the I/O model but necessary for proper MCP compliance.

---

## Phase 5: Resource & Prompt Support (Priority: Low)

### 5.1 — MCP Resources

**Problem:** MCP servers can expose resources (files, data, schemas) that could enrich the conversation context.

**Design:**

1. **Discover resources** via `resources/list` after connecting
2. **Expose via `/mcp resources [server]`** command
3. **Auto-inject relevant resources** into the system prompt when the server provides `resourceTemplates`
4. **Read resources** via `resources/read` when the model requests context

This is a significant feature and should be designed as a follow-up RFC once the core MCP improvements land.

---

### 5.2 — MCP Prompts

**Problem:** MCP servers can expose pre-built prompt templates.

**Design:**

1. **Discover prompts** via `prompts/list` after connecting
2. **Register as slash commands** — each prompt becomes `/mcp:{server}:{prompt}` in the command palette
3. **Execute prompts** — call `prompts/get` with user-provided arguments, inject result messages into conversation

Lower priority — useful but not essential for the core MCP experience.

---

## Phase 6: Observability & Developer Experience (Priority: Low)

### 6.1 — MCP Request Logging

**Design:**

Add optional debug logging for MCP JSON-RPC traffic, useful for debugging server issues.

1. **Config flag:** `MCPConfig.Debug bool` or per-server `MCPServerConfig.Debug bool`
2. **Log to file:** `~/.config/polycode/mcp-debug.log` (not TUI — too noisy)
3. **Content:** timestamp, server name, direction (→/←), method, truncated params/result

### 6.2 — MCP Tool Usage in Token Display

**Design:**

Track MCP tool call counts alongside token usage. Show in the token display:

```
Tokens: claude 1.2k↑ 3.4k↓ | gpt-4o 1.1k↑ 2.8k↓ | MCP: 3 calls
```

---

## Implementation Order

| Order | Item | Scope | Depends On | Status |
|-------|------|-------|------------|--------|
| 1 | 1.2 Fix server name parsing | `mcp/client.go`, `app.go` | — | ✅ Done |
| 2 | 1.1 Confirmation gate | `config.go`, `app.go` | 1.2 | ✅ Done |
| 3 | 4.1 Env var passthrough | `config.go`, `mcp/client.go` | — | ✅ Done |
| 4 | 3.1 Per-server failure isolation | `mcp/client.go` | — | ✅ Done |
| 5 | 3.3 Per-call timeout | `mcp/client.go` | — | ✅ Done |
| 6 | 2.1 Settings view MCP status | `tui/settings.go`, `tui/model.go` | 3.1 | ✅ Done |
| 7 | 2.4 Status bar indicator | `tui/view.go` | 2.1 | ✅ Done |
| 8 | 2.3 MCP Wizard | `tui/mcp_wizard.go` | 2.1, 4.1 | ✅ Done |
| 9 | 2.2 `/mcp` slash command | `tui/model.go`, `app.go` | 2.1 | ✅ Done |
| 10 | 3.2 Auto-reconnect | `mcp/client.go` | 3.1, 3.3 | ✅ Done |
| 11 | 3.4 Graceful shutdown | `mcp/client.go` | — | ✅ Done |
| 12 | 4.3 Read-only tool annotation | `mcp/client.go`, `app.go` | 1.1 | ✅ Done |
| 13 | 4.2 SSE/HTTP transport | `mcp/http_transport.go` | 3.3 | ✅ Done |
| 14 | 4.4 Dynamic tool refresh | `mcp/client.go` | 4.2 refactor | ✅ Done |
| 15 | 5.1 Resources | `mcp/client.go`, `app.go` | 4.4 | ✅ Done |
| 16 | 5.2 Prompts | `mcp/client.go`, `app.go` | 4.4 | ✅ Done |
| 17 | 6.1 Request logging | `mcp/client.go`, `debug_log.go` | — | ✅ Done |
| 18 | 6.2 Tool usage display | `tui/view.go`, `app.go` | — | ✅ Done |
