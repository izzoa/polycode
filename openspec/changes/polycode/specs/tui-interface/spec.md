## ADDED Requirements

### Requirement: Interactive prompt input
The system SHALL provide a text input area where the user can type natural language prompts and submit them with Enter.

#### Scenario: User types and submits prompt
- **WHEN** the user types a prompt in the input area and presses Enter
- **THEN** the prompt is dispatched to all configured providers and the input area is cleared

#### Scenario: Multi-line input
- **WHEN** the user presses Shift+Enter
- **THEN** a newline is inserted in the input area without submitting the prompt

### Requirement: Provider response panels
The system SHALL display each provider's streaming response in a labeled panel, showing real-time token output and completion status.

#### Scenario: Responses stream in parallel
- **WHEN** multiple providers are streaming responses simultaneously
- **THEN** each provider's panel updates independently with its incoming tokens

#### Scenario: Provider status indicators
- **WHEN** a provider is queried
- **THEN** its panel shows a status indicator: spinning/loading while streaming, checkmark when complete, X when failed or timed out

### Requirement: Consensus output panel
The system SHALL display the consensus synthesis in a distinct, visually prominent panel that is clearly differentiated from individual provider responses.

#### Scenario: Consensus displayed after synthesis
- **WHEN** the primary model completes consensus synthesis
- **THEN** the consensus output is displayed in the primary panel with visual emphasis (e.g., border highlight, label)

#### Scenario: Consensus streams in real time
- **WHEN** the primary model is generating the consensus
- **THEN** tokens appear incrementally in the consensus panel

### Requirement: Conversation history
The system SHALL maintain and display a scrollable conversation history showing all user prompts and consensus responses within the current session.

#### Scenario: Previous exchanges visible
- **WHEN** the user has submitted multiple prompts in a session
- **THEN** all previous prompt/response pairs are visible by scrolling up

#### Scenario: Individual responses expandable
- **WHEN** the user navigates to a previous exchange in the history
- **THEN** they can expand it to view the individual provider responses that contributed to the consensus

### Requirement: Keyboard navigation
The system SHALL support keyboard shortcuts for common actions: quit (Ctrl+C or q), scroll up/down, toggle individual response visibility, and copy output.

#### Scenario: Quit application
- **WHEN** the user presses Ctrl+C
- **THEN** the application exits gracefully

#### Scenario: Toggle individual responses
- **WHEN** the user presses Tab
- **THEN** the display toggles between showing only the consensus panel and showing all individual provider panels alongside it

### Requirement: Provider status bar
The system SHALL display a status bar showing all configured providers, their health status, and which one is designated as primary.

#### Scenario: Status bar at startup
- **WHEN** the application starts
- **THEN** the status bar lists all providers with indicators for health (green/red) and a marker for the primary provider
