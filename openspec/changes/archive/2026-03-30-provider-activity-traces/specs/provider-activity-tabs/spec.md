## ADDED Requirements

### Requirement: Provider tabs show the full participation trace for each provider
The system SHALL render each provider tab as the chronological trace of every runtime phase that provider participated in during the current turn.

#### Scenario: Non-primary provider shows fan-out work only
- **WHEN** a non-primary provider participates only in the fan-out phase
- **THEN** its tab shows the fan-out output for that turn and does not invent later phases

#### Scenario: Primary provider shows all phases it performed
- **WHEN** the primary provider participates in fan-out, synthesis, tool execution, follow-up generation, and verification
- **THEN** its tab shows entries for each of those phases in the order they occurred

### Requirement: Provider tab completion reflects the full provider lifecycle
A provider tab SHALL not be marked complete until all phases for that provider in the current turn have completed.

#### Scenario: Primary provider remains in progress after fan-out
- **WHEN** the primary provider has finished fan-out but synthesis or tool execution is still running
- **THEN** the primary provider tab remains in a loading or in-progress state

#### Scenario: Provider tab completes after final phase
- **WHEN** the final phase for a provider ends successfully
- **THEN** that provider tab transitions to done and preserves the accumulated trace content

#### Scenario: Provider error is surfaced in the trace
- **WHEN** a provider fails during any phase of its turn
- **THEN** the provider tab records the error in the trace and marks that provider as failed

### Requirement: Provider traces label phase boundaries
The system SHALL visually separate major provider phases so users can distinguish fan-out, synthesis, tool execution, and verification output.

#### Scenario: New phase starts
- **WHEN** a provider begins emitting output for a new phase
- **THEN** the rendered provider trace inserts a visible phase label before that phase's content

#### Scenario: Multiple chunks in the same phase
- **WHEN** consecutive trace chunks belong to the same provider phase
- **THEN** the system appends them under the existing phase section instead of repeating the phase label for every chunk
