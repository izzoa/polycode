## Context

Polycode is a greenfield TUI coding assistant that differentiates itself by querying multiple LLMs in parallel and synthesizing a consensus response. There is no existing codebase — this is a new standalone application. The target user is a developer who has API access to multiple LLM providers and wants to leverage the collective intelligence of multiple models rather than relying on a single one.

The application sits in the same category as Claude Code, OpenAI Codex CLI, and Gemini CLI, but adds a fan-out/consensus layer on top.

## Goals / Non-Goals

**Goals:**
- Build a usable TUI that accepts natural language prompts and returns consensus-driven coding assistance
- Support Anthropic Claude, OpenAI, Google Gemini, and custom OpenAI-compatible endpoints out of the box
- Parallel fan-out of queries to all configured providers with streaming response collection
- Configurable "primary" model that synthesizes consensus from all responses
- Execute coding actions (file read/write, shell commands) based on consensus output
- Provide clear visibility into individual model responses and the synthesis process
- Simple YAML-based configuration for provider management

**Non-Goals:**
- Building a GUI or web interface (TUI only for v1)
- Implementing agents with long-running autonomous loops (v1 is single-turn request/response with consensus)
- Building custom model hosting or inference (we only call external APIs)
- Implementing conversation branching or A/B comparison UIs
- Supporting non-coding use cases (this is a coding assistant)
- Plugin or extension system (v1 is monolithic)
- Cost optimization or smart routing (all configured models are always queried)

## Decisions

### 1. Language: Go + Bubble Tea

**Choice**: Go with the Bubble Tea TUI framework.

**Rationale**: Go produces single static binaries, has excellent concurrency primitives (goroutines for parallel API calls), and Bubble Tea is the most mature terminal UI framework available. Claude Code uses Node.js, OpenCode uses Go — Go is the proven choice for this category.

**Alternatives considered**:
- **Rust + Ratatui**: Higher performance ceiling but slower development velocity; smaller ecosystem for HTTP/API clients
- **Python + Textual**: Good prototyping speed but distribution is painful (requires Python runtime); concurrency model is weaker
- **TypeScript + Ink**: Good ecosystem but adds Node.js runtime dependency; not ideal for CLI distribution

### 2. Architecture: Pipeline with Fan-Out Collector

**Choice**: A three-stage pipeline: **Dispatch → Collect → Synthesize**

1. **Dispatch**: User prompt is sent to all configured providers concurrently via goroutines
2. **Collect**: Responses stream back and are collected with a configurable timeout (default 60s). A provider that fails or times out is excluded from consensus.
3. **Synthesize**: All collected responses are bundled into a consensus prompt and sent to the primary model. The primary's output becomes the final answer.

**Rationale**: This is the simplest architecture that achieves the goal. Each stage is independently testable. The pipeline is stateless per-request which simplifies the mental model.

**Alternatives considered**:
- **Iterative refinement**: Send to primary first, then ask secondary models to critique, then re-synthesize. More sophisticated but adds latency and complexity for v1.
- **Voting/scoring**: Have each model score other models' outputs. Requires structured output from all models which isn't reliably available.

### 3. Provider Abstraction: Interface-Based with Adapters

**Choice**: Define a `Provider` interface with methods like `Query(ctx, prompt) → ResponseStream`. Each provider (Anthropic, OpenAI, Gemini, OpenAI-compatible) implements this interface via an adapter.

```go
type Provider interface {
    ID() string
    Query(ctx context.Context, messages []Message, opts QueryOpts) (<-chan StreamChunk, error)
    Authenticate() error
    Validate() error
}
```

**Rationale**: Clean separation of concerns. Adding a new provider means implementing one interface. The fan-out system doesn't need to know provider-specific details.

**Alternatives considered**:
- **Single OpenAI-compatible adapter for everything**: Anthropic and Gemini have their own SDKs with features (like Claude's extended thinking) that wouldn't be accessible through an OpenAI-compatibility shim.

### 4. Configuration: YAML at ~/.config/polycode/config.yaml

**Choice**: YAML configuration file following XDG conventions.

```yaml
providers:
  - name: claude
    type: anthropic
    auth: oauth          # or "api_key"
    model: claude-sonnet-4-20250514
    primary: true        # this is the consensus synthesizer
  - name: gpt4
    type: openai
    auth: api_key
    model: gpt-4o
  - name: gemini
    type: google
    auth: oauth
    model: gemini-2.5-pro
  - name: local-llama
    type: openai_compatible
    base_url: http://localhost:11434/v1
    model: llama3
    auth: none

consensus:
  timeout: 60s           # max wait for slowest provider
  min_responses: 2       # minimum responses before synthesizing

tui:
  theme: dark
  show_individual: true  # show each model's response in panels
```

**Rationale**: YAML is human-readable and widely understood. XDG base dir spec is the standard for CLI tools on macOS/Linux. The config is declarative and easy to validate.

### 5. Consensus Prompt Strategy

**Choice**: The primary model receives a structured prompt containing all other models' responses, with instructions to synthesize the best answer.

The consensus prompt template:

```
You are synthesizing responses from multiple AI models to produce the best possible answer.

Original question: {user_prompt}

Model responses:
---
[Model: {name}]
{response}
---
[Model: {name}]
{response}
---

Analyze all responses. Identify areas of agreement, unique insights, and any errors.
Produce a single, authoritative response that represents the best synthesis.
If models disagree, use your judgment to determine the correct approach.
```

**Rationale**: This leverages the primary model's reasoning ability to evaluate and merge responses rather than using mechanical voting. The primary model can identify when a minority response is actually correct.

### 6. Action Execution: Tool-Use Pattern

**Choice**: The consensus output goes through a tool-use layer that can execute file operations and shell commands, similar to how Claude Code handles tool use. The primary model's response can include structured tool calls.

**Rationale**: This is the established pattern for coding assistants. The primary model (which produces the consensus) should be a model that supports tool use natively.

### 7. Auth: Keyring + OAuth Device Flow

**Choice**:
- API keys stored via the OS keyring (macOS Keychain, Linux secret-service)
- OAuth providers use device authorization flow (user visits URL, enters code)
- Fallback to encrypted file storage if keyring unavailable

**Rationale**: Keyring is the most secure option for CLI tools. Device flow works well in terminals where browser redirect isn't straightforward.

## Risks / Trade-offs

- **Latency**: Consensus adds a full extra round-trip to the primary model after collecting all responses. The total latency is `max(all_provider_latencies) + primary_synthesis_latency`. → **Mitigation**: Configurable timeout, streaming display of individual responses while waiting, option to skip slow providers.

- **Cost**: Every query hits N providers instead of 1, multiplying API costs by the number of configured providers, plus the consensus synthesis call. → **Mitigation**: Clear documentation about cost implications. Future: allow per-query provider selection.

- **Consensus quality**: The primary model might ignore good answers or introduce its own biases during synthesis. → **Mitigation**: Show individual model responses in the TUI so the user can evaluate. The consensus prompt explicitly asks the model to consider all responses.

- **Provider API instability**: Different providers have different API versions, rate limits, and error modes. → **Mitigation**: Each provider adapter handles its own error cases. Failed providers are excluded from consensus rather than failing the whole query.

- **OAuth complexity**: OAuth device flows for Claude and Gemini require maintaining token refresh logic and handling expiration gracefully. → **Mitigation**: Use well-tested OAuth libraries. Fall back to API keys as the simpler auth path.

- **Context window limits**: Feeding N full model responses into the primary's consensus prompt could exceed context limits for long responses. → **Mitigation**: Truncate individual responses if total exceeds a configurable threshold. Summarize rather than truncate when possible.
