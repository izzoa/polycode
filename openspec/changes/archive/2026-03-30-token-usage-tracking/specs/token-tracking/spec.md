## ADDED Requirements

### Requirement: Provider adapters report token usage
Each provider adapter SHALL extract input and output token counts from the API response and include them in the final `StreamChunk` (where `Done` is true).

#### Scenario: Anthropic reports usage
- **WHEN** the Anthropic adapter receives a `message_delta` SSE event with `usage.input_tokens` and `usage.output_tokens`
- **THEN** the final StreamChunk includes the reported input and output token counts

#### Scenario: OpenAI reports usage
- **WHEN** the OpenAI adapter receives a streaming response with a `usage` field in the final chunk
- **THEN** the final StreamChunk includes the reported input and output token counts

#### Scenario: Gemini reports usage
- **WHEN** the Gemini adapter receives a response with `usageMetadata.promptTokenCount` and `usageMetadata.candidatesTokenCount`
- **THEN** the final StreamChunk includes the reported input and output token counts

#### Scenario: Provider does not report usage
- **WHEN** a provider (e.g., a custom OpenAI-compatible endpoint) does not include usage data in its response
- **THEN** the final StreamChunk reports zero for both input and output tokens

### Requirement: Session-wide token accumulation
The system SHALL maintain a running total of input tokens and output tokens per provider across all turns in the current session.

#### Scenario: Tokens accumulate across turns
- **WHEN** a provider reports 500 input tokens on turn 1 and 800 input tokens on turn 2
- **THEN** the session total for that provider shows 1300 input tokens

#### Scenario: Consensus synthesis tracked separately
- **WHEN** the primary provider is queried once for fan-out and once for consensus synthesis
- **THEN** both calls' token usage is accumulated into the primary provider's session total

### Requirement: Known model context window limits
The system SHALL maintain a registry of known model context window limits. Users SHALL be able to override the limit for any provider via the `max_context` field in the config file.

#### Scenario: Known model uses built-in limit
- **WHEN** a provider is configured with model `gpt-4o` and no `max_context` override
- **THEN** the system uses the built-in limit of 128,000 tokens for that provider

#### Scenario: Config overrides built-in limit
- **WHEN** a provider is configured with `max_context: 150000`
- **THEN** the system uses 150,000 as the context limit, regardless of the built-in value

#### Scenario: Unknown model has no limit
- **WHEN** a provider uses a model not in the built-in registry and has no `max_context` override
- **THEN** the system treats the context limit as unlimited (no warnings, no exclusion)

### Requirement: TUI displays token usage per provider
The system SHALL display the current session token usage for each provider in the status bar, formatted as `used/limit` (e.g., `12.4K/200K`).

#### Scenario: Usage displayed in status bar
- **WHEN** a provider has consumed 12,400 input tokens and has a 200,000 token limit
- **THEN** the status bar shows `12.4K/200K` next to that provider's name

#### Scenario: Unlimited provider shows usage only
- **WHEN** a provider has no known context limit
- **THEN** the status bar shows only the used token count (e.g., `12.4K`) with no limit denominator

### Requirement: Visual warning at high usage
The system SHALL display a visual warning when any provider's cumulative input token usage exceeds 80% of its context window limit.

#### Scenario: Provider at 80% usage
- **WHEN** a provider's session input tokens reach 80% of its context limit
- **THEN** the provider's usage display in the status bar changes to a warning color (yellow/amber)

#### Scenario: Provider at 95% usage
- **WHEN** a provider's session input tokens reach 95% of its context limit
- **THEN** the provider's usage display changes to a critical color (red)

### Requirement: Skip providers exceeding context limit
The system SHALL exclude a provider from fan-out dispatch when its estimated next-turn input token count would exceed its context window limit.

#### Scenario: Provider excluded due to limit
- **WHEN** a provider's last reported input token count is at or above its context limit
- **THEN** the system skips that provider for the current query and shows a notice in the TUI

#### Scenario: Primary provider excluded
- **WHEN** the primary provider would exceed its context limit
- **THEN** the system reports an error indicating the conversation must be reset or the primary's limit increased

#### Scenario: Non-primary excluded, others continue
- **WHEN** a non-primary provider is excluded due to context limits but the primary and at least one other provider are within limits
- **THEN** the query proceeds with the remaining providers
