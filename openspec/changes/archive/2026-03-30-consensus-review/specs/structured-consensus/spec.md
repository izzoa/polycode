## ADDED Requirements

### Requirement: Structured consensus synthesis prompt
The system SHALL use an enhanced consensus prompt that instructs the primary model to produce a structured synthesis containing: recommendation, confidence level, areas of agreement, minority reports, and cited evidence.

#### Scenario: Structured synthesis output
- **WHEN** the primary model synthesizes responses from multiple providers
- **THEN** the synthesis prompt requests sections for Recommendation, Confidence, Agreement, Minority Report, and Evidence

#### Scenario: Single provider fallback preserves structure
- **WHEN** only one provider responds and synthesis is skipped
- **THEN** the response is displayed as-is without forced structure

### Requirement: Confidence level in consensus
The system SHALL extract a confidence level (high, medium, or low) from the synthesis output and display it in the TUI.

#### Scenario: High confidence consensus
- **WHEN** all providers agree on the approach and the primary synthesizes with "Confidence: high"
- **THEN** the TUI displays a green confidence indicator

#### Scenario: Low confidence consensus
- **WHEN** providers significantly disagree and the primary synthesizes with "Confidence: low"
- **THEN** the TUI displays a red confidence indicator

### Requirement: Minority reports
The system SHALL extract and display minority positions from the synthesis — cases where one or more models disagreed with the consensus and provided an alternative with reasoning.

#### Scenario: Minority report present
- **WHEN** the synthesis contains a "Minority Report" section with a dissenting model's view
- **THEN** the TUI makes the minority report viewable via the provenance panel

#### Scenario: No minority report
- **WHEN** all models agree and the synthesis contains no minority report
- **THEN** the provenance panel shows "All models agreed" and no minority section

### Requirement: Provenance panel in TUI
The system SHALL provide a toggleable provenance panel (via `p` key) that shows consensus details: confidence, agreement areas, minority reports, and which models contributed.

#### Scenario: Toggle provenance panel
- **WHEN** the user presses `p` in the chat view after a consensus response
- **THEN** the provenance panel appears below the consensus showing confidence, agreement, and minority reports

#### Scenario: Provenance panel toggle off
- **WHEN** the user presses `p` again while the provenance panel is visible
- **THEN** the provenance panel is hidden
