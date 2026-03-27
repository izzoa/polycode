## ADDED Requirements

### Requirement: Wizard test uses staged config
The MCP wizard test step SHALL validate the in-progress MCPServerConfig by spawning a temporary connection, performing the initialize handshake, and discovering tools — without modifying the live MCPClient state.

#### Scenario: Testing a new server that doesn't exist yet
- **WHEN** the user reaches the test step while adding a new server
- **THEN** a temporary connection is created from the wizard's staged config, the test reports success/failure and tool count, and no state is mutated on the live MCPClient

#### Scenario: Testing an edited server with changed config
- **WHEN** the user reaches the test step while editing an existing server's command or args
- **THEN** the test validates the new config (not the saved config) via a temporary connection

#### Scenario: Test step auto-triggers on entry
- **WHEN** the wizard advances to mcpStepTest
- **THEN** the connection test begins automatically without requiring an additional keypress

### Requirement: TestConnection is a standalone function
A standalone `TestConnection(ctx, cfg MCPServerConfig) (toolCount int, err error)` function SHALL exist that creates a temporary connection, validates it, and tears it down without side effects.

#### Scenario: TestConnection succeeds
- **WHEN** called with a valid MCPServerConfig
- **THEN** returns the number of discovered tools and a nil error

#### Scenario: TestConnection fails
- **WHEN** called with an invalid command or unreachable URL
- **THEN** returns 0 and a descriptive error
