## ADDED Requirements

### Requirement: MCP server configs are validated at load time
`Config.Validate()` SHALL check MCP server configurations and return errors for invalid entries.

#### Scenario: Empty server name
- **WHEN** an MCP server config has an empty name
- **THEN** Validate() returns an error

#### Scenario: Duplicate server names
- **WHEN** two MCP servers have the same name
- **THEN** Validate() returns an error

#### Scenario: No command or URL
- **WHEN** an MCP server config has neither command nor URL set
- **THEN** Validate() returns an error

#### Scenario: Negative timeout
- **WHEN** an MCP server config has a negative timeout
- **THEN** Validate() returns an error

#### Scenario: Valid MCP config passes
- **WHEN** MCP servers have unique names and valid command/URL
- **THEN** Validate() returns nil
