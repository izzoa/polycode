## ADDED Requirements

### Requirement: Consensus tool calls trigger execution
The system SHALL detect tool calls in the consensus synthesis response and execute them via the tool loop, feeding results back to the primary model until a final text response is produced.

#### Scenario: Consensus includes file_read tool call
- **WHEN** the primary model's consensus response includes a `file_read` tool call
- **THEN** the system reads the file, sends the content back to the primary model, and streams the follow-up response

#### Scenario: Consensus includes file_write tool call
- **WHEN** the primary model's consensus response includes a `file_write` tool call
- **THEN** the system displays the proposed change in the TUI, waits for user confirmation, applies the change if confirmed, and feeds the result back to the primary model

#### Scenario: Consensus includes shell_exec tool call
- **WHEN** the primary model's consensus response includes a `shell_exec` tool call
- **THEN** the system displays the command in the TUI, waits for user confirmation, executes if confirmed, captures output, and feeds the result back to the primary model

#### Scenario: Multiple tool calls in sequence
- **WHEN** the primary model issues tool calls, receives results, and issues more tool calls
- **THEN** the system continues the loop until the model produces a final text response or the iteration limit (10) is reached

### Requirement: TUI confirmation for destructive actions
The system SHALL display a confirmation prompt in the TUI for file writes and shell commands, blocking the tool execution until the user responds.

#### Scenario: User confirms action
- **WHEN** the TUI displays a confirmation prompt and the user presses `y`
- **THEN** the action executes and results are fed back to the model

#### Scenario: User rejects action
- **WHEN** the TUI displays a confirmation prompt and the user presses `n`
- **THEN** the action is skipped and a rejection message is fed back to the model

#### Scenario: Confirmation timeout
- **WHEN** the user does not respond to a confirmation prompt within 5 minutes
- **THEN** the action is cancelled and a timeout error is fed back to the model

### Requirement: Tool results persisted in conversation state
The system SHALL include tool call requests and their results in the conversation message history so the model has full context on subsequent turns.

#### Scenario: Tool result in next turn context
- **WHEN** a file_read tool call executes successfully on turn 1
- **THEN** the file contents are present in the conversation history for turn 2

#### Scenario: Tool results in session file
- **WHEN** a tool call executes and the session is saved
- **THEN** the session file includes the tool call request and its result

#### Scenario: Session resume with tool results
- **WHEN** polycode resumes a session that included tool calls
- **THEN** the conversation state includes the tool call results from the previous session

### Requirement: Deterministic failure handling
The system SHALL handle tool execution errors, provider timeouts, and provider failures with clear, inspectable behavior and no silent data loss.

#### Scenario: Tool execution error
- **WHEN** a shell command fails with a non-zero exit code
- **THEN** the error output is captured and fed back to the model as a tool result, and the conversation continues

#### Scenario: Provider timeout during tool loop
- **WHEN** the primary model times out during a tool-use follow-up query
- **THEN** the system displays an error in the TUI and preserves all conversation state including partial tool results

#### Scenario: Session saved on error
- **WHEN** any error occurs during the execution pipeline
- **THEN** the current conversation state (including any completed tool results) is saved to the session file before the error is displayed
