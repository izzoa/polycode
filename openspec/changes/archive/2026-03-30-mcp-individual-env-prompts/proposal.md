## Why

The MCP wizard env step currently uses a freeform `KEY=value` text input for all environment variables. When a server is selected from the registry or hardcoded list, we already know which env vars are needed, which are required, and which are secrets — but this metadata is wasted because the freeform input can't mask individual values or prompt for known vars by name.

The result: users must manually type `GITHUB_TOKEN=ghp_abc...` with the entire token visible, even though we know that var is a secret. The `EchoPassword` mode can't help because it would mask both the key name and the value.

## What Changes

- **Two-phase env step**: First, prompt each known/pre-populated env var individually (name shown as label, value as input — secret vars get `EchoPassword`). Second, offer freeform `KEY=value` for any additional vars.
- **Env var prompting order**: Required vars first, then optional, then freeform.
- **Per-var labels**: Show the env var name as the prompt title, with description if available from registry metadata, and "(required)" / "(secret)" indicators.
- **Wizard state**: Track which known env var is currently being prompted via an index into the pre-populated list.

## Capabilities

### New Capabilities
- `mcp-individual-env-prompts`: Known env vars prompted individually with per-var masking and labels

### Modified Capabilities

## Impact

- `internal/tui/mcp_wizard.go` — env step rendering + input handling split into known-var phase and freeform phase
- `internal/tui/model.go` — new fields for tracking current known env var index and ordered list
