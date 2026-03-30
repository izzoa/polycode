## ADDED Requirements

### Requirement: Worker progress panel in TUI
The system SHALL display a worker progress panel during `/plan` execution showing each worker's role, provider, status, and a summary of its output.

#### Scenario: Workers shown during execution
- **WHEN** a `/plan` job is running
- **THEN** the TUI shows each worker with status: ✓ (complete), ● (running), ○ (pending)

#### Scenario: Completed worker shows summary
- **WHEN** a worker completes
- **THEN** its row in the panel updates to show a one-line summary of its output

#### Scenario: All workers complete
- **WHEN** the final stage completes
- **THEN** the reviewer's output is displayed in the main chat as the final answer and the worker panel fades to show all stages as complete
