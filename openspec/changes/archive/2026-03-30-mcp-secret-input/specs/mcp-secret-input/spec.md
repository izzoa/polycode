## ADDED Requirements

### Requirement: CLI browse uses masked input for secret env vars
`polycode mcp browse` SHALL use `huh.EchoModePassword` when prompting for env vars marked as secret.

#### Scenario: Secret env var prompted with masking
- **WHEN** the user is prompted for `GITHUB_TOKEN` (isSecret=true) during browse
- **THEN** the input is masked with dots/bullets and not visible in terminal scrollback

#### Scenario: Non-secret env var prompted normally
- **WHEN** the user is prompted for `BASE_URL` (isSecret=false) during browse
- **THEN** the input is shown in plain text

### Requirement: TUI wizard uses masked input for secret env vars
The MCP wizard env step SHALL use `textinput.EchoPassword` when the current env var being entered is marked as secret.

#### Scenario: Secret env var in wizard
- **WHEN** the user enters a value for a secret env var in the TUI wizard
- **THEN** the input characters are shown as `•` (password mode)

### Requirement: Secret values displayed as masked in wizard summary
The wizard summary and env var list steps SHALL display secret values as `••••••••` instead of the actual value.

#### Scenario: Summary shows masked secret
- **WHEN** the wizard reaches the summary/confirm step
- **THEN** secret env var values show as `•••••••• (configured)` and non-secret values show the actual text
