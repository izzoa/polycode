## ADDED Requirements

### Requirement: Lifecycle hooks execute shell commands
The system SHALL execute user-defined shell commands at lifecycle events: pre_query, post_query, post_tool, on_error.

#### Scenario: Pre-query hook runs
- **WHEN** a pre_query hook is configured and the user submits a prompt
- **THEN** the hook command runs before the query is dispatched

#### Scenario: On-error hook with template variable
- **WHEN** an on_error hook is configured with `{{.Error}}` and an error occurs
- **THEN** the hook command runs with the error message substituted

#### Scenario: Hook failure does not block pipeline
- **WHEN** a hook command exits with non-zero status
- **THEN** the system logs a warning but continues the pipeline
