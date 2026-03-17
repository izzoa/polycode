## ADDED Requirements

### Requirement: Auth methods filtered by provider type
The wizard SHALL show only auth methods valid for the selected provider type.

#### Scenario: Anthropic auth options
- **WHEN** the user selects `anthropic` as the provider type
- **THEN** the wizard shows `api_key` and `oauth` as auth options (not `none`)

#### Scenario: OpenAI-compatible auth options
- **WHEN** the user selects `openai_compatible` as the provider type
- **THEN** the wizard shows `api_key` and `none` as auth options

### Requirement: Model selection from litellm metadata
The wizard SHALL present available models for the selected provider type, sourced from the litellm metadata store.

#### Scenario: Models listed for anthropic
- **WHEN** the user selects `anthropic` and the litellm metadata is available
- **THEN** the wizard shows a list of Anthropic models with their context window and capabilities

#### Scenario: Manual model entry
- **WHEN** the user selects the "custom" option in the model list
- **THEN** the wizard shows a text input for entering a model name manually

#### Scenario: Litellm data unavailable
- **WHEN** the litellm metadata is not cached and the network is unavailable
- **THEN** the wizard falls back to a text input with a hardcoded default model suggestion

### Requirement: Model capability display
The wizard SHALL display model metadata (context window, function calling, vision, reasoning) alongside each model in the selection list.

#### Scenario: Model with all capabilities
- **WHEN** a model supports function calling, vision, and reasoning
- **THEN** the wizard displays badges like `200K context | tools | vision | reasoning`

#### Scenario: Model with limited capabilities
- **WHEN** a model only supports function calling
- **THEN** the wizard displays only `128K context | tools`

### Requirement: Connection validation in wizard
The wizard SHALL test the provider connection after credentials are entered and show the result before saving.

#### Scenario: Connection succeeds
- **WHEN** the test query returns a response
- **THEN** the wizard shows "✓ Connected successfully" and proceeds to save

#### Scenario: Connection fails
- **WHEN** the test query fails (auth error, network error)
- **THEN** the wizard shows the error and offers to re-enter credentials or skip validation
