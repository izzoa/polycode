## ADDED Requirements

### Requirement: mcpClient access is synchronized
All reads and writes of the `mcpClient` variable in `app.go` SHALL be protected by a `sync.RWMutex` to prevent data races between query goroutines and the config-change handler.

#### Scenario: Config change writes mcpClient while query reads it
- **WHEN** the config-change handler creates a new MCPClient in a goroutine AND a query handler reads mcpClient concurrently
- **THEN** no data race occurs (verified by go test -race)

#### Scenario: Nil check + method call is atomic
- **WHEN** a goroutine checks `mcpClient != nil` and then calls a method on it
- **THEN** the read lock is held across both operations so mcpClient cannot become nil between the check and the call

#### Scenario: Multiple concurrent readers
- **WHEN** multiple query goroutines read mcpClient simultaneously
- **THEN** they proceed concurrently without blocking each other (RLock, not Lock)
