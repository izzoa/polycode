## ADDED Requirements

### Requirement: Persistent repo memory
The system SHALL load markdown files from `~/.config/polycode/memory/` and include their content in the system prompt for every query.

#### Scenario: Memory files loaded on startup
- **WHEN** polycode starts and memory files exist (build.md, architecture.md, conventions.md)
- **THEN** their content is appended to the system prompt

#### Scenario: No memory files
- **WHEN** no memory files exist
- **THEN** the system prompt uses only the default and instruction hierarchy content

### Requirement: /memory command
The system SHALL provide a `/memory` command to view and edit repo memory from within the TUI.

#### Scenario: View memory
- **WHEN** the user types `/memory`
- **THEN** the TUI displays the contents of all memory files

#### Scenario: Edit memory
- **WHEN** the user types `/memory edit build`
- **THEN** the TUI opens an editor for the build.md memory file
