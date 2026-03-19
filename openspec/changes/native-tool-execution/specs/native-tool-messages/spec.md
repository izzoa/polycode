## ADDED Requirements

### Requirement: Message type supports tool call fields
`provider.Message` SHALL include optional `ToolCalls` and `ToolCallID` fields alongside the existing `Role` and `Content`.

#### Scenario: Assistant message with tool calls
- **WHEN** the model responds with tool call requests
- **THEN** the assistant Message has `ToolCalls` populated with the tool call ID, name, and arguments

#### Scenario: Tool result message
- **WHEN** a tool has been executed and its output is available
- **THEN** a Message with `Role: "tool"`, `Content: <output>`, and `ToolCallID: <call_id>` is created

#### Scenario: Regular messages unaffected
- **WHEN** a message has no tool calls or tool results
- **THEN** `ToolCalls` is nil/empty and `ToolCallID` is empty string (zero values)

### Requirement: RoleTool role constant exists
A `RoleTool` constant with value `"tool"` SHALL be defined alongside the existing RoleUser, RoleAssistant, RoleSystem.

### Requirement: Provider adapters serialize tool messages correctly
OpenAI and OpenAI-compatible provider adapters SHALL serialize `ToolCalls` on assistant messages and accept `tool` role messages with `tool_call_id` in API requests.

#### Scenario: OpenAI-compatible request with tool continuation
- **WHEN** the message list includes an assistant message with ToolCalls followed by tool result messages
- **THEN** the API request body encodes `tool_calls` on the assistant message and `role: "tool"` with `tool_call_id` on result messages

#### Scenario: API request without tool messages
- **WHEN** no messages have ToolCalls or ToolCallID
- **THEN** the API request body is identical to the current format (no regression)

### Requirement: Tool loop uses native message format
`ToolLoop.Run` SHALL append assistant messages with `ToolCalls` and tool result messages with `RoleTool` + `ToolCallID` instead of fake user messages.

#### Scenario: Tool result sent as native tool message
- **WHEN** a tool call is executed and the result is collected
- **THEN** the message appended has `Role: RoleTool`, `ToolCallID: <call_id>`, and `Content: <output>`

#### Scenario: Assistant tool call message preserved
- **WHEN** the model emits tool calls
- **THEN** an assistant Message with the original `ToolCalls` is appended before the tool results

### Requirement: Tool loop receives correct conversation context
The tool loop SHALL receive the conversation that includes the synthesis turn that produced the tool calls, not the raw chat history.

#### Scenario: Synthesis context passed to tool loop
- **WHEN** the consensus model emits tool calls during synthesis
- **THEN** the tool loop's initial message list includes the synthesis prompt context and the assistant message with its ToolCalls

#### Scenario: Tool loop does not use raw chat history
- **WHEN** tool execution begins
- **THEN** `conv.snapshot()` is NOT passed directly as the tool loop's conversation
