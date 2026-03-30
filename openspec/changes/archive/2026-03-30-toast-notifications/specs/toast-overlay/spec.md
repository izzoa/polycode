## ADDED Requirements

### Requirement: Toast notification renders as a non-blocking overlay
The system SHALL render toast notifications as overlays in the bottom-right corner of the viewport without stealing focus or interrupting input.

#### Scenario: Toast appears without blocking input
- **WHEN** a ToastMsg is received while the user is typing in the chat input
- **THEN** the toast SHALL appear in the bottom-right corner AND the chat input SHALL remain focused and editable

### Requirement: Toasts stack up to a maximum of 3
The system SHALL display at most 3 concurrent toasts, stacked vertically. When a 4th toast arrives, the oldest toast SHALL be evicted immediately.

#### Scenario: Fourth toast evicts oldest
- **WHEN** 3 toasts are visible and a new ToastMsg is received
- **THEN** the oldest toast SHALL be removed AND the new toast SHALL appear at the bottom of the stack

### Requirement: Toasts auto-dismiss after a timer
Each toast SHALL auto-dismiss after a duration based on its variant: 3 seconds for Info/Success/Warning, 5 seconds for Error.

#### Scenario: Success toast auto-dismisses
- **WHEN** a Success toast is created
- **THEN** it SHALL be removed from the display after 3 seconds

#### Scenario: Error toast has longer display
- **WHEN** an Error toast is created
- **THEN** it SHALL be removed from the display after 5 seconds

### Requirement: Four toast variants with distinct styling
The system SHALL support four toast variants: Info (blue), Success (green), Warning (yellow), Error (red). Each variant SHALL use theme-appropriate colors for its border or accent.

#### Scenario: Each variant renders with correct color
- **WHEN** toasts of each variant type are displayed
- **THEN** Info SHALL use blue tones, Success SHALL use green, Warning SHALL use yellow, Error SHALL use red
