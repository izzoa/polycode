## ADDED Requirements

### Requirement: Three operating modes
The system SHALL support three operating modes — quick, balanced, and thorough — that control how many providers are queried and whether consensus is used.

#### Scenario: Quick mode
- **WHEN** the mode is set to `quick`
- **THEN** only the primary provider is queried, no consensus synthesis occurs, and the response is returned directly

#### Scenario: Balanced mode
- **WHEN** the mode is set to `balanced`
- **THEN** the primary and one secondary provider (chosen by the router) are queried, and consensus synthesis occurs

#### Scenario: Thorough mode
- **WHEN** the mode is set to `thorough`
- **THEN** all healthy providers are queried, consensus synthesis occurs, and the verifier lane runs if configured

### Requirement: /mode command to switch modes
The system SHALL allow users to switch modes mid-session via `/mode <name>`.

#### Scenario: Switch to quick mode
- **WHEN** the user types `/mode quick`
- **THEN** the mode changes to quick and subsequent queries use only the primary provider

#### Scenario: Mode displayed in status bar
- **WHEN** the mode is set
- **THEN** the status bar shows the current mode name
