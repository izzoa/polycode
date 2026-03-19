## ADDED Requirements

### Requirement: Tool loop streams follow-up responses live
`ToolLoop.Run` SHALL relay response chunks from the model as they arrive instead of buffering the entire response.

#### Scenario: Follow-up response streams to TUI
- **WHEN** the model responds after tool results are sent
- **THEN** each response chunk is sent to the TUI as a `ConsensusChunkMsg` immediately

#### Scenario: Multi-iteration tool loop streams each response
- **WHEN** the tool loop runs multiple iterations (model requests more tools)
- **THEN** each iteration's follow-up response streams live, with tool execution messages interleaved

### Requirement: Tool execution output shown in TUI
After each tool execution, the command and its output SHALL be displayed in the consensus stream.

#### Scenario: Shell command output displayed
- **WHEN** a `shell_exec` tool call completes
- **THEN** the consensus stream shows the command and its truncated output (first 500 characters)

#### Scenario: File read output displayed
- **WHEN** a `file_read` tool call completes
- **THEN** the consensus stream shows the file path and truncated content

#### Scenario: Tool error displayed
- **WHEN** a tool call fails
- **THEN** the consensus stream shows the error message

### Requirement: Tool loop has separate timeout
The tool loop SHALL have its own timeout context (5 minutes) independent of the query pipeline timeout.

#### Scenario: Query timeout does not kill tool loop
- **WHEN** fan-out and consensus synthesis consume most of the query timeout
- **THEN** the tool loop still has its full 5-minute budget to execute tools and get follow-up responses

#### Scenario: Individual commands respect executor timeout
- **WHEN** a single shell command runs during tool execution
- **THEN** it still has the per-command 120-second timeout from the executor
