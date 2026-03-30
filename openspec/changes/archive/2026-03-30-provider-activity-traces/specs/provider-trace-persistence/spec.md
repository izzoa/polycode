## ADDED Requirements

### Requirement: Session exchanges persist provider activity traces
The system SHALL persist provider activity traces for each exchange alongside the consensus response.

#### Scenario: Provider traces survive save and load
- **WHEN** a turn completes with provider trace content
- **THEN** saving and later loading the session restores the same provider trace data for that exchange

#### Scenario: Legacy session remains loadable
- **WHEN** a saved session does not contain provider trace data
- **THEN** session load continues to use the legacy per-provider `Individual` summaries without failing

### Requirement: Share and export surfaces prefer persisted provider traces
Session export and other share surfaces SHALL use persisted provider traces when available and fall back to legacy individual summaries otherwise.

#### Scenario: Export uses provider traces
- **WHEN** an exchange contains persisted provider trace data
- **THEN** the exported representation includes that provider trace content for the exchange

#### Scenario: Export falls back to legacy summaries
- **WHEN** an exchange has no provider trace data but does include legacy `Individual` summaries
- **THEN** the exported representation uses the legacy summaries instead of omitting provider details
