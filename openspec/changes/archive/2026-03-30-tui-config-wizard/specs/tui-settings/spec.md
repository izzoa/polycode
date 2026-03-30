## ADDED Requirements

### Requirement: Access settings screen via keyboard shortcut
The system SHALL allow the user to open the settings screen from the chat view by pressing `Ctrl+S` or typing `/settings` in the input field.

#### Scenario: Open settings with Ctrl+S
- **WHEN** the user presses `Ctrl+S` while in the chat view and no query is active
- **THEN** the TUI transitions to the settings screen showing a list of all configured providers

#### Scenario: Open settings with /settings command
- **WHEN** the user types `/settings` in the input and presses Enter
- **THEN** the TUI transitions to the settings screen

#### Scenario: Cannot open settings during active query
- **WHEN** the user presses `Ctrl+S` while a query is in progress
- **THEN** the system displays a brief notice "Cannot modify settings during active query" and stays in chat view

### Requirement: Settings screen displays provider list
The system SHALL display all configured providers in a table format showing name, type, model, auth method, primary status, and connection health.

#### Scenario: Provider list with multiple providers
- **WHEN** the settings screen is displayed and 3 providers are configured
- **THEN** all 3 providers are listed with their details in a navigable table

#### Scenario: Empty provider list
- **WHEN** the settings screen is displayed and no providers are configured
- **THEN** the system shows "No providers configured" and prompts to add one

#### Scenario: Navigation in provider list
- **WHEN** the user presses j/↓ or k/↑ in the settings screen
- **THEN** the selection cursor moves between providers

### Requirement: Add provider wizard
The system SHALL provide a step-by-step wizard to add a new provider, triggered by pressing `a` in the settings screen.

#### Scenario: Complete add-provider flow
- **WHEN** the user presses `a` and completes all steps (type, name, auth, API key, model, primary)
- **THEN** the new provider is added to the config, saved to disk, and the provider registry is refreshed

#### Scenario: Provider type determines available fields
- **WHEN** the user selects `openai_compatible` as the provider type
- **THEN** the wizard includes a step for entering the base URL

#### Scenario: Auth method determines credential step
- **WHEN** the user selects `api_key` as the auth method
- **THEN** the wizard shows a masked text input for the API key

#### Scenario: Auth method none skips credential step
- **WHEN** the user selects `none` as the auth method
- **THEN** the wizard skips the credential entry step

#### Scenario: Cancel add wizard
- **WHEN** the user presses `Esc` at any point during the add wizard
- **THEN** the wizard is cancelled and the user returns to the settings list without changes

### Requirement: Edit provider
The system SHALL allow editing an existing provider's settings by pressing `e` while the provider is selected in the settings list.

#### Scenario: Edit provider model
- **WHEN** the user selects a provider, presses `e`, and changes the model field
- **THEN** the updated model is saved to config and the provider is re-initialized

#### Scenario: Change primary designation
- **WHEN** the user edits a provider and sets it as primary
- **THEN** the previous primary provider is un-marked, the new one is marked primary, and the pipeline is rebuilt with the new primary

### Requirement: Remove provider
The system SHALL allow removing a provider by pressing `d` while the provider is selected, with a confirmation prompt.

#### Scenario: Remove non-primary provider
- **WHEN** the user selects a non-primary provider, presses `d`, and confirms
- **THEN** the provider is removed from config, saved to disk, and the registry is refreshed

#### Scenario: Remove primary provider blocked
- **WHEN** the user selects the primary provider and presses `d`
- **THEN** the system shows "Cannot remove the primary provider — designate another provider as primary first"

#### Scenario: Cancel removal
- **WHEN** the user presses `d` then selects "No" on the confirmation
- **THEN** no changes are made

### Requirement: Test provider connection
The system SHALL allow testing a provider's connection by pressing `t` while the provider is selected.

#### Scenario: Successful connection test
- **WHEN** the user selects a provider and presses `t`, and the provider responds
- **THEN** the system displays "Connection successful" with the response time

#### Scenario: Failed connection test
- **WHEN** the user selects a provider and presses `t`, and the provider fails to respond
- **THEN** the system displays the error message (e.g., "401 Unauthorized", "connection refused")

### Requirement: Live config reload after changes
The system SHALL immediately apply configuration changes to the running session after any add, edit, or remove operation — without requiring a restart.

#### Scenario: New provider available after add
- **WHEN** a new provider is added via the wizard and the user returns to chat
- **THEN** the next query fans out to the new provider

#### Scenario: Removed provider excluded after delete
- **WHEN** a provider is removed via settings and the user returns to chat
- **THEN** the next query does not include the removed provider

### Requirement: Return to chat from settings
The system SHALL allow the user to return to the chat view by pressing `Esc` from the settings list.

#### Scenario: Return to chat
- **WHEN** the user presses `Esc` in the settings screen
- **THEN** the TUI transitions back to the chat view with any changes applied
