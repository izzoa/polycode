## Why

The project has a comprehensive README (30K) and detailed CLAUDE.md, but lacks contributor-facing documentation. There is no `CONTRIBUTING.md`, no architecture decision records, and exported types in key packages (`consensus.Pipeline`, `provider.Provider`, `tokens.TokenTracker`) have minimal godoc comments. This creates a barrier for new contributors and makes the codebase harder to onboard into. As the project approaches v2.0 territory with headless mode and plugin architecture, clear documentation is essential.

## What Changes

- Add `CONTRIBUTING.md` with dev setup, testing guidelines, PR process, and OpenSpec workflow
- Add godoc comments to exported types in core packages
- Add inline architecture notes to key packages (brief package-level doc comments)

## Capabilities

### New Capabilities
<!-- None — documentation only -->

### Modified Capabilities
<!-- No code changes -->

## Impact

- **Files created** (1): `CONTRIBUTING.md`
- **Files modified** (~8): Package-level doc comments in `internal/consensus/`, `internal/provider/`, `internal/tokens/`, `internal/auth/`, `internal/mcp/`, `internal/action/`, `internal/tui/`, `internal/config/`
- **Dependencies**: None
- **Config schema**: No changes
- **Scope**: ~300-400 lines of documentation
