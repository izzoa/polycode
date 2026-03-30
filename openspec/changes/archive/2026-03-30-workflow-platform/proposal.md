## Why

Polycode now has a strong core: multi-model consensus, tool execution, agent teams, adaptive routing, and repo memory. But it operates in isolation — no way to connect to external tool servers, no lifecycle hooks for custom workflows, no CI integration, and no way to share review artifacts with teammates. Phase 5 adds the ecosystem plumbing required for team adoption: MCP support, hooks, permissions, CI mode, session sharing, and a minimal editor bridge.

## What Changes

- **MCP client support**: Connect to external MCP (Model Context Protocol) servers to extend polycode's tool capabilities with third-party tools (databases, APIs, custom functions)
- **Hook system**: User-defined shell commands that run at lifecycle events (pre-query, post-query, post-tool, on-error) for custom integrations
- **Permission model**: Per-tool approval policies (always-allow, always-ask, deny) scoped by repo or user, replacing the current blanket confirm-everything approach
- **`polycode ci` mode**: Headless CI agent that reviews PRs and posts structured consensus findings — builds on `polycode review` but designed for automation
- **Session sharing**: Export consensus traces as shareable markdown or JSON artifacts so teammates can see what models said, where they agreed/disagreed, and what actions were taken
- **Editor bridge**: A minimal HTTP server that accepts file selections and prompts from VS Code or other editors and forwards them to polycode
- **Replayable review artifacts**: Save structured review output with provenance for comparison across runs and model ensembles

## Capabilities

### New Capabilities
- `mcp-client`: Connect to and use tools from external MCP servers
- `hook-system`: Lifecycle hooks for custom shell commands at key events
- `permission-model`: Per-tool approval policies with repo/user scoping
- `ci-mode`: Headless CI agent for automated PR review
- `session-sharing`: Export/import consensus traces as shareable artifacts
- `editor-bridge`: HTTP server for editor integration

### Modified Capabilities
_(none — all existing features continue working; this adds integration surfaces)_

## Impact

- **New `internal/mcp/`**: MCP client, tool discovery, server connection management
- **New `internal/hooks/`**: Hook definitions, lifecycle event dispatch, config
- **New `internal/permissions/`**: Permission policies, approval logic
- **`cmd/polycode/main.go`**: New `ci` subcommand, `serve` subcommand for editor bridge
- **`cmd/polycode/ci.go`**: CI mode implementation
- **`cmd/polycode/serve.go`**: HTTP server for editor bridge
- **`internal/config/`**: Hook definitions, permission policies, MCP server config
- **`internal/config/session.go`**: Export/import methods for shareable artifacts
