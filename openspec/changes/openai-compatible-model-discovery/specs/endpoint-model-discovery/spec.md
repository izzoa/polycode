## ADDED Requirements

### Requirement: Wizard collects base URL before model selection for openai_compatible
For `openai_compatible` providers, the CLI setup wizard SHALL collect the base URL and authentication credentials before the model selection step, enabling endpoint querying.

#### Scenario: Wizard step order for openai_compatible
- **WHEN** provider type is "openai_compatible"
- **THEN** the wizard steps are: type → name → base URL → auth method → API key (if applicable) → model selection → primary

#### Scenario: Other provider types are unaffected
- **WHEN** provider type is "anthropic", "openai", or "google"
- **THEN** the wizard step order remains unchanged (base URL is not collected)

### Requirement: Discover models from endpoint via /v1/models
After collecting base URL and credentials, the wizard SHALL query the OpenAI-compatible `/models` endpoint to discover available models.

#### Scenario: Successful model discovery
- **WHEN** base URL and credentials are provided and the endpoint responds to `GET /models`
- **THEN** the wizard parses the response and extracts model IDs from the `data` array

#### Scenario: Base URL includes /v1 path
- **WHEN** the base URL ends with `/v1` (e.g., `http://localhost:11434/v1`)
- **THEN** the wizard queries `{base_url}/models` (producing `http://localhost:11434/v1/models`)

#### Scenario: Base URL does not include /v1 path
- **WHEN** the base URL does not end with `/v1` and `{base_url}/models` returns a 404
- **THEN** the wizard retries with `{base_url}/v1/models` before falling back

#### Scenario: Discovery includes Bearer auth when API key is provided
- **WHEN** auth method is "api_key" and an API key has been entered
- **THEN** the `/models` request includes an `Authorization: Bearer {key}` header

#### Scenario: Discovery without auth
- **WHEN** auth method is "none"
- **THEN** the `/models` request is sent without an Authorization header

#### Scenario: Discovery request timeout
- **WHEN** the `/models` request does not respond within 10 seconds
- **THEN** the wizard falls back to text input for model selection

#### Scenario: Discovery request fails
- **WHEN** the `/models` request returns an error (network failure, non-200 status, malformed response)
- **THEN** the wizard falls back to text input for model selection with a brief message indicating discovery failed

### Requirement: Show discovered models in a selectable list
Discovered models SHALL be presented in a filterable `huh.Select` list, consistent with the model selection UX for other provider types.

#### Scenario: Models displayed in filterable list
- **WHEN** model discovery returns one or more models
- **THEN** a filterable selectable list is shown with all discovered model IDs

#### Scenario: Custom model escape hatch
- **WHEN** the discovered model list is displayed
- **THEN** a "Custom model..." option is appended at the end of the list, which opens a text input when selected

### Requirement: Cross-reference discovered models with litellm for capabilities
For each discovered model, the system SHALL attempt to find a matching litellm metadata entry to display capability information.

#### Scenario: Discovered model has litellm match
- **WHEN** a discovered model ID matches a litellm entry (via exact, prefixed, or suffix matching)
- **THEN** the model option displays capabilities inline (e.g., `llama3  (8K context | tools)`)

#### Scenario: Discovered model has no litellm match
- **WHEN** a discovered model ID has no matching litellm entry
- **THEN** the model option displays the model ID only, without capabilities

### Requirement: Fallback to text input when no models discovered
When model discovery returns zero models or fails entirely, the wizard SHALL fall back to a text input.

#### Scenario: Empty model list from endpoint
- **WHEN** the `/models` endpoint returns an empty `data` array
- **THEN** the wizard shows a text input with hint text (e.g., "e.g. mistral-large-latest, llama-3-70b")

#### Scenario: Discovery skipped due to failure
- **WHEN** model discovery fails (timeout, network error, auth failure)
- **THEN** the wizard shows a text input with hint text and a message that model discovery was unavailable
