## ADDED Requirements

### Requirement: RunPipeline is a reusable top-level function
The consensus package SHALL export a `RunPipeline()` function that encapsulates provider dispatch, response collection, and consensus synthesis without depending on Bubble Tea.

#### Scenario: Headless caller uses RunPipeline
- **WHEN** the headless run command calls `consensus.RunPipeline(ctx, config, prompt, opts)`
- **THEN** it SHALL receive the consensus text, token usage, and any error

#### Scenario: TUI caller uses RunPipeline
- **WHEN** the TUI initiates a query
- **THEN** it SHALL call RunPipeline with a message-emitting writer AND receive streaming chunks via the writer

### Requirement: RunPipeline accepts an io.Writer for streaming
RunPipeline SHALL accept an optional `io.Writer` parameter. Consensus chunks SHALL be written to this writer as they arrive.

#### Scenario: Writer receives chunks
- **WHEN** RunPipeline is called with an io.Writer
- **THEN** each consensus chunk SHALL be written to the writer as it arrives from the synthesizer

#### Scenario: Nil writer collects silently
- **WHEN** RunPipeline is called with a nil writer
- **THEN** the full response SHALL be returned as a string without streaming

### Requirement: RunPipeline options control behavior
RunPipeline SHALL accept options for: tool execution mode (auto/confirm/none), single provider override, and context.

#### Scenario: No-tools option
- **WHEN** RunPipeline is called with tool mode "none"
- **THEN** no tool calls SHALL be dispatched regardless of model requests

#### Scenario: Single provider option
- **WHEN** RunPipeline is called with a provider override
- **THEN** only that provider SHALL be queried and no consensus synthesis SHALL occur
