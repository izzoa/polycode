## ADDED Requirements

### Requirement: Heuristic provider scoring
The system SHALL score providers based on telemetry history (latency, error rate, usage volume) and use scores to select the best secondary provider in balanced mode.

#### Scenario: Provider with best score selected
- **WHEN** the mode is balanced and multiple secondary providers are available
- **THEN** the router selects the non-primary provider with the highest score

#### Scenario: New provider with no history
- **WHEN** a provider has no telemetry history
- **THEN** it receives a neutral score and is eligible for selection

### Requirement: Periodic calibration in quick/balanced modes
The system SHALL periodically run a full-consensus query in the background to keep routing heuristics fresh.

#### Scenario: Calibration triggered
- **WHEN** the calibration interval (default every 10th query) is reached in quick or balanced mode
- **THEN** a full-consensus query runs silently in the background and results are logged to telemetry

#### Scenario: Calibration disabled
- **WHEN** `routing.calibration_interval: 0` is set in config
- **THEN** no calibration queries are run
