## ADDED Requirements

### Requirement: Provider type selection uses interactive list
The CLI setup wizard SHALL present provider types (anthropic, openai, google, openai_compatible) as an arrow-key navigable select list instead of a text input field.

#### Scenario: User selects provider type with arrow keys
- **WHEN** the wizard reaches the provider type step
- **THEN** a selectable list of provider types is displayed, navigable with up/down arrow keys, and confirmed with Enter

#### Scenario: Default provider type is pre-selected
- **WHEN** the provider type list is displayed
- **THEN** the first option (anthropic) SHALL be highlighted by default

### Requirement: Auth method selection uses interactive list
The CLI setup wizard SHALL present auth methods as an arrow-key navigable select list, filtered to only show methods valid for the chosen provider type.

#### Scenario: User selects auth method for anthropic
- **WHEN** provider type is "anthropic" and the wizard reaches the auth method step
- **THEN** a selectable list shows only "api_key" and "oauth" as options

#### Scenario: User selects auth method for openai_compatible
- **WHEN** provider type is "openai_compatible" and the wizard reaches the auth method step
- **THEN** a selectable list shows only "api_key" and "none" as options

#### Scenario: Default auth method is pre-selected
- **WHEN** the auth method list is displayed
- **THEN** the first valid auth method for the provider type SHALL be highlighted by default

### Requirement: Primary provider toggle uses interactive list
When adding a provider and other providers already exist in the config, the wizard SHALL present a yes/no selector for "Set as primary?" instead of requiring text input.

#### Scenario: First provider is always primary
- **WHEN** no other providers exist in the config
- **THEN** the primary step SHALL be skipped and the provider is automatically set as primary

#### Scenario: User chooses primary with selector
- **WHEN** other providers exist and the wizard reaches the primary step
- **THEN** a selectable list shows "Yes" and "No" options

### Requirement: Free-form fields remain text inputs
The wizard SHALL continue to use text input (not selectable lists) for fields that accept arbitrary user values.

#### Scenario: Provider name is a text input
- **WHEN** the wizard reaches the provider name step
- **THEN** a text input field is displayed with placeholder text (e.g., "claude, gpt4")

#### Scenario: API key is a text input
- **WHEN** the wizard reaches the API key step
- **THEN** a password-masked text input field is displayed

#### Scenario: Base URL is a text input
- **WHEN** provider type is "openai_compatible" and the wizard reaches the base URL step
- **THEN** a text input field is displayed with placeholder text
