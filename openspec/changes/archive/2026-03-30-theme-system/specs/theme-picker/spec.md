## ADDED Requirements

### Requirement: Theme picker overlay accessible via shortcut and command
The system SHALL open a theme picker overlay when the user presses `Ctrl+T` or types `/theme`. The picker SHALL display all available built-in themes in a navigable list.

#### Scenario: Ctrl+T opens theme picker
- **WHEN** the user presses `Ctrl+T` while in chat mode and not querying
- **THEN** a theme picker overlay SHALL appear listing all 6 built-in themes

#### Scenario: /theme command opens theme picker
- **WHEN** the user types `/theme` and presses Enter
- **THEN** the theme picker overlay SHALL open

### Requirement: Theme picker supports keyboard navigation
The picker SHALL support Up/Down arrow keys to move the cursor, Enter to select and apply, and Esc to cancel without changes.

#### Scenario: Navigate and select theme
- **WHEN** the user navigates to "Catppuccin Mocha" and presses Enter
- **THEN** the theme SHALL be applied immediately, the picker SHALL close, and the selection SHALL be persisted to config

#### Scenario: Cancel without applying
- **WHEN** the user presses Esc in the theme picker
- **THEN** the picker SHALL close and the current theme SHALL remain unchanged

### Requirement: Theme picker shows current theme indicator
The picker SHALL indicate which theme is currently active with a visual marker (e.g., "(current)" suffix or highlight).

#### Scenario: Current theme marked
- **WHEN** the theme picker opens with Tokyo Night active
- **THEN** the Tokyo Night entry SHALL display a "(current)" indicator
