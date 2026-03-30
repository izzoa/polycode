## Context

MCP enhancements landed 18 features + partial fixes verified by 5 rounds of Codex code review. All 17 test packages pass including race detector. The code is functionally correct but a pre-release audit found: no tests for Reconfigure/TestConnection/parseSSEResponse/HTTP transport, CLAUDE.md omits MCP entirely, Config.Validate() ignores MCP, and the `mcp_{server}_{tool}` naming format has a collision edge case.

## Goals / Non-Goals

**Goals:**
- Every critical MCP code path has at least one unit test
- Developers have CLAUDE.md guidance for modifying MCP code
- Invalid MCP configs are caught at load time, not runtime
- Tool name collisions are detected and surfaced clearly

**Non-Goals:**
- 100% line coverage (not needed — focus on behavioral coverage of complex paths)
- Integration/E2E tests (unit tests are sufficient for this scope)
- Changing the `mcp_{server}_{tool}` naming format (would be breaking)

## Decisions

### D1: Test strategy — mock server reuse + standalone tests

Reuse the existing `newTestClient` pattern for tests that need a live connection (multiplexed reader, resource/prompt discovery). Use pure state manipulation for Reconfigure/DisconnectServer tests. Use `net/http/httptest` for HTTP transport and SSE tests.

### D2: Collision detection via error at discovery time

During `discoverTools()`, if a prefixed name already exists in `toolIndex` from a *different* server, log a warning and skip the duplicate. This prevents silent tool shadowing. Not an error — allows the rest of the server's tools to work.

### D3: Config validation — warn on MCP issues, don't block startup

Add MCP validation to `Validate()` but make MCP errors warnings (logged) rather than hard errors, since MCP is optional and shouldn't prevent the app from starting when only provider config is valid. However, duplicate server names and empty names are hard errors.

### D4: CLAUDE.md — concise additions to existing sections

Add MCP to the Architecture tree, Key Patterns, and "When Modifying" sections. Don't create a separate MCP section — integrate into the existing structure.

## Risks / Trade-offs

- **[Mock server complexity]** → The mock server in client_test.go doesn't support resources/list or prompts/list. Mitigation: extend the mock to handle these methods.
- **[Collision detection false positives]** → Two servers legitimately providing the same tool name would be flagged. Mitigation: log a warning instead of erroring, so the first server's tool wins.
