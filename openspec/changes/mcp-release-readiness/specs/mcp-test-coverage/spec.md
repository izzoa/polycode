## ADDED Requirements

### Requirement: Reconfigure has unit tests
The `Reconfigure()` method SHALL have tests covering: adding a new server, removing a server, reconnecting a changed server, and the `mcpConfigChanged()` helper detecting ReadOnly/Timeout changes.

#### Scenario: Reconfigure adds new server
- **WHEN** Reconfigure is called with a config containing a new server name
- **THEN** Status() includes the new server

#### Scenario: Reconfigure removes server
- **WHEN** Reconfigure is called without a previously-configured server
- **THEN** Status() no longer includes the removed server and its tools/resources/prompts are gone

#### Scenario: mcpConfigChanged detects all fields
- **WHEN** two configs differ only in ReadOnly or Timeout
- **THEN** mcpConfigChanged returns true

### Requirement: TestConnection has unit tests
The standalone `TestConnection()` function SHALL have a test that validates it against a mock server.

#### Scenario: TestConnection succeeds against mock
- **WHEN** called with a valid MCPServerConfig pointing to a mock server
- **THEN** returns the correct tool count and nil error

### Requirement: parseSSEResponse has unit tests
The SSE parser SHALL have tests covering: single-line events, multi-line data events, blank line separators, event/id/comment prefixes, and missing trailing blank line.

#### Scenario: Single data line event
- **WHEN** an SSE stream has one `data:` line followed by a blank line
- **THEN** the JSON-RPC result is correctly extracted

#### Scenario: Multi-line data event
- **WHEN** an SSE stream has multiple `data:` lines before a blank separator
- **THEN** the lines are joined with newline and parsed as one JSON payload

### Requirement: HTTP transport has unit tests
`httpConn.sendRequest()` SHALL have tests covering: successful JSON response, SSE response, and HTTP error responses.

#### Scenario: HTTP returns JSON
- **WHEN** a mock HTTP server responds with application/json
- **THEN** the JSON-RPC result is correctly parsed

#### Scenario: HTTP returns SSE
- **WHEN** a mock HTTP server responds with text/event-stream
- **THEN** the SSE parser is used and the result is correctly extracted

### Requirement: Multiplexed reader has unit tests
The `startReader()` goroutine and multiplexed request/response routing SHALL have a test exercising concurrent requests.

#### Scenario: Concurrent requests routed correctly
- **WHEN** two requests are sent concurrently to the same server
- **THEN** each receives the correct response by ID

### Requirement: Resource and prompt discovery have unit tests
`discoverResources()` and `discoverPrompts()` SHALL have tests against a mock server that returns resources/prompts.

#### Scenario: Resources discovered from mock
- **WHEN** discoverResources is called on a mock server returning 2 resources
- **THEN** 2 MCPResource entries are returned with correct fields

#### Scenario: Prompts discovered from mock
- **WHEN** discoverPrompts is called on a mock server returning 1 prompt with arguments
- **THEN** 1 MCPPrompt entry is returned with correct arguments
