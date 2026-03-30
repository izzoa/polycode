## ADDED Requirements

### Requirement: Tool name collisions are detected during discovery
When building the `toolIndex` during `Connect()` or `reconnectServer()`, if a prefixed tool name already exists from a *different* server, the system SHALL log a warning and skip the duplicate tool to prevent silent tool shadowing.

#### Scenario: Two servers produce the same prefixed name
- **WHEN** server "my_db" exposes tool "read" and server "my" exposes tool "db_read" (both produce "mcp_my_db_read")
- **THEN** the first server's tool is kept, the second is skipped, and a warning is logged

#### Scenario: Same server reconnect does not trigger collision
- **WHEN** a server is reconnected and re-discovers the same tools
- **THEN** no collision warning is logged (old entries were removed before re-adding)
