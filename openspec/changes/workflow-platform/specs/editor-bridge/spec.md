## ADDED Requirements

### Requirement: HTTP server for editor integration
The system SHALL provide a `polycode serve` command that starts an HTTP server for editor integration.

#### Scenario: POST /prompt
- **WHEN** an editor sends POST /prompt with a JSON body containing a prompt
- **THEN** the server runs the consensus pipeline and returns the response as JSON

#### Scenario: POST /review
- **WHEN** an editor sends POST /review with a diff in the body
- **THEN** the server runs a structured review and returns the findings as JSON

#### Scenario: GET /status
- **WHEN** a client sends GET /status
- **THEN** the server returns provider health, current mode, and token usage
