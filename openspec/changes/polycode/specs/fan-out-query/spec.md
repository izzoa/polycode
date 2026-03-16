## ADDED Requirements

### Requirement: Parallel query dispatch
The system SHALL send the user's prompt to all configured and healthy providers simultaneously using concurrent async tasks.

#### Scenario: Query sent to all providers
- **WHEN** the user submits a prompt and 3 providers are configured and healthy
- **THEN** the system dispatches the prompt to all 3 providers concurrently, not sequentially

#### Scenario: Unhealthy provider excluded
- **WHEN** a provider was marked unhealthy at startup or failed during a previous query
- **THEN** the system skips that provider and dispatches only to healthy providers

### Requirement: Streaming response collection
The system SHALL collect streaming responses from each provider as they arrive, making partial results available to the TUI in real time.

#### Scenario: Streaming tokens displayed
- **WHEN** a provider streams back tokens
- **THEN** the TUI displays the tokens incrementally in that provider's response panel

#### Scenario: Provider completes response
- **WHEN** a provider finishes streaming its full response
- **THEN** the system marks that provider as complete and its full response is available for consensus

### Requirement: Configurable response timeout
The system SHALL enforce a configurable timeout (default 60 seconds) for provider responses. Providers that exceed the timeout are excluded from consensus.

#### Scenario: Provider responds within timeout
- **WHEN** a provider completes its response within the configured timeout
- **THEN** the response is included in the consensus input

#### Scenario: Provider exceeds timeout
- **WHEN** a provider has not completed its response within the configured timeout
- **THEN** the system cancels the request to that provider, marks it as timed out, and proceeds with available responses

#### Scenario: Custom timeout configured
- **WHEN** the config specifies `consensus.timeout: 120s`
- **THEN** the system waits up to 120 seconds for each provider before timing out

### Requirement: Minimum response threshold
The system SHALL require a configurable minimum number of responses (default 2) before initiating consensus synthesis.

#### Scenario: Minimum responses met
- **WHEN** at least `min_responses` providers have completed their responses
- **THEN** the system proceeds to consensus synthesis (after timeout or all providers complete)

#### Scenario: Minimum responses not met
- **WHEN** fewer than `min_responses` providers complete their responses (due to failures/timeouts)
- **THEN** the system falls back to using whatever responses are available, with a warning to the user. If only the primary responded, its response is used directly without consensus.

### Requirement: Error isolation per provider
The system SHALL isolate failures so that one provider's error does not affect other providers or the overall query.

#### Scenario: One provider returns an error
- **WHEN** one provider returns an API error (rate limit, auth failure, server error) while others succeed
- **THEN** the failed provider is excluded from consensus and the user sees a warning, but the query continues with remaining providers

#### Scenario: All non-primary providers fail
- **WHEN** all providers except the primary fail
- **THEN** the system uses the primary's direct response without consensus synthesis and informs the user
