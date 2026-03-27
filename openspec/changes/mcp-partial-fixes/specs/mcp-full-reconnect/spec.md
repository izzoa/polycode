## ADDED Requirements

### Requirement: Reconnect refreshes tools, resources, and prompts
`reconnectServer()` SHALL re-discover resources and prompts alongside tools after reconnecting a server. Old per-server entries in the resources and prompts slices MUST be replaced.

#### Scenario: Auto-reconnect after server crash
- **WHEN** a server dies and CallTool triggers auto-reconnect
- **THEN** tools, resources, and prompts for that server are all refreshed

#### Scenario: Manual /mcp reconnect
- **WHEN** the user runs `/mcp reconnect myserver`
- **THEN** tools, resources, and prompts for that server are all refreshed and /mcp resources and /mcp prompts reflect the new state

### Requirement: DisconnectServer cleans up resources and prompts
`DisconnectServer()` SHALL remove resources and prompts for the disconnected server in addition to tools and index entries.

#### Scenario: DisconnectServer removes all metadata
- **WHEN** `DisconnectServer("myserver")` is called
- **THEN** Resources() and Prompts() no longer contain entries with ServerName "myserver"
