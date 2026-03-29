## ADDED Requirements

### Requirement: Session picker displays branch hierarchy
The session picker SHALL display branch sessions indented under their parent session with tree connector characters.

#### Scenario: Branch shown under parent
- **WHEN** session "Debug auth" has two branches "Debug auth (branch 1)" and "Try OAuth flow"
- **THEN** the picker SHALL show "Debug auth" at root level with both branches indented below it

#### Scenario: Root sessions sorted by recency
- **WHEN** the session picker is opened
- **THEN** root sessions SHALL be sorted by most recently accessed AND branches SHALL be sorted by creation time under their parent

### Requirement: Branch indentation capped at 3 levels
The session picker SHALL indent branches up to 3 levels deep. Branches deeper than 3 levels SHALL display at level 3 with a depth indicator.

#### Scenario: Deep nesting display
- **WHEN** a branch chain is 5 levels deep
- **THEN** levels 4 and 5 SHALL display at indentation level 3 with a depth prefix like "[+2]"

### Requirement: Branch count shown on parent
Parent sessions with branches SHALL display a branch count indicator.

#### Scenario: Parent shows branch count
- **WHEN** a session has 3 branches
- **THEN** the session picker entry SHALL include an indicator like "(3 branches)"
