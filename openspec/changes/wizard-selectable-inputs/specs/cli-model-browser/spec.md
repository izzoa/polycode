## ADDED Requirements

### Requirement: Model selection uses filterable interactive list
The CLI wizard SHALL present available models as a filterable, arrow-key navigable select list pre-populated from litellm metadata for the chosen provider type.

#### Scenario: Models loaded from litellm for anthropic
- **WHEN** provider type is "anthropic" and the wizard reaches the model step
- **THEN** a selectable list is populated with Anthropic models from litellm metadata, sorted by priority (sonnet, opus, haiku first)

#### Scenario: Models loaded from litellm for openai
- **WHEN** provider type is "openai" and the wizard reaches the model step
- **THEN** a selectable list is populated with OpenAI models from litellm metadata, sorted by priority (gpt-4o, gpt-4-turbo first)

#### Scenario: Models loaded from litellm for google
- **WHEN** provider type is "google" and the wizard reaches the model step
- **THEN** a selectable list is populated with Google models from litellm metadata, sorted by priority (gemini-2.5-pro, gemini-2.5-flash first)

### Requirement: Model list shows capabilities inline
Each model option in the select list SHALL display the model name followed by its capabilities summary.

#### Scenario: Model with capabilities displayed
- **WHEN** a model has capabilities data (context window, tools, vision, reasoning)
- **THEN** the option displays as `model-name  (128K context | tools | vision)` using the existing FormatCapabilities format

#### Scenario: Model without capabilities data
- **WHEN** a model has no capabilities data from litellm
- **THEN** the option displays the model name only without a capabilities suffix

### Requirement: Default model is pre-selected
The select list SHALL pre-select the default model for the provider type (as defined by `DefaultModelByType`).

#### Scenario: Default model exists in list
- **WHEN** the model list contains the default model for the provider type
- **THEN** that model SHALL be highlighted/selected by default

#### Scenario: Default model not in list
- **WHEN** the model list does not contain the default model
- **THEN** the first model in the list SHALL be highlighted by default

### Requirement: Type-to-filter narrows model list
The user SHALL be able to type characters to filter the model list to matching entries.

#### Scenario: User filters by typing
- **WHEN** the model list is displayed and the user types "sonnet"
- **THEN** only models containing "sonnet" in their name are shown

### Requirement: Custom model escape hatch
The model select list SHALL include a "Custom model..." option at the end that allows the user to enter an arbitrary model name.

#### Scenario: User selects custom model
- **WHEN** the user selects "Custom model..." from the list
- **THEN** a text input is displayed where the user can type any model name

#### Scenario: Custom model with default hint
- **WHEN** the user selects "Custom model..." and the provider has a default model
- **THEN** the text input shows the default model as placeholder text

### Requirement: Fallback to text input when models unavailable
When litellm metadata is unavailable or returns no models for the provider type, the wizard SHALL fall back to a text input with model name hints.

#### Scenario: No litellm data available
- **WHEN** litellm metadata fetch fails and no cache exists
- **THEN** a text input is displayed with a provider-specific hint (e.g., "e.g. claude-sonnet-4-20250514")

#### Scenario: No models for openai_compatible
- **WHEN** provider type is "openai_compatible" (which has no standard litellm prefix)
- **THEN** a text input is displayed with a hint (e.g., "e.g. mistral-large-latest, llama-3-70b")
