## ADDED Requirements

### Requirement: ToMCPServerConfig returns env var metadata
`ToMCPServerConfig` SHALL return both a `config.MCPServerConfig` and a `[]EnvVarMeta` slice that preserves `IsSecret`, `IsRequired`, `Name`, and `Description` from the registry.

#### Scenario: Registry server with secret env var
- **WHEN** a registry server has `GITHUB_TOKEN` with `isSecret: true`
- **THEN** the returned `EnvVarMeta` slice includes `{Name: "GITHUB_TOKEN", IsSecret: true}`

#### Scenario: Registry server with non-secret env var
- **WHEN** a registry server has `BASE_URL` with `isSecret: false`
- **THEN** the returned `EnvVarMeta` slice includes `{Name: "BASE_URL", IsSecret: false}`

### Requirement: PopularMCPServers uses typed env var entries
The hardcoded `MCPServerTemplate.EnvVars` SHALL use a typed struct with `Name` and `IsSecret` fields instead of plain strings.

#### Scenario: GitHub template env vars
- **WHEN** the GitHub template is selected
- **THEN** `GITHUB_TOKEN` is marked `IsSecret: true`

#### Scenario: Brave Search template env vars
- **WHEN** the Brave Search template is selected
- **THEN** `BRAVE_API_KEY` is marked `IsSecret: true`
