## ADDED Requirements

### Requirement: Keybindings configurable via YAML
The system SHALL support a `keybindings:` section in config.yaml that maps action names to key sequences.

#### Scenario: Custom binding in config
- **WHEN** the config contains `keybindings: { toggle_settings: "ctrl+," }`
- **THEN** pressing Ctrl+, SHALL trigger the toggle_settings action

#### Scenario: Unspecified actions use defaults
- **WHEN** the config keybindings section does not include "quit"
- **THEN** the quit action SHALL use its default binding (ctrl+c)

### Requirement: Default bindings match current hardcoded behavior
The system SHALL ship default bindings for all actions that exactly reproduce the current hardcoded keybindings.

#### Scenario: Default behavior preserved
- **WHEN** no keybindings section exists in config
- **THEN** all keyboard shortcuts SHALL work identically to the current hardcoded behavior

### Requirement: Conflict detection on load
The system SHALL detect when two actions are mapped to the same key sequence and log a warning.

#### Scenario: Duplicate binding warning
- **WHEN** the config maps both "quit" and "cancel" to "ctrl+c"
- **THEN** a warning SHALL be logged indicating the conflict AND the first-defined binding SHALL take precedence

### Requirement: KeyMap provides action lookup from key events
The system SHALL provide an O(1) lookup from a key event string to its bound action name.

#### Scenario: Key-to-action resolution
- **WHEN** a tea.KeyMsg with value "ctrl+s" is received
- **THEN** the KeyMap SHALL resolve it to the "submit" action (or whatever action is bound to ctrl+s)

### Requirement: Action names are stable identifiers
Each bindable action SHALL have a stable string name (e.g., "quit", "submit", "toggle_settings", "copy_last") that does not change across versions.

#### Scenario: Action name stability
- **WHEN** a user configures `keybindings: { submit: "ctrl+enter" }` and upgrades polycode
- **THEN** the "submit" action SHALL still be recognized and the custom binding SHALL apply
