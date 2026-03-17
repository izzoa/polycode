## ADDED Requirements

### Requirement: Three-tier instruction hierarchy
The system SHALL load instructions from repo-level, user-level, and built-in sources, concatenating them in precedence order into the system prompt.

#### Scenario: Repo-level instructions exist
- **WHEN** `.polycode/instructions.md` exists in the current working directory
- **THEN** its content is prepended to the system prompt (highest precedence)

#### Scenario: User-level instructions exist
- **WHEN** `~/.config/polycode/instructions.md` exists
- **THEN** its content is included after repo-level instructions

#### Scenario: No custom instructions
- **WHEN** neither repo-level nor user-level instruction files exist
- **THEN** only the built-in default system prompt is used

#### Scenario: All three tiers present
- **WHEN** repo-level, user-level, and built-in instructions all exist
- **THEN** the system prompt contains all three, in order: repo > user > built-in
