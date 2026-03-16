# polycode

A multi-model consensus coding assistant for your terminal. Polycode queries multiple LLMs in parallel and synthesizes their responses into a single authoritative answer through a designated primary model.

## How it works

1. You type a prompt
2. Polycode fans out the query to all configured LLM providers simultaneously
3. Each provider's response streams in real-time in the TUI
4. The designated **primary** model synthesizes all responses into a consensus answer
5. The consensus drives tool execution (file edits, shell commands) — just like a single-model coding assistant, but smarter

## Features

- **Multi-provider support**: Anthropic Claude, OpenAI, Google Gemini, and any OpenAI-compatible endpoint (OpenRouter, Ollama, etc.)
- **Parallel fan-out**: All providers queried concurrently — latency is max(providers), not sum
- **Consensus synthesis**: The primary model evaluates all responses and produces the best answer
- **Continuous dialogue**: Full conversation context carried across turns
- **Token tracking**: Per-provider usage display with context window warnings (80%/95% thresholds)
- **Dynamic model metadata**: Fetches model limits and capabilities from litellm's database — supports 1000+ models without manual updates
- **Tool execution**: File read/write, shell commands with confirmation prompts and safety guardrails
- **OAuth & API key auth**: OAuth device flow for Claude/Gemini, API keys for all providers, OS keyring storage

## Installation

### From source

```bash
go install github.com/anthonyizzo/polycode/cmd/polycode@latest
```

### Build from repo

```bash
git clone https://github.com/anthonyizzo/polycode.git
cd polycode
go build -o polycode ./cmd/polycode/
```

## Quick start

```bash
# Initialize configuration (interactive wizard)
polycode init

# Start the TUI
polycode
```

## Configuration

Configuration lives at `~/.config/polycode/config.yaml`:

```yaml
providers:
  - name: claude
    type: anthropic
    auth: api_key
    model: claude-sonnet-4-20250514
    primary: true                    # this model synthesizes consensus
  - name: gpt4
    type: openai
    auth: api_key
    model: gpt-4o
  - name: gemini
    type: google
    auth: api_key
    model: gemini-2.5-pro
  - name: local-llama
    type: openai_compatible
    base_url: http://localhost:11434/v1
    model: llama3
    auth: none

consensus:
  timeout: 60s          # max wait per provider
  min_responses: 2      # minimum responses before synthesizing

metadata:
  cache_ttl: 24h        # how often to refresh litellm model data
  # url: https://custom-mirror.example.com/models.json  # optional override

tui:
  theme: dark
  show_individual: true
```

### Provider types

| Type | Description | Auth methods |
|------|-------------|--------------|
| `anthropic` | Anthropic Claude API | `api_key`, `oauth` |
| `openai` | OpenAI API | `api_key` |
| `google` | Google Gemini API | `api_key`, `oauth` |
| `openai_compatible` | Any OpenAI-compatible endpoint | `api_key`, `none` |

### Per-provider options

- `max_context`: Override the detected context window limit (tokens)
- `base_url`: Required for `openai_compatible` type

## Authentication

```bash
# Store an API key
polycode auth login claude

# Check auth status
polycode auth status

# Remove credentials
polycode auth logout claude
```

API keys are stored in your OS keyring (macOS Keychain, Linux secret-service). Falls back to encrypted file storage if keyring is unavailable.

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Submit prompt |
| `Shift+Enter` | New line in input |
| `Tab` | Toggle individual provider panels |
| `Ctrl+C` | Quit |
| Any key | Dismiss splash screen |

## How consensus works

When you send a prompt, polycode:

1. **Dispatches** to all healthy providers in parallel
2. **Collects** streaming responses (with configurable timeout)
3. **Synthesizes** by sending all responses to the primary model with instructions to identify areas of agreement, unique insights, and errors
4. **Acts** on the consensus output (file operations, shell commands)

The primary model sees something like:

```
You are synthesizing responses from multiple AI models...

[Model: claude] Use a map for O(1) lookups...
[Model: gpt4] A binary search tree would work well...
[Model: gemini] Consider a trie for prefix matching...

Analyze all responses and produce the best synthesis.
```

## License

MIT
