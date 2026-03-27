## ADDED Requirements

### Requirement: All surviving exported MCP functions have unit tests
Every exported function in the MCP package that has at least one call site SHALL have a direct unit test.

#### Scenario: Status returns correct server info
- **WHEN** Status() is called after connecting
- **THEN** it returns entries for all configured servers with correct Connected/ToolCount/Error fields

#### Scenario: Reconnect recovers a server
- **WHEN** Reconnect() is called for a connected server
- **THEN** the server is reconnected and tools are refreshed

#### Scenario: Close shuts down all connections
- **WHEN** Close() is called
- **THEN** all servers are disconnected and tools/resources/prompts are cleared

#### Scenario: CallCount tracks invocations
- **WHEN** CallTool is called N times
- **THEN** CallCount() returns N

#### Scenario: ReadOnlyToolDefinitions filters correctly
- **WHEN** tools have mixed ReadOnly flags
- **THEN** ReadOnlyToolDefinitions returns only tools with ReadOnly=true

#### Scenario: TestConnection validates against mock
- **WHEN** TestConnection is called with a valid config
- **THEN** it returns the correct tool count

#### Scenario: Reconfigure applies config diff
- **WHEN** Reconfigure is called with changed configs
- **THEN** Status reflects the new server set
