## ADDED Requirements

### Requirement: Provider configuration via YAML
The system SHALL support defining LLM providers in a YAML configuration file located at `~/.config/polycode/config.yaml` (following XDG base directory conventions). Each provider entry SHALL include a name, type, authentication method, and model identifier.

#### Scenario: Valid configuration with multiple providers
- **WHEN** the config file contains entries for Anthropic, OpenAI, Gemini, and a custom OpenAI-compatible provider
- **THEN** the system loads all four providers and makes them available for querying

#### Scenario: Missing configuration file
- **WHEN** no config file exists at the expected path
- **THEN** the system launches an interactive setup wizard that guides the user through configuring at least one provider

#### Scenario: Invalid provider configuration
- **WHEN** a provider entry is missing required fields (name, type, or model)
- **THEN** the system reports a clear validation error identifying the malformed entry and refuses to start

### Requirement: Supported provider types
The system SHALL support the following provider types: `anthropic` (Anthropic Claude API), `openai` (OpenAI API), `google` (Google Gemini API), and `openai_compatible` (any OpenAI-compatible endpoint with a user-defined base URL).

#### Scenario: Anthropic provider type
- **WHEN** a provider is configured with `type: anthropic`
- **THEN** the system uses the Anthropic Claude API for queries to that provider

#### Scenario: OpenAI provider type
- **WHEN** a provider is configured with `type: openai`
- **THEN** the system uses the OpenAI API for queries to that provider

#### Scenario: Google provider type
- **WHEN** a provider is configured with `type: google`
- **THEN** the system uses the Google Gemini API for queries to that provider

#### Scenario: Custom OpenAI-compatible endpoint
- **WHEN** a provider is configured with `type: openai_compatible` and a `base_url` field
- **THEN** the system sends OpenAI-compatible API requests to the specified base URL

### Requirement: Primary model designation
The system SHALL allow exactly one provider to be designated as `primary: true` in the configuration. The primary provider is used for consensus synthesis.

#### Scenario: One provider marked primary
- **WHEN** exactly one provider has `primary: true` in the config
- **THEN** that provider is used as the consensus synthesizer

#### Scenario: No provider marked primary
- **WHEN** no provider has `primary: true`
- **THEN** the system reports an error and refuses to start, instructing the user to designate a primary provider

#### Scenario: Multiple providers marked primary
- **WHEN** more than one provider has `primary: true`
- **THEN** the system reports an error identifying the conflicting entries and refuses to start

### Requirement: Provider health validation
The system SHALL validate that each configured provider is reachable and properly authenticated at startup.

#### Scenario: All providers healthy
- **WHEN** all configured providers respond to a validation check
- **THEN** the system starts normally and displays the list of available providers

#### Scenario: Provider unreachable at startup
- **WHEN** a non-primary provider fails validation
- **THEN** the system starts with a warning indicating that provider is unavailable, and excludes it from queries

#### Scenario: Primary provider unreachable
- **WHEN** the primary provider fails validation
- **THEN** the system reports an error and refuses to start, since consensus synthesis requires the primary
