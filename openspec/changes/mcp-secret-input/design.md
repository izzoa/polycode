## Context

The MCP config already supports `$KEYRING:key_name` references in env var values — `connectStdio()` resolves them at runtime via `auth.Store.Get()`. The registry client already parses `isSecret` on env vars. The provider wizard already uses `textinput.EchoPassword` for API keys and `huh.EchoModePassword` for CLI key entry. The plumbing exists; it just needs to be connected for MCP env vars.

Current gaps:
- `ToMCPServerConfig()` returns `Env map[string]string` with empty values — no way to know which are secrets
- CLI browse prompts all vars with plain `huh.NewInput()` — tokens visible
- TUI wizard env step has no masking
- Secrets stored as plain text in config.yaml instead of `$KEYRING:` references

## Goals / Non-Goals

**Goals:**
- Secret env vars entered with masked/password input in both CLI and TUI
- Secret values stored in keyring via `auth.Store`, config.yaml contains `$KEYRING:mcp_{server}_{var}` references
- Secret values displayed as `••••` in wizard summary and env var list
- Registry `isSecret` metadata flows through to all input paths
- Hardcoded `PopularMCPServers` env vars gain `IsSecret` flag

**Non-Goals:**
- Encrypting the config file itself
- Managing keyring entries outside MCP (provider auth already handles its own keys)
- Adding secret detection heuristics (only explicit `isSecret` flags)

## Decisions

### D1: New `EnvVarMeta` type alongside config mapping

**Choice**: Add `EnvVarMeta` struct with `Name`, `Description`, `IsSecret`, `IsRequired` fields. `ToMCPServerConfig` returns both the config and a `[]EnvVarMeta` slice. Callers use the metadata for input masking and keyring decisions.

**Alternative**: Embed secret flags in the config struct. Rejected — config.yaml shouldn't grow metadata fields that only matter during setup.

### D2: Keyring key format `mcp_{server}_{var}`

**Choice**: Store MCP secrets with keyring key `mcp_{serverName}_{envVarName}` (e.g., `mcp_github_GITHUB_TOKEN`). Config.yaml stores `$KEYRING:mcp_github_GITHUB_TOKEN`. This matches the existing `$KEYRING:` resolution in `connectStdio()`.

**Rationale**: Follows the pattern already proposed in MCP_ENHANCEMENTS.md and already implemented in `connectStdio()`. Keys are namespaced by server to avoid collisions.

### D3: TUI wizard tracks secrets via `mcpWizardEnvSecrets map[string]bool`

**Choice**: Add a map on the Model that tracks which env var names are secrets. Set during browse/template selection. Used by the env input step to toggle `EchoPassword` and by the summary step to mask display.

### D4: MCPServerTemplate gains `EnvSecrets` for hardcoded servers

**Choice**: Extend the `MCPServerTemplate.EnvVars` from `[]string` to `[]MCPTemplateEnvVar` with `Name` and `IsSecret` fields, so hardcoded popular servers (GitHub, Slack, Brave Search) also get masked input.

**Alternative**: Keep `[]string` and detect secrets by name pattern (e.g., contains `TOKEN`, `KEY`, `SECRET`). Rejected — too fragile and inconsistent with the explicit registry metadata approach.

## Risks / Trade-offs

- **[Keyring availability]** → If no keyring is available, `auth.Store` falls back to file-based storage (already handled by the auth package). No MCP-specific risk.
- **[Template type change]** → Changing `EnvVars []string` to `[]MCPTemplateEnvVar` on `MCPServerTemplate` is a breaking change to the internal type. But it's internal and only used in `mcp_wizard.go`. Low risk.
