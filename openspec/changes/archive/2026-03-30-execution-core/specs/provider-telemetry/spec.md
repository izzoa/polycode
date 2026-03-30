## ADDED Requirements

### Requirement: Log per-provider telemetry events
The system SHALL log telemetry events to a local JSONL file at `~/.config/polycode/telemetry.jsonl` for every provider query, consensus synthesis, and tool execution.

#### Scenario: Provider query logged
- **WHEN** a provider completes a query (success or failure)
- **THEN** the system appends a JSONL event with: timestamp, provider_id, event_type ("provider_response"), latency_ms, input_tokens, output_tokens, error (if any)

#### Scenario: Consensus synthesis logged
- **WHEN** the consensus synthesis completes
- **THEN** the system appends a JSONL event with: timestamp, provider_id (primary), event_type ("consensus_complete"), latency_ms, input_tokens, output_tokens

#### Scenario: Tool execution logged
- **WHEN** a tool call executes
- **THEN** the system appends a JSONL event with: timestamp, event_type ("tool_executed"), tool_name, duration_ms, success, error (if any)

### Requirement: Telemetry file rotation
The system SHALL rotate or truncate the telemetry file if it exceeds 10MB on startup to prevent unbounded growth.

#### Scenario: File under limit
- **WHEN** polycode starts and the telemetry file is under 10MB
- **THEN** the system appends to the existing file

#### Scenario: File over limit
- **WHEN** polycode starts and the telemetry file exceeds 10MB
- **THEN** the system truncates the file to the most recent 5MB of events and continues appending
