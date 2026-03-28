## 1. Registry Client

- [x] 1.1 Create `internal/mcp/registry.go` with `RegistryClient` struct: `http.Client`, base URL, cache map
- [x] 1.2 Define `RegistryServer` type with fields: Name, Description, Version, Packages (RegistryType, Identifier, Transport, EnvVars), Remotes (Type, URL, Headers), Repository URL
- [x] 1.3 Implement `Search(ctx, query, limit) ([]RegistryServer, string, error)` — sends GET request, parses JSON response, returns servers + nextCursor
- [x] 1.4 Implement `SearchNext(ctx, query, limit, cursor) ([]RegistryServer, string, error)` — paginated follow-up
- [x] 1.5 Implement in-memory cache: cache key = query+limit, 15-minute TTL, evict on access
- [x] 1.6 Implement `ToMCPServerConfig(server RegistryServer) config.MCPServerConfig` — maps registry metadata to config: npm→npx, pip→uvx, oci→docker, remote→URL, env vars pre-populated, name derived from registry name
- [x] 1.7 Add 5-second timeout on HTTP requests to registry

## 2. Registry Client Tests

- [x] 2.1 Create `internal/mcp/registry_test.go`
- [x] 2.2 Add `TestRegistrySearch` — mock HTTP server returning sample registry JSON, verify parsed RegistryServer fields
- [x] 2.3 Add `TestRegistrySearchPagination` — mock returns nextCursor, verify SearchNext works
- [x] 2.4 Add `TestRegistryCache` — verify second call within TTL doesn't make HTTP request
- [x] 2.5 Add `TestRegistryCacheExpiry` — verify call after TTL makes fresh HTTP request
- [x] 2.6 Add `TestRegistryUnreachable` — mock returns error, verify error propagation
- [x] 2.7 Add `TestToMCPServerConfig_NPM` — npm package maps to npx command
- [x] 2.8 Add `TestToMCPServerConfig_Pip` — pip package maps to uvx command
- [x] 2.9 Add `TestToMCPServerConfig_OCI` — oci package maps to docker run
- [x] 2.10 Add `TestToMCPServerConfig_Remote` — remote-only maps to URL
- [x] 2.11 Add `TestToMCPServerConfig_EnvVars` — env vars pre-populated with empty values

## 3. TUI Wizard Registry Browse

- [x] 3.1 Add `registryClient *mcp.RegistryClient` field to TUI Model, initialize in NewModel or app.go
- [x] 3.2 Add `mcpWizardRegistryResults []mcp.RegistryServer` and `mcpWizardRegistryQuery string` fields to Model
- [x] 3.3 Add `MCPRegistryResultsMsg` TUI message type with servers + error
- [x] 3.4 Modify `mcpStepBrowse` rendering: show search input at top, scrollable results list below with name/description/transport
- [x] 3.5 Modify `updateMCPWizardBrowse`: on text input, fire async registry search; on Enter, select server and call `ToMCPServerConfig`
- [x] 3.6 Handle `MCPRegistryResultsMsg` in Update(): populate `mcpWizardRegistryResults`
- [x] 3.7 Add fallback: if registry search returns error, fall back to hardcoded `PopularMCPServers` with "(offline)" note
- [x] 3.8 Pre-fill wizard fields from selected registry server: name, command/URL, args, env vars, transport

## 4. /mcp search Slash Command

- [x] 4.1 Add `/mcp search <query>` handling in updateChat slash command parser
- [x] 4.2 Wire handler in app.go: call `registryClient.Search(ctx, query, 20)`, format results, send as ConsensusChunkMsg
- [x] 4.3 Format output: table with Name, Description, Transport, Package columns
- [x] 4.4 Add `/mcp search <query>` to slash command palette hints (gated under /mcp)

## 5. CLI Search + Browse Commands

- [x] 5.1 Add `mcpSearchCmd` to main.go: `polycode mcp search <query>` with `cobra.MinimumNArgs(1)`
- [x] 5.2 Implement `runMCPSearch` in mcp.go: create RegistryClient, search, print table
- [x] 5.3 Add `mcpBrowseCmd` to main.go: `polycode mcp browse`
- [x] 5.4 Implement `runMCPBrowse` in mcp.go: huh input for search query → display results as huh select → select server → auto-populate mcp add flow with ToMCPServerConfig → confirm and save
- [x] 5.5 Handle no results and API errors gracefully in both commands

## 6. Documentation + Integration

- [x] 6.1 Update CLAUDE.md: document RegistryClient, registry URL, cache TTL, fallback behavior
- [x] 6.2 Update README.md: add `polycode mcp search` and `polycode mcp browse` to CLI commands table, document registry integration in MCP section
- [x] 6.3 Update CHANGELOG.md with registry integration entry
- [x] 6.4 Add `/mcp search` to slash command palette hints

## 7. Final Verification

- [x] 7.1 Run `go build ./...` — clean compile
- [x] 7.2 Run `go test ./... -count=1 -race` — all packages pass with race detector
- [x] 7.3 Manual test: `polycode mcp search github` returns results
- [x] 7.4 Manual test: `/mcp search github` in TUI shows results in chat
- [x] 7.5 Manual test: wizard browse step shows live registry results
