## Why

MCP servers frequently require API keys and tokens (e.g., `GITHUB_TOKEN`, `BRAVE_API_KEY`). The registry already flags which env vars are secrets via `isSecret: true`, but this metadata is discarded during config mapping. As a result:

1. **CLI browse** (`polycode mcp browse`) prompts for secrets with plain text input — tokens are visible in the terminal and scrollback
2. **TUI wizard** env var step has no masking — all values are entered and displayed in plain text
3. **Secrets are stored in config.yaml** as plain text instead of using the existing keyring (`auth.Store`) with `$KEYRING:` references

The codebase already has all the building blocks: `huh.EchoModePassword` (CLI), `textinput.EchoPassword` (TUI), `auth.Store` (keyring), and `$KEYRING:` reference resolution in `connectStdio()`. This change connects them for MCP env vars.

## What Changes

- **Preserve secret metadata** through `ToMCPServerConfig` — return env var metadata alongside the config so callers know which vars need masking and keyring storage
- **CLI browse**: use `huh.EchoModePassword` for secret env var prompts, store secret values in keyring with `$KEYRING:mcp_{server}_{var}` references in config.yaml
- **TUI wizard**: use `textinput.EchoPassword` for secret env var input, store in keyring on save
- **TUI wizard display**: mask secret values with `••••` in the env var list and summary steps
- **CLI `mcp add`**: same masking + keyring treatment for manually-entered env vars flagged as secrets
- **PopularMCPServers**: add `IsSecret` flag to the `MCPServerTemplate.EnvVars` entries so hardcoded servers also get masking

## Capabilities

### New Capabilities
- `mcp-secret-metadata`: Preserve env var secret flags through config mapping
- `mcp-secret-input`: Masked input for secret env vars in CLI and TUI
- `mcp-secret-storage`: Keyring storage for secret env vars with $KEYRING: references

### Modified Capabilities

## Impact

- `internal/mcp/registry.go` — `ToMCPServerConfig` returns env var metadata; new `EnvVarMetadata` type
- `cmd/polycode/mcp.go` — browse/add use masked input for secrets, store in keyring
- `internal/tui/mcp_wizard.go` — env step uses EchoPassword for secrets, save stores in keyring, display masks secrets
- `internal/tui/model.go` — `mcpWizardEnvSecrets` field tracks which vars are secret
