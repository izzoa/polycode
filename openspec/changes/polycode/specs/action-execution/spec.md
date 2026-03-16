## ADDED Requirements

### Requirement: File read operations
The system SHALL allow the consensus output to trigger file read operations, displaying file contents in the TUI when the primary model's tool-use output requests it.

#### Scenario: Consensus requests file read
- **WHEN** the consensus output includes a tool call to read a file
- **THEN** the system reads the file from disk and makes its contents available in the conversation context

#### Scenario: File not found
- **WHEN** a file read is requested for a non-existent path
- **THEN** the system reports the error to the primary model for the next interaction turn

### Requirement: File write operations
The system SHALL allow the consensus output to create or modify files, with user confirmation before applying changes.

#### Scenario: Consensus proposes file edit
- **WHEN** the consensus output includes a tool call to write or edit a file
- **THEN** the system displays a diff of proposed changes and waits for user confirmation before applying

#### Scenario: User approves file change
- **WHEN** the user confirms a proposed file change (presses 'y' or Enter)
- **THEN** the system writes the change to disk and reports success

#### Scenario: User rejects file change
- **WHEN** the user rejects a proposed file change (presses 'n')
- **THEN** the system does not modify the file and informs the primary model that the change was rejected

### Requirement: Shell command execution
The system SHALL allow the consensus output to execute shell commands, with user confirmation before execution.

#### Scenario: Consensus proposes shell command
- **WHEN** the consensus output includes a tool call to run a shell command
- **THEN** the system displays the command and waits for user confirmation before executing

#### Scenario: Command execution with output
- **WHEN** a confirmed shell command is executed
- **THEN** the system captures stdout and stderr and makes them available in the conversation context

#### Scenario: Command execution timeout
- **WHEN** a shell command runs longer than 120 seconds (configurable)
- **THEN** the system terminates the command and reports a timeout error

### Requirement: Tool-use schema
The system SHALL define a set of tools (file_read, file_write, shell_exec) in the format expected by the primary model's API, and include them in the consensus synthesis request so the primary model can invoke them.

#### Scenario: Tools included in consensus request
- **WHEN** the consensus synthesis request is sent to the primary model
- **THEN** the request includes tool definitions for file_read, file_write, and shell_exec

#### Scenario: Primary model calls a tool
- **WHEN** the primary model's consensus response includes a tool call
- **THEN** the system executes the tool (with confirmation where required) and feeds the result back to the primary model

### Requirement: Action safety guardrails
The system SHALL never execute destructive or irreversible actions without explicit user confirmation. Destructive actions include: deleting files, running commands with `rm`, `sudo`, or commands that modify system state.

#### Scenario: Destructive command requires confirmation
- **WHEN** the consensus output proposes a command identified as destructive
- **THEN** the system highlights the risk level and requires explicit user confirmation

#### Scenario: Non-destructive read operations skip confirmation
- **WHEN** the consensus output requests a file read operation
- **THEN** the system executes it without user confirmation since reads are non-destructive
