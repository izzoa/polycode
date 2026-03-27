## Why

A pre-release audit identified one critical race condition, missing unit tests for 9 exported functions, and a config namespace gap. The race condition on the `mcpClient` variable in `app.go` can corrupt state when the config-change handler writes concurrently with query goroutines reading. This must be fixed before the MCP feature ships.

## What Changes

- **Critical: mcpClient race fix** — Protect the `mcpClient` pointer in `app.go` with a `sync.RWMutex` so reads from query goroutines and writes from the config-change handler don't race.
- **Unit tests for untested exports** — Add direct tests for: `Reconnect()`, `Status()`, `Reconfigure()`, `Close()`, `TestConnection()`, `CallCount()`, `ReadOnlyToolDefinitions()`, `ReadResource()`, `GetPrompt()`.
- **Cross-namespace config warning** — Add a warning in `Config.Validate()` when a provider and MCP server share the same name.
- **Remove orphaned code** — Remove `ReadResource()` and `GetPrompt()` since they have zero call sites and no UI trigger. They can be re-added when interactive resource reading and prompt execution are implemented.

## Capabilities

### New Capabilities
- `mcp-thread-safety`: Synchronized mcpClient access in app.go
- `mcp-export-tests`: Unit tests for all remaining untested exported MCP functions
- `mcp-namespace-validation`: Cross-namespace name collision warning in Config.Validate()
- `mcp-code-cleanup`: Remove orphaned ReadResource/GetPrompt methods

### Modified Capabilities

## Impact

- `cmd/polycode/app.go` — mcpClient access wrapped with RWMutex
- `internal/mcp/client.go` — Remove ReadResource, GetPrompt; minor test helpers
- `internal/mcp/client_test.go` — New tests for Reconnect, Status, Reconfigure, Close, TestConnection, CallCount, ReadOnlyToolDefinitions
- `internal/config/config.go` — Cross-namespace warning in Validate()
- `internal/config/config_test.go` — Test for cross-namespace warning
