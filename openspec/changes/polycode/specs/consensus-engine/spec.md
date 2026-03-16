## ADDED Requirements

### Requirement: Consensus synthesis via primary model
The system SHALL send all collected provider responses to the designated primary model with a structured consensus prompt, and use the primary model's synthesis as the final answer.

#### Scenario: Multiple responses synthesized
- **WHEN** 3 providers have completed their responses to a user prompt
- **THEN** the system constructs a consensus prompt containing the original user prompt and all 3 responses, sends it to the primary model, and presents the primary's synthesis as the final output

#### Scenario: Two responses synthesized
- **WHEN** only 2 providers completed (one timed out)
- **THEN** the system synthesizes consensus from the 2 available responses via the primary model

### Requirement: Consensus prompt structure
The system SHALL use a structured prompt template that includes the original user question and all model responses, clearly labeled by provider name, instructing the primary model to analyze agreement, identify unique insights, flag errors, and produce an authoritative synthesis.

#### Scenario: Prompt contains all responses
- **WHEN** the consensus prompt is constructed
- **THEN** it includes the original user prompt, each model's response labeled with the provider name, and instructions to synthesize

#### Scenario: Provider names are visible
- **WHEN** the consensus prompt is sent to the primary
- **THEN** each response section is labeled with the provider's configured name so the primary can reference them

### Requirement: Primary model excluded from fan-out when synthesizing
The system SHALL include the primary model in the initial fan-out query (so it generates its own independent response), then use it again for synthesis. The primary's own initial response is included alongside other responses in the consensus prompt.

#### Scenario: Primary provides initial response and synthesizes
- **WHEN** a query is dispatched
- **THEN** the primary model receives the user prompt in the fan-out phase, its response is collected, and then the primary receives a second call with all responses (including its own) for synthesis

### Requirement: Context window overflow handling
The system SHALL detect when the combined responses would exceed the primary model's context window limit and truncate individual responses proportionally to fit.

#### Scenario: Responses fit within context window
- **WHEN** all collected responses plus the consensus prompt fit within the primary model's context limit
- **THEN** all responses are included in full

#### Scenario: Responses exceed context window
- **WHEN** the combined responses exceed the primary model's context limit
- **THEN** the system truncates the longest responses proportionally, appending a "[truncated]" marker, so the total fits within the limit

### Requirement: Consensus streaming output
The system SHALL stream the consensus synthesis response to the TUI in real time, just like individual provider responses.

#### Scenario: Consensus streams to display
- **WHEN** the primary model begins generating the consensus synthesis
- **THEN** the TUI displays the synthesis tokens incrementally in the consensus output panel
