## 1. MCP Client

- [x] 1.1 Create `internal/mcp/client.go` with `MCPClient` struct: manages connections to MCP servers
- [x] 1.2 Implement stdio transport: spawn subprocess, communicate via stdin/stdout JSON-RPC
- [x] 1.3 Implement `DiscoverTools(server)` — send `tools/list` request, parse response into `[]provider.ToolDefinition`
- [x] 1.4 Implement `CallTool(server, toolName, args)` — send `tools/call` request, return result
- [x] 1.5 Add `mcp.servers` config section with `name`, `command`, `args`, and `url` fields
- [x] 1.6 On startup, connect to all configured MCP servers and collect their tools
- [x] 1.7 Merge MCP tools with built-in tools (file_read, file_write, shell_exec) in the tool definitions sent to the primary model
- [x] 1.8 Route MCP tool calls from the model to the correct MCP server in the tool executor

## 2. Hook System

- [x] 2.1 Create `internal/hooks/hooks.go` with `HookManager` struct and `HookEvent` type (PreQuery, PostQuery, PostTool, OnError)
- [x] 2.2 Add `hooks` config section: pre_query, post_query, post_tool, on_error shell command strings
- [x] 2.3 Implement `HookManager.Run(event, context)` — execute the hook shell command with Go text/template variable substitution (.Prompt, .Response, .Error, .ToolName)
- [x] 2.4 Run hooks at appropriate lifecycle points in app.go: pre_query before pipeline run, post_query after completion, post_tool after each tool execution, on_error on failures
- [x] 2.5 Log hook execution to telemetry; do not block the pipeline on hook failure

## 3. Permission Model

- [x] 3.1 Create `internal/permissions/permissions.go` with `PolicyManager` struct
- [x] 3.2 Define `Policy` type: Allow, Ask, Deny
- [x] 3.3 Implement `LoadPolicies()` — read `.polycode/permissions.yaml` (repo) then `~/.config/polycode/permissions.yaml` (user), merge with repo taking precedence
- [x] 3.4 Implement `PolicyManager.Check(toolName string) Policy` — exact match first, then glob pattern matching
- [x] 3.5 Integrate with the tool executor's confirm function: if policy is Allow, auto-approve; if Deny, auto-reject; if Ask, show TUI confirmation
- [x] 3.6 Support glob patterns for MCP tool namespaces (e.g., `mcp_filesystem_*`)

## 4. CI Mode

- [x] 4.1 Add `ci` subcommand to Cobra in main.go with `--pr` flag
- [x] 4.2 Create `cmd/polycode/ci.go` with `runCI()` implementation
- [x] 4.3 Load config from `.polycode/config.yaml` in cwd (repo-level) instead of user config
- [x] 4.4 Support environment variable auth: check `POLYCODE_ANTHROPIC_KEY`, `POLYCODE_OPENAI_KEY`, `POLYCODE_GEMINI_KEY` before keyring
- [x] 4.5 Run headless review of PR diff (reuse `polycode review` logic), post as PR comment
- [x] 4.6 Parse consensus review for critical issues; exit with status 1 if found, 0 if clean
- [x] 4.7 Create a GitHub Actions workflow template at `.github/workflows/polycode-review.yml` in the repo as an example

## 5. Session Sharing

- [x] 5.1 Add `export` subcommand to Cobra: `polycode export [--format md|json] [--output file]`
- [x] 5.2 Implement markdown export: format session as a readable document with prompts, responses, provenance, and tool actions
- [x] 5.3 Implement JSON export: serialize the full `Session` struct
- [x] 5.4 Add `import` subcommand: `polycode import <file>` — load a JSON session file and restore conversation state
- [x] 5.5 Also support `/export` as a slash command in the TUI (exports current session to a file)

## 6. Editor Bridge

- [x] 6.1 Add `serve` subcommand to Cobra: `polycode serve --port 9876`
- [x] 6.2 Create `cmd/polycode/serve.go` with HTTP server setup
- [x] 6.3 Implement `POST /prompt` endpoint: accepts JSON `{prompt}`, runs consensus pipeline, returns `{response}`
- [x] 6.4 Implement `POST /review` endpoint: accepts JSON `{diff}`, runs review pipeline, returns structured review
- [x] 6.5 Implement `GET /status` endpoint: returns provider health, current mode, token usage
- [x] 6.6 Add CORS headers for local editor access
- [x] 6.7 Graceful shutdown on SIGINT/SIGTERM

## 7. Testing

- [x] 7.1 Unit test: MCP tool discovery from a mock stdio server
- [x] 7.2 Unit test: Hook template variable substitution
- [x] 7.3 Unit test: Permission policy loading and matching (exact + glob)
- [x] 7.4 Unit test: Session markdown export format
- [x] 7.5 Unit test: CI exit code logic (critical issues → 1, clean → 0)
- [x] 7.6 Unit test: Editor bridge endpoints return correct responses
