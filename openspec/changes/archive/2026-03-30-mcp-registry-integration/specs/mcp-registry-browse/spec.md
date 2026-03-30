## ADDED Requirements

### Requirement: Wizard browse step fetches from live registry
When the user selects "Popular servers" in the MCP wizard, the browse step SHALL fetch servers from the MCP Registry API instead of using the hardcoded list.

#### Scenario: Registry available
- **WHEN** the user enters the browse step and the registry is reachable
- **THEN** the wizard shows a search input and scrollable list of registry servers with name and description

#### Scenario: Registry unreachable
- **WHEN** the registry API fails or times out
- **THEN** the wizard falls back to the hardcoded `PopularMCPServers` list with a dimmed "(offline)" note

#### Scenario: User searches in browse
- **WHEN** the user types a search query in the browse step
- **THEN** the results update to show matching servers from the registry

### Requirement: Browse step shows transport and package info
Each server entry in the browse list SHALL show the server name, description, and transport type (stdio/http).

#### Scenario: npm server display
- **WHEN** a server has an npm package with stdio transport
- **THEN** the entry shows the server name, description, and "npm/stdio"

#### Scenario: Remote-only server display
- **WHEN** a server has only remote endpoints
- **THEN** the entry shows the server name, description, and "http"
