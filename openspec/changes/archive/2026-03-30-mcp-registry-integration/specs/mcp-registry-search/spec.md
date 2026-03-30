## ADDED Requirements

### Requirement: /mcp search slash command
The TUI SHALL support `/mcp search <query>` to search the MCP Registry and display results in the chat area.

#### Scenario: Search with results
- **WHEN** the user runs `/mcp search github`
- **THEN** matching servers are displayed with name, description, transport, and package identifier

#### Scenario: Search with no results
- **WHEN** the user runs `/mcp search xyznonexistent`
- **THEN** a "No servers found" message is displayed

#### Scenario: Registry unreachable during search
- **WHEN** the registry API fails during a `/mcp search` call
- **THEN** an error message is displayed

### Requirement: polycode mcp search CLI command
The CLI SHALL support `polycode mcp search <query>` to search the MCP Registry and print results as a table.

#### Scenario: CLI search with results
- **WHEN** the user runs `polycode mcp search database`
- **THEN** a table is printed with Name, Description, Transport, Package columns

#### Scenario: CLI search with no results
- **WHEN** the user runs `polycode mcp search xyznonexistent`
- **THEN** "No servers found for 'xyznonexistent'" is printed

### Requirement: polycode mcp browse CLI command
The CLI SHALL support `polycode mcp browse` for interactive registry browsing with search and server selection.

#### Scenario: Browse, search, and select
- **WHEN** the user runs `polycode mcp browse`, enters a search query, and selects a server
- **THEN** the selected server's config is auto-populated and the user is prompted to confirm and save

#### Scenario: Browse and cancel
- **WHEN** the user presses Esc or Ctrl+C during browse
- **THEN** the command exits cleanly with no config changes
