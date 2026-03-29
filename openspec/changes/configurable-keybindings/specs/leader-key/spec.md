## ADDED Requirements

### Requirement: Optional leader key with prefix timeout
The system SHALL support an optional leader key configured via `leader_key` in config. When pressed, it arms a prefix state for 500ms during which the next keypress completes a leader-prefixed binding.

#### Scenario: Leader key triggers prefix state
- **WHEN** the leader key (e.g., Space) is pressed
- **THEN** the system SHALL enter leader-armed state for 500ms

#### Scenario: Leader+key completes binding
- **WHEN** the system is in leader-armed state and the user presses "t"
- **THEN** the system SHALL resolve the binding for `<leader>t` and execute the bound action

#### Scenario: Leader timeout expires
- **WHEN** the system is in leader-armed state and 500ms passes with no keypress
- **THEN** the leader-armed state SHALL expire and the leader key event SHALL be processed normally (e.g., Space inserts a space)

### Requirement: Leader-armed visual indicator
The system SHALL display a subtle visual indicator (e.g., "LEADER" in the status bar) when leader-armed state is active.

#### Scenario: Indicator shown while armed
- **WHEN** the leader key is pressed
- **THEN** a "LEADER" indicator SHALL appear in the status bar AND disappear when the state expires or completes

### Requirement: Leader prefix expanded at parse time
Bindings using `<leader>` prefix SHALL be expanded to the configured leader key at config load time.

#### Scenario: Leader expansion
- **WHEN** the config has `leader_key: " "` and binding `toggle_theme: "<leader>t"`
- **THEN** the binding SHALL be stored internally as requiring Space followed by "t" within the timeout window

### Requirement: Leader key disabled when textarea focused
The leader key SHALL not arm prefix state when a text input field (chat input, edit textarea) has focus.

#### Scenario: Leader key types normally in input
- **WHEN** the chat input textarea is focused and the user presses the leader key (Space)
- **THEN** a space character SHALL be inserted into the textarea AND leader-armed state SHALL NOT activate
