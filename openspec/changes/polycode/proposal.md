## Why

There is no terminal-based coding assistant that queries multiple LLMs simultaneously and synthesizes their responses into a single consensus answer. Today, developers must choose one AI tool (Claude Code, Codex, Gemini CLI, etc.) and trust its output in isolation. Different models have different strengths — one may excel at debugging, another at architecture, another at specific languages. Polycode eliminates this trade-off by fan-out querying all configured LLMs in parallel and using a user-designated "primary" model to synthesize a consensus response, then acting on it.

## What Changes

- **New standalone TUI application** (`polycode`) built as a terminal-based interactive coding assistant
- **Multi-provider LLM integration** supporting Anthropic Claude, OpenAI, Google Gemini, and arbitrary OpenAI-compatible endpoints (e.g., OpenRouter, local models)
- **Authentication flexibility** — OAuth-based login flows (Claude, Gemini) and API key-based auth for all providers, plus custom endpoint configuration
- **Parallel fan-out query system** that sends every user prompt to all configured LLMs simultaneously
- **Primary/master model designation** where one configured LLM is marked as the "primary" and is responsible for synthesizing responses from all other models into a consensus
- **Consensus pipeline** — collects all model responses, feeds them to the primary model with a consensus prompt, and returns the synthesized answer
- **Action execution** — the consensus output drives tool use (file edits, shell commands, etc.) just like single-model coding assistants
- **Provider configuration system** — YAML/TOML config file for managing providers, API keys, model selections, and primary designation
- **Streaming TUI** with real-time display of which models have responded, their individual outputs, and the final consensus

## Capabilities

### New Capabilities
- `provider-management`: Configuration, authentication, and lifecycle management for multiple LLM providers (Anthropic, OpenAI, Google, custom OpenAI-compatible endpoints)
- `fan-out-query`: Parallel dispatch of user prompts to all configured providers and collection of responses
- `consensus-engine`: Synthesis of multiple LLM responses into a single consensus answer via the designated primary model
- `tui-interface`: Terminal user interface for interactive multi-model coding assistance with streaming output
- `action-execution`: Execution of coding actions (file operations, shell commands) based on consensus output
- `provider-auth`: OAuth and API key authentication flows for supported providers

### Modified Capabilities
_(none — this is a greenfield project)_

## Impact

- **New codebase**: Entirely new Go application (leveraging Bubble Tea for TUI)
- **External API dependencies**: Anthropic API, OpenAI API, Google Generative AI API, plus arbitrary OpenAI-compatible endpoints
- **Auth flows**: OAuth 2.0 device/browser flows for Claude and Gemini; API key storage for all providers
- **Configuration**: New config file format (~/.polycode/config.yaml) for provider setup
- **Network**: Concurrent outbound HTTPS connections to multiple LLM APIs per query
- **Security**: Secure storage of API keys and OAuth tokens
