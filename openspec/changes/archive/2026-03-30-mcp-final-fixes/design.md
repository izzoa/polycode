## Context

The MCP feature is functionally complete with 24 unit tests passing and race detector clean on the MCP package. However, the `mcpClient` variable in `app.go` is shared across goroutines without synchronization — the config-change handler can write `mcpClient = newMCP` while query handlers concurrently read it. Additionally, 9 exported functions in the MCP package lack direct tests, and `ReadResource()`/`GetPrompt()` have zero call sites.

## Goals / Non-Goals

**Goals:**
- Eliminate the mcpClient race condition
- Every surviving exported MCP function has at least one direct unit test
- Config warns about confusing same-name provider + MCP server
- No orphaned exported code in the MCP package

**Non-Goals:**
- Adding interactive resource reading or prompt execution UI (future work)
- Rewriting app.go's closure-based architecture
- 100% branch coverage

## Decisions

### D1: sync.RWMutex wrapper for mcpClient

**Choice**: Create a small `mcpClientHolder` struct with `sync.RWMutex` that wraps get/set access to the `mcpClient` pointer. All reads use `RLock`, the config-change writer uses `Lock`.

**Alternative considered**: `atomic.Pointer[*mcp.MCPClient]` — simpler but doesn't protect multi-field operations like "check nil then call method". The RWMutex lets us hold the read lock across the nil check + method call.

**Alternative considered**: Channel-based serialization — too invasive for the closure architecture.

### D2: Remove ReadResource/GetPrompt rather than test them

**Choice**: Remove these methods since they have zero call sites and no UI wiring. They can be re-added from git history when interactive resource/prompt features are built.

**Rationale**: Testing orphaned code wastes effort and gives false confidence. The discovery methods (`discoverResources`, `discoverPrompts`) and list commands (`/mcp resources`, `/mcp prompts`) remain — only the invocation methods are removed.

### D3: Cross-namespace warning (not error)

**Choice**: Log a warning during `Validate()` when a provider and MCP server share the same name. Don't return an error — it's not functionally broken, just confusing.

**Rationale**: Hard errors would break configs that currently work. A warning in the validation output is sufficient.

## Risks / Trade-offs

- **[RWMutex contention]** → Reads vastly outnumber writes (writes only on config change). RLock has near-zero contention. No performance risk.
- **[Removing ReadResource/GetPrompt]** → If someone built against these methods externally... they're in `internal/`, so no external consumers possible.
