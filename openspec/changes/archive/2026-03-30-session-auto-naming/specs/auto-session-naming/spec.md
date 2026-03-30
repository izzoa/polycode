## ADDED Requirements

### Requirement: Auto-generate session name after first exchange
The system SHALL automatically generate a 3-5 word descriptive session name by querying the primary model after the first query-response exchange completes.

#### Scenario: First exchange triggers naming
- **WHEN** the first exchange in a new session completes (QueryDoneMsg received for exchange index 0)
- **THEN** the system SHALL send a background naming request to the primary model AND apply the result as the session name

#### Scenario: Subsequent exchanges do not re-trigger naming
- **WHEN** the second or later exchange completes in a session that already has an auto-generated name
- **THEN** the system SHALL NOT send another naming request

### Requirement: Session name displays in status bar and picker
The session name SHALL be displayed in the status bar alongside existing metadata and in the session picker list.

#### Scenario: Named session in picker
- **WHEN** a session has name "Refactor auth module"
- **THEN** the session picker SHALL display "Refactor auth module" instead of or alongside the timestamp

### Requirement: User can override name via slash command
The system SHALL support `/sessions name <text>` to set a custom session name. A user-set name SHALL prevent auto-naming from overwriting it.

#### Scenario: User renames session
- **WHEN** the user types `/sessions name My custom project`
- **THEN** the session name SHALL be set to "My custom project" AND the UserNamed flag SHALL be set to true

#### Scenario: Auto-naming skips user-named sessions
- **WHEN** a session has UserNamed=true and the first exchange completes
- **THEN** the system SHALL NOT send a naming request

### Requirement: Session name persists across restarts
The session name SHALL be saved to the session metadata file and restored when the session is loaded.

#### Scenario: Name survives restart
- **WHEN** a session named "Debug websocket issue" is saved and the application restarts
- **THEN** the session SHALL load with name "Debug websocket issue"

### Requirement: Auto-generated names are concise
Auto-generated names SHALL be truncated to 40 characters maximum with trailing punctuation and quotes stripped.

#### Scenario: Long name is truncated
- **WHEN** the primary model returns "Refactoring the authentication module to support OAuth2 flows"
- **THEN** the stored name SHALL be at most 40 characters
