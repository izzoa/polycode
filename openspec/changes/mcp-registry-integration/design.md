## Context

The official MCP Registry (`registry.modelcontextprotocol.io`) provides a free, unauthenticated REST API for discovering MCP servers. The `/v0/servers` endpoint returns structured JSON with server metadata including:

- `name` — unique identifier (reverse-domain, e.g., `ai.autoblocks/ctxl-mcp`)
- `description` — human-readable purpose
- `packages[]` — installable packages with `registryType` (npm/pip/oci), `identifier`, `transport.type` (stdio/streamable-http), and `environmentVariables[]`
- `remotes[]` — hosted HTTP endpoints with URL and auth headers
- `repository` — GitHub source URL

Query params: `search` (text search), `limit` (max 100), `cursor` (pagination), `updated_since`, `version`.

Currently, `PopularMCPServers` in `mcp_wizard.go` is a hardcoded slice of 8 servers that will become stale over time.

## Goals / Non-Goals

**Goals:**
- Live server discovery from the official MCP Registry
- Search from both TUI (`/mcp search`) and CLI (`polycode mcp search`)
- Wizard browse step uses live registry with fallback to hardcoded list
- Selected registry server auto-populates add wizard fields (name, command/URL, args, env vars)
- Cache results to avoid hitting the API on every wizard open

**Non-Goals:**
- Publishing servers to the registry (read-only integration)
- Auto-installing npm/pip packages (user runs `npx` themselves)
- Replacing the custom server path in the wizard (registry is one source option alongside manual entry)

## Decisions

### D1: Registry client as `internal/mcp/registry.go`

**Choice**: Standalone Go file with `RegistryClient` struct holding an `http.Client`, base URL, and in-memory cache. No new dependencies — uses `net/http` + `encoding/json`.

**Rationale**: Follows existing provider/token patterns. No external HTTP framework.

### D2: In-memory cache with 15-minute TTL

**Choice**: Cache search results in a `map[string]cachedResult` keyed by query string. Each entry has a timestamp; stale entries are evicted on next access. No persistent disk cache.

**Rationale**: Registry data changes slowly. 15 minutes prevents hammering the API while keeping results reasonably fresh. Disk cache adds complexity for minimal benefit — the app restarts reset it anyway.

### D3: Registry server → MCPServerConfig mapping

**Choice**: When a user selects a server from registry results, map its metadata to an `MCPServerConfig`:

- **npm package** → `Command: "npx"`, `Args: ["-y", identifier]`
- **pip package** → `Command: "uvx"`, `Args: [identifier]`
- **Remote (HTTP)** → `URL: remote.url`
- **Environment vars** → pre-populate `Env` map with var names (values empty for user to fill)
- **Name** → derive short name from the registry name (part after `/`, or last segment)

### D4: Fallback to hardcoded list

**Choice**: If the registry API is unreachable (timeout, error, no network), silently fall back to the existing `PopularMCPServers` hardcoded list. Show a dimmed note "(offline — showing built-in servers)".

**Rationale**: The wizard should always work, even without internet.

### D5: TUI browse step with inline search

**Choice**: Replace the current category-grouped static list with a search input + scrollable results list. User types to search, results update live (debounced to avoid spamming API). Enter selects a server and pre-fills the wizard.

### D6: CLI search as table output

**Choice**: `polycode mcp search <query>` prints a table with Name, Description, Transport, Package columns. `polycode mcp browse` is interactive — search, select, then auto-run `mcp add` flow with pre-populated fields.

## Risks / Trade-offs

- **[Network dependency]** → Wizard browse requires internet. Mitigation: hardcoded fallback list.
- **[API stability]** → Registry is v0 (preview). Mitigation: pin to `/v0/servers` endpoint, handle schema changes gracefully with optional fields.
- **[Rate limiting]** → No documented rate limits, but cache prevents excessive requests. Mitigation: 15-min TTL, single fetch per wizard open.
- **[Large result sets]** → Registry has hundreds of servers. Mitigation: default limit of 20 results per search, pagination via cursor.
