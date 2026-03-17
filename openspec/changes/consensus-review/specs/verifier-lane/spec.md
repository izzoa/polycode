## ADDED Requirements

### Requirement: Post-consensus verification
The system SHALL optionally run a verification command (tests, lint) after consensus proposes a code change, and include the verification result in the output.

#### Scenario: Verification enabled and passes
- **WHEN** `consensus.verify: true` is set in config and the consensus produces a file change that is applied
- **THEN** the system runs the configured verify command, and on success displays "Verification passed" in the TUI

#### Scenario: Verification enabled and fails
- **WHEN** the verify command exits with a non-zero status
- **THEN** the system displays the failure output in the TUI and feeds it back to the primary model for a follow-up response

#### Scenario: Verification disabled
- **WHEN** `consensus.verify` is not set or is false
- **THEN** no verification step is run after consensus

#### Scenario: Auto-detected verify command
- **WHEN** `consensus.verify: true` is set but no `verify_command` is specified
- **THEN** the system auto-detects the verify command based on project files (go.mod → `go test ./...`, package.json → `npm test`, Makefile → `make test`)
