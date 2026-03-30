## ADDED Requirements

### Requirement: HTTP transport handles SSE responses
When an MCP server responds with `Content-Type: text/event-stream`, the HTTP transport SHALL parse SSE frames and extract the JSON-RPC response from `data:` lines.

#### Scenario: Server responds with plain JSON
- **WHEN** the response Content-Type is `application/json`
- **THEN** the response body is parsed as a single JSON-RPC response (existing behavior)

#### Scenario: Server responds with SSE stream
- **WHEN** the response Content-Type is `text/event-stream`
- **THEN** the transport reads `data:` lines from the stream, parses the JSON-RPC response from the last complete data frame, and returns the result

#### Scenario: SSE stream contains multiple events
- **WHEN** the SSE stream contains multiple `data:` events
- **THEN** only the final JSON-RPC response with a matching ID is returned to the caller

### Requirement: SSE parsing handles edge cases
The SSE parser SHALL handle blank lines between events, `event:` type prefixes, and `id:` fields without errors.

#### Scenario: SSE event with event type prefix
- **WHEN** an SSE frame includes `event: message` before `data:`
- **THEN** the data line is parsed correctly regardless of the event type

#### Scenario: SSE stream with blank line separators
- **WHEN** SSE events are separated by blank lines (per SSE spec)
- **THEN** each event is parsed independently
