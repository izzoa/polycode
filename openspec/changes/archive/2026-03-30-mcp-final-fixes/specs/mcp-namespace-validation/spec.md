## ADDED Requirements

### Requirement: Config warns about provider/MCP name collision
`Config.Validate()` SHALL log a warning when a provider and MCP server share the same name. It SHALL NOT return an error for this case.

#### Scenario: Same name used for provider and MCP server
- **WHEN** a provider is named "github" and an MCP server is also named "github"
- **THEN** Validate() returns nil (no error) but the collision is detectable

#### Scenario: No collision
- **WHEN** provider and MCP server names are all distinct
- **THEN** Validate() returns nil with no warnings
