## ADDED Requirements

### Requirement: Display ASCII art banner on TUI startup
The system SHALL display an ASCII art "polycode" banner with version and tagline when the TUI launches.

#### Scenario: Normal startup
- **WHEN** the user runs `polycode` and the TUI initializes
- **THEN** the system displays the ASCII art banner centered on screen with the version number and tagline "multi-model consensus coding assistant"

### Requirement: Auto-dismiss splash after timeout
The system SHALL automatically transition from the splash screen to the main TUI after 1.5 seconds.

#### Scenario: No user input during splash
- **WHEN** the splash screen is displayed and 1.5 seconds pass without user input
- **THEN** the system transitions to the main TUI view

### Requirement: Dismiss splash on any keypress
The system SHALL immediately dismiss the splash screen and transition to the main TUI when the user presses any key.

#### Scenario: User presses key during splash
- **WHEN** the splash screen is displayed and the user presses any key
- **THEN** the system immediately transitions to the main TUI view
