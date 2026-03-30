## ADDED Requirements

### Requirement: Secret env vars stored in keyring with $KEYRING: references
When saving an MCP server config, secret env var values SHALL be stored in the keyring via `auth.Store.Set()` and the config.yaml SHALL contain `$KEYRING:mcp_{server}_{var}` as the value.

#### Scenario: Secret stored on wizard save
- **WHEN** the user saves an MCP server with `GITHUB_TOKEN` = "ghp_abc123" (isSecret=true)
- **THEN** `auth.Store.Set("mcp_github_GITHUB_TOKEN", "ghp_abc123")` is called AND config.yaml contains `GITHUB_TOKEN: $KEYRING:mcp_github_GITHUB_TOKEN`

#### Scenario: Non-secret stored as plain text
- **WHEN** the user saves an MCP server with `BASE_URL` = "http://localhost" (isSecret=false)
- **THEN** config.yaml contains `BASE_URL: http://localhost` (no keyring)

### Requirement: Secret env vars stored in keyring during CLI browse save
`polycode mcp browse` SHALL store secret values in keyring and write `$KEYRING:` references to config, matching the wizard behavior.

#### Scenario: CLI browse stores secret in keyring
- **WHEN** the user completes `polycode mcp browse` for a server with a secret env var
- **THEN** the secret value is stored in keyring and config.yaml has the `$KEYRING:` reference

### Requirement: Existing $KEYRING: resolution continues to work
`connectStdio()` SHALL continue resolving `$KEYRING:key_name` references at runtime via `auth.Store.Get()`.

#### Scenario: Runtime resolution
- **WHEN** an MCP server starts with `GITHUB_TOKEN: $KEYRING:mcp_github_GITHUB_TOKEN` in config
- **THEN** `connectStdio()` resolves it from keyring and passes the actual value as an environment variable to the subprocess
