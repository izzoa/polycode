## ADDED Requirements

### Requirement: Export sessions as shareable artifacts
The system SHALL allow exporting the current session as markdown or JSON via `polycode export`.

#### Scenario: Export as markdown
- **WHEN** the user runs `polycode export --format md`
- **THEN** the system writes a markdown file with prompts, responses, agreement/disagreement, and tool actions

#### Scenario: Export as JSON
- **WHEN** the user runs `polycode export --format json`
- **THEN** the system writes the full session JSON with all structured data

### Requirement: Import sessions for replay
The system SHALL allow importing a previously exported session via `polycode import <file>`.

#### Scenario: Import JSON session
- **WHEN** the user runs `polycode import session.json`
- **THEN** the session is loaded and the conversation history is restored in the TUI
