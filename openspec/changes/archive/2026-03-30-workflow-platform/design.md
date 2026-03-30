## Context

Polycode has a complete internal feature set (Phases 1-4). Phase 5 focuses on external integration surfaces: connecting to external tools (MCP), automating workflows (hooks), controlling access (permissions), running in CI, sharing results, and bridging to editors. These features are individually simple but collectively transform polycode from a standalone tool into a team-ready platform.

## Goals / Non-Goals

**Goals:**
- MCP client that connects to external tool servers and exposes their tools to the primary model
- Lifecycle hooks that run user-defined commands at key events
- Permission policies that control tool approval per-tool, per-repo
- CI mode for automated PR review
- Session export/import for sharing consensus traces
- Minimal HTTP server for editor integration

**Non-Goals:**
- Building polycode as an MCP server (it's a client in v1)
- A full plugin marketplace or registry
- VS Code extension development (just the HTTP bridge API)
- Cloud-hosted collaboration features
- Multi-user authentication or access control

## Decisions

### 1. MCP client: stdio-based server connections

**Choice**: Polycode connects to MCP servers via stdio (spawn a subprocess) or SSE (HTTP). The MCP config specifies servers to connect to:

```yaml
mcp:
  servers:
    - name: filesystem
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir"]
    - name: database
      url: http://localhost:3000/sse
```

On startup, polycode connects to configured MCP servers, discovers their tools via `tools/list`, and adds them to the tool definitions sent to the primary model.

**Rationale**: MCP is the emerging standard for tool interop. Stdio transport is simplest and covers most servers. SSE covers remote servers.

### 2. Hooks: config-defined shell commands

**Choice**: Hooks are defined in config and run as shell commands at lifecycle events:

```yaml
hooks:
  pre_query: "echo 'Query starting' >> /tmp/polycode.log"
  post_query: "notify-send 'Polycode done'"
  post_tool: ""
  on_error: "echo 'Error: {{.Error}}' >> /tmp/polycode-errors.log"
```

Template variables (`.Prompt`, `.Response`, `.Error`, `.ToolName`) are available via Go text/template syntax.

**Rationale**: Shell commands are universally composable. Template variables give hooks useful context. No plugin API needed.

### 3. Permissions: YAML policy file

**Choice**: A `.polycode/permissions.yaml` file (repo-level) or `~/.config/polycode/permissions.yaml` (user-level) defines per-tool approval policies:

```yaml
tools:
  file_read: allow    # never ask for confirmation
  file_write: ask     # always ask (default)
  shell_exec: ask     # always ask
  mcp_filesystem_*: allow  # allow all filesystem MCP tools
  mcp_database_*: deny     # deny all database MCP tools
```

Policies: `allow` (auto-approve), `ask` (confirm in TUI), `deny` (reject silently).

**Rationale**: File-based policies are versionable and shareable. Glob patterns cover MCP tool namespaces.

### 4. CI mode: `polycode ci --pr <number>`

**Choice**: `polycode ci` is a headless mode designed for GitHub Actions:
- Loads config from repo-level `.polycode/config.yaml` (not user config)
- Reviews the PR diff via `gh pr diff`
- Posts the consensus review as a PR comment
- Exits with non-zero status if critical issues are found

This builds on `polycode review --pr --comment` but adds CI-specific behavior (repo config, exit codes, structured output).

**Rationale**: CI mode needs its own subcommand because the config source and output format differ from interactive review.

### 5. Session sharing: export to markdown

**Choice**: `polycode export [--format json|md]` exports the current session as a shareable artifact. The markdown format includes:
- User prompts and consensus responses
- Provider agreement/disagreement summaries
- Tool actions taken
- Token usage

Sessions can be imported: `polycode import session.json` loads a session for replay.

**Rationale**: Markdown is human-readable and pasteable into PRs/Slack. JSON preserves full fidelity for tooling.

### 6. Editor bridge: HTTP server on localhost

**Choice**: `polycode serve --port 9876` starts an HTTP server with endpoints:
- `POST /prompt` — send a prompt, get the consensus response
- `POST /review` — send a diff, get a structured review
- `GET /status` — health check and provider status

Editors send HTTP requests to this server. No WebSocket or streaming in v1 — simple request/response.

**Rationale**: HTTP is universally supported by editors. A VS Code extension can be a simple HTTP client. No custom protocol needed.

## Risks / Trade-offs

- **MCP spec evolution**: The MCP protocol is still maturing. → **Mitigation**: Implement the core (tools/list, tools/call) and skip optional features. Update as the spec stabilizes.

- **Hook security**: Arbitrary shell commands are dangerous. → **Mitigation**: Hooks run with the user's permissions. Document the security implications clearly. No hooks in CI mode unless explicitly enabled.

- **CI mode needs secrets**: CI needs API keys. → **Mitigation**: Support environment variable auth (`POLYCODE_ANTHROPIC_KEY`, etc.) as an alternative to keyring in CI environments.
