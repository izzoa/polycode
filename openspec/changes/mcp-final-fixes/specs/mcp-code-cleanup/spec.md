## ADDED Requirements

### Requirement: No orphaned exported functions in MCP package
Every exported function in the MCP package SHALL have at least one in-repo call site. Functions with zero call sites SHALL be removed.

#### Scenario: ReadResource removed
- **WHEN** the codebase is searched for ReadResource
- **THEN** it does not exist in client.go (removed)

#### Scenario: GetPrompt removed
- **WHEN** the codebase is searched for GetPrompt
- **THEN** it does not exist in client.go (removed)

#### Scenario: /mcp resources still works
- **WHEN** ReadResource is removed
- **THEN** /mcp resources command still lists available resources (uses Resources() accessor, not ReadResource)

#### Scenario: /mcp prompts still works
- **WHEN** GetPrompt is removed
- **THEN** /mcp prompts command still lists available prompts (uses Prompts() accessor, not GetPrompt)
