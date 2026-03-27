## ADDED Requirements

### Requirement: MCPClient supports runtime reconfiguration
The MCPClient SHALL provide a `Reconfigure(configs []MCPServerConfig)` method that diffs the current config against the new config, connects newly-added servers, disconnects removed servers, and reconnects servers whose config changed — without replacing the MCPClient pointer.

#### Scenario: Server added at runtime
- **WHEN** `Reconfigure` is called with a new server config not present in the current configs
- **THEN** the new server is connected, its tools/resources/prompts are discovered, and Status() reflects the new server

#### Scenario: Server removed at runtime
- **WHEN** `Reconfigure` is called without a server that was previously configured
- **THEN** the server is disconnected, its tools/resources/prompts are removed from all accessors, and Status() no longer lists it

#### Scenario: Server config changed at runtime
- **WHEN** `Reconfigure` is called with a server whose command, args, URL, or env differs from the current config
- **THEN** the server is reconnected with the new config and all metadata is refreshed

### Requirement: Wizard save triggers live reconfiguration
The TUI wizard save path SHALL trigger `Reconfigure` via the config-change handler so that add/edit/delete operations take effect immediately without requiring a restart.

#### Scenario: Wizard adds a new MCP server
- **WHEN** the user saves a new server via the MCP wizard
- **THEN** the new server is connected and its tools appear in the model's tool list within the same session

#### Scenario: Wizard deletes an MCP server
- **WHEN** the user deletes a server via the settings view
- **THEN** the server is disconnected and removed from Status() within the same session

### Requirement: /mcp remove triggers live disconnection and config cleanup
`/mcp remove` SHALL disconnect the server, remove it from `c.configs`, and update Status() immediately.

#### Scenario: /mcp remove cleans up all state
- **WHEN** the user runs `/mcp remove myserver`
- **THEN** the server is disconnected, removed from configs, tools, resources, prompts, and Status() no longer lists it
