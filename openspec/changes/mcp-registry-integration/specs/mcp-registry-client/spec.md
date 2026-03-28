## ADDED Requirements

### Requirement: RegistryClient fetches servers from the MCP Registry API
A `RegistryClient` SHALL exist in `internal/mcp/registry.go` that queries `GET /v0/servers` on `registry.modelcontextprotocol.io` with support for search, pagination, and caching.

#### Scenario: Search by query
- **WHEN** `Search(ctx, "github")` is called
- **THEN** it sends `GET /v0/servers?search=github&limit=20` and returns parsed `RegistryServer` results

#### Scenario: Paginated fetch
- **WHEN** the response includes `metadata.nextCursor`
- **THEN** a subsequent call with the cursor retrieves the next page

#### Scenario: Cache hit within TTL
- **WHEN** the same search query is repeated within 15 minutes
- **THEN** the cached result is returned without an HTTP request

#### Scenario: Cache miss after TTL
- **WHEN** a cached entry is older than 15 minutes
- **THEN** a fresh HTTP request is made and the cache is updated

#### Scenario: API unreachable
- **WHEN** the registry API times out or returns an error
- **THEN** `Search` returns an error that callers can use to trigger fallback

### Requirement: RegistryServer type captures registry metadata
`RegistryServer` SHALL include: Name, Description, Packages (with RegistryType, Identifier, Transport, EnvVars), Remotes (URL, Headers), Repository URL, and Version.

#### Scenario: npm package server parsed
- **WHEN** a server has a package with `registryType: "npm"` and `transport.type: "stdio"`
- **THEN** `RegistryServer.Packages[0]` has RegistryType="npm", Identifier="@scope/name", Transport="stdio"

#### Scenario: Remote-only server parsed
- **WHEN** a server has only `remotes` with `type: "streamable-http"`
- **THEN** `RegistryServer.Remotes[0]` has Type="streamable-http" and URL set
