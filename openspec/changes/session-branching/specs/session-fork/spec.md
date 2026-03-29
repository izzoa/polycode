## ADDED Requirements

### Requirement: /branch creates a forked session
The system SHALL support a `/branch` command that creates a new session forked from the current session at the current exchange index.

#### Scenario: Basic branch creation
- **WHEN** the user types `/branch` during exchange index 3
- **THEN** a new session SHALL be created containing exchanges 0-3 from the current session AND the user SHALL be switched to the new session

#### Scenario: Branch with custom name
- **WHEN** the user types `/branch Try different approach`
- **THEN** the new session SHALL be named "Try different approach"

#### Scenario: Branch with auto-generated name
- **WHEN** the user types `/branch` and the parent session is named "Debug auth"
- **THEN** the new session SHALL be named "Debug auth (branch 1)" or similar

### Requirement: Branch metadata tracks parent relationship
The branch session SHALL store the parent session ID and the exchange index at which it was branched.

#### Scenario: Branch metadata is set
- **WHEN** a branch is created from session "abc123" at exchange index 5
- **THEN** the new session metadata SHALL have ParentSessionID="abc123" and BranchExchangeIndex=5

### Requirement: Branch sessions are independent after fork
Changes to a branch session SHALL NOT affect the parent session, and vice versa.

#### Scenario: Independent history after fork
- **WHEN** a branch is created and the user sends a new query in the branch
- **THEN** the parent session SHALL NOT contain the new exchange

### Requirement: Deleting parent promotes children
When a parent session is deleted, its child branches SHALL be promoted to root level.

#### Scenario: Parent deletion promotes branches
- **WHEN** a parent session with 2 branches is deleted
- **THEN** both branch sessions SHALL have their ParentSessionID cleared and appear as root sessions
