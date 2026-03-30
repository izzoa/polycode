## ADDED Requirements

### Requirement: Registry server maps to MCPServerConfig
When a user selects a server from registry results, its metadata SHALL be mapped to an `MCPServerConfig` with correct command, args, URL, and env vars.

#### Scenario: npm package maps to npx command
- **WHEN** a server has package `registryType: "npm"`, `identifier: "@modelcontextprotocol/server-github"`
- **THEN** the config is populated with `Command: "npx"`, `Args: ["-y", "@modelcontextprotocol/server-github"]`

#### Scenario: pip package maps to uvx command
- **WHEN** a server has package `registryType: "pip"`, `identifier: "mcp-server-sqlite"`
- **THEN** the config is populated with `Command: "uvx"`, `Args: ["mcp-server-sqlite"]`

#### Scenario: Remote-only server maps to URL
- **WHEN** a server has only a remote with `type: "streamable-http"`, `url: "https://example.com/mcp"`
- **THEN** the config is populated with `URL: "https://example.com/mcp"`

#### Scenario: Environment variables pre-populated
- **WHEN** a server's package has `environmentVariables` with `name: "GITHUB_TOKEN"`, `isRequired: true`
- **THEN** the config's `Env` map includes `"GITHUB_TOKEN": ""` (empty value for user to fill)

#### Scenario: Server name derived from registry name
- **WHEN** a server's registry name is `ai.smithery/github-mcp`
- **THEN** the derived config name is `github-mcp` (part after `/`)

### Requirement: OCI package maps to docker command
When a server has an OCI (container) package, it SHALL map to a docker run command.

#### Scenario: OCI package mapping
- **WHEN** a server has package `registryType: "oci"`, `identifier: "docker.io/org/server:1.0"`
- **THEN** the config is populated with `Command: "docker"`, `Args: ["run", "--rm", "-i", "docker.io/org/server:1.0"]`
