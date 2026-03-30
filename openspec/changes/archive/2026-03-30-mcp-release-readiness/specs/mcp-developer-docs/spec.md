## ADDED Requirements

### Requirement: CLAUDE.md documents MCP architecture
The Architecture section SHALL include `internal/mcp/` with a brief description.

#### Scenario: Developer reads architecture
- **WHEN** a developer reads the Architecture section
- **THEN** they see `internal/mcp/` listed as "MCP client — server connections, tool/resource/prompt discovery, JSON-RPC transport"

### Requirement: CLAUDE.md documents MCP key patterns
The Key Patterns section SHALL describe MCP client architecture, tool naming, and transport abstraction.

#### Scenario: Developer reads key patterns
- **WHEN** a developer reads the Key Patterns section
- **THEN** they understand: MCPClient manages server connections, tools are prefixed `mcp_{server}_{tool}`, stdio and HTTP transports exist, and the multiplexed reader handles notifications

### Requirement: CLAUDE.md documents MCP modification guidance
The "When Modifying" section SHALL include guidance for MCP changes.

#### Scenario: Developer modifies MCP
- **WHEN** a developer reads "When Modifying" and needs to change MCP code
- **THEN** they see guidance about: updating discoverTools for new tool metadata, extending Reconfigure for new config fields, adding new MCP methods through sendRequest, and testing with the mock server pattern
