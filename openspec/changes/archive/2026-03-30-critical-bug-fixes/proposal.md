## Why

A comprehensive code analysis identified 15 bugs across the codebase ranging from build-breaking (impossible go.mod version) to security (shell injection in hooks) to data correctness (Gemini tool calls dropped, cost tracking lost on config change). These bugs affect core functionality and need to be fixed before further feature development.

## What Changes

### P0 — Critical
- Fix go.mod version from impossible `1.26.1` to actual toolchain version
- Fix Gemini tool calls being silently dropped when split across SSE chunks
- Fix shell injection vulnerability in hooks template rendering

### P1 — High
- Fix Gemini system messages using wrong role (causes API errors)
- Re-wire cost tracking after config changes
- Fix auto-summarization using accumulated tokens instead of last-request tokens
- Add max iteration limit (25 rounds) to fan-out tool loops
- (Bug 8 — data race — deferred to separate change, requires architectural refactor)

### P2 — Medium
- Increase MCP stdio scanner buffer from 64KB to 4MB
- Fix Anthropic custom base URL missing /v1/messages path

### P3 — Low
- Expand destructive command detection patterns
- Fix session restore gating exchange history on Messages being non-empty
- Make MCP Connect() idempotent by clearing tools/index on re-call

## Impact

- **Files modified**: go.mod, gemini.go, hooks.go, anthropic.go, app.go, fanout.go, shell.go, client.go
- **Security**: Hooks shell injection closed
- **Correctness**: Gemini tool execution fixed, cost tracking preserved, auto-summarization accurate
- **Robustness**: Tool loops bounded, MCP handles large responses
