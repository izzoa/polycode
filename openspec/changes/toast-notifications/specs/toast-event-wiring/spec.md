## ADDED Requirements

### Requirement: Config save triggers success toast
The system SHALL emit a Success toast with message "Config saved" when a configuration change is persisted to disk.

#### Scenario: Config save notification
- **WHEN** the user saves config changes (settings or MCP wizard)
- **THEN** a Success toast with text "Config saved" SHALL appear

### Requirement: MCP reconnect triggers info toast
The system SHALL emit an Info toast when an MCP server reconnects successfully, including the server name.

#### Scenario: MCP reconnection notification
- **WHEN** an MCP server named "filesystem" reconnects successfully
- **THEN** an Info toast with text containing "filesystem" and "connected" SHALL appear

### Requirement: Clipboard copy triggers success toast
The system SHALL emit a Success toast with message "Copied to clipboard" when content is copied to the system clipboard.

#### Scenario: Clipboard copy notification
- **WHEN** the user copies content via the copy keybinding
- **THEN** a Success toast with text "Copied to clipboard" SHALL appear

### Requirement: Pipeline errors trigger error toast
The system SHALL emit an Error toast when a provider or consensus error occurs, showing a truncated error message.

#### Scenario: Provider error notification
- **WHEN** a provider returns an error during query execution
- **THEN** an Error toast SHALL appear with the error message (truncated to 80 characters if longer)
