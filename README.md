<p align="center">
  <img src="logo.svg" alt="polycode" width="600"/>
</p>

<p align="center">
  <strong>A multi-model consensus coding assistant for your terminal.</strong>
</p>

---

Polycode queries multiple LLMs in parallel — Anthropic Claude, OpenAI, Google Gemini, local models, or any OpenAI-compatible endpoint — and synthesizes their responses into a single authoritative answer through a designated primary model. Think of it as Claude Code, Codex, or opencode, but backed by the collective intelligence of every AI model you have access to.

---

## Why polycode?

Every AI model has blind spots. Claude might excel at architecture, GPT at debugging, Gemini at specific frameworks. Today you pick one and hope it's right. Polycode eliminates the trade-off: ask once, get answers from all of them, and let your strongest model synthesize the best response.

- **Better answers** — consensus catches errors that single models miss
- **No vendor lock-in** — works with any combination of providers
- **Same workflow** — continuous dialogue with tool execution, just like single-model assistants
- **Full visibility** — see what each model said before consensus

---

## Features

### Multi-Provider Consensus

Query every configured LLM simultaneously. Responses fan out in parallel (latency = slowest provider, not sum of all), and your designated **primary** model synthesizes them into a single answer — identifying areas of agreement, unique insights, and errors.

### Supported Providers

| Provider | Type | Streaming | Tool Use | Auth |
|----------|------|-----------|----------|------|
| **Anthropic Claude** | `anthropic` | SSE | Function calling | API key, OAuth |
| **OpenAI (GPT, o-series)** | `openai` | SSE | Function calling | API key |
| **Google Gemini** | `google` | SSE | Function calling | API key, OAuth |
| **OpenAI-compatible** | `openai_compatible` | SSE | Function calling | API key, none |

OpenAI-compatible covers **OpenRouter**, **Ollama**, **vLLM**, **LM Studio**, **Together AI**, and any endpoint that speaks the OpenAI chat completions API.

### Interactive TUI

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss):

- ASCII art splash screen on launch
- Scrollable chat conversation with full multi-turn dialogue
- Real-time streaming from each provider in labeled panels
- Consensus output panel with visual emphasis
- Status bar with provider health indicators and token usage
- Toggle between consensus-only and all-panels view with `Tab`

### In-TUI Provider Configuration

No need to edit YAML — configure everything from within the TUI:

- **`Ctrl+S`** opens the settings screen
- **Add** providers with a step-by-step wizard (type, name, auth, model, primary)
- **Edit** existing providers inline
- **Delete** providers with confirmation (primary removal blocked until reassigned)
- **Test** provider connections with a single keypress
- Changes apply **immediately** — no restart required

### Token Usage & Cost Tracking

Real-time visibility into how much context and budget you're consuming:

- Per-provider token counts displayed in the status bar (`12.4K/200K`)
- **Estimated cost** per provider from litellm pricing data (e.g., `$0.12`)
- Color-coded warnings at **80%** (amber) and **95%** (red) of context limits
- Automatic provider exclusion when context limit is reached
- **Context auto-summarization** — when nearing 80% of context limit, early conversation turns are compressed into a summary to free tokens
- Consensus synthesis usage tracked separately

### Dynamic Model Metadata

Polycode fetches model information from [litellm's model database](https://github.com/BerriAI/litellm) — a community-maintained registry of **1,000+ models** with context window sizes and capability flags. No manual updates needed when new models ship.

- Cached locally with configurable TTL (default 24h)
- Three-tier limit resolution: config override > litellm data > built-in fallback
- Works offline using cached or hardcoded data
- Capability awareness: function calling, vision, reasoning support
- **Endpoint discovery** for OpenAI-compatible providers: queries `GET /models` on the configured base URL to list available models, cross-referenced with litellm for capability metadata

### Tool Execution

The consensus output can drive coding actions, just like single-model assistants:

- **File read** — read file contents into conversation context (no confirmation needed)
- **File write** — propose changes with unified diff preview, user confirms before applying
- **Shell exec** — run commands with confirmation, destructive command detection (`rm`, `sudo`, etc.)
- **Tool-use loop** — the primary model can chain multiple tool calls (up to 10 iterations)

### Authentication

- **API keys** stored in your OS keyring (macOS Keychain, Linux secret-service)
- **Encrypted file fallback** when keyring is unavailable
- **OAuth 2.0 device flow** for providers that support it (Claude, Gemini)
- **No-auth mode** for local models (Ollama, etc.)

### Skills / Plugins

Extend polycode with installable skills that add slash commands, system prompts, and tool definitions:

```bash
polycode skill install ./my-skill    # install from local directory
polycode skill list                  # list installed skills
polycode skill remove my-skill       # uninstall
```

Skills live in `~/.config/polycode/skills/` with a `skill.yaml` manifest:

```yaml
name: git-review
version: "1.0"
description: Automated git diff review
command: review          # registers /review slash command
tools:
  - name: diff
    description: Get the current git diff
    handler: git diff --cached
```

### Operating Modes

All modes query every configured provider. The mode controls **synthesis depth** and **reasoning effort**:

| Mode | Synthesis | Reasoning |
|------|-----------|-----------|
| **`quick`** | Concise, direct answer — no structured sections | Low |
| **`balanced`** | Structured synthesis — confidence, agreements, minority reports, evidence | Medium |
| **`thorough`** | Deep analysis — step-by-step reasoning, trade-offs, verification, alternatives | High |

Switch modes with `/mode quick`, `/mode balanced`, or `/mode thorough`, or open the mode picker from the tab bar. Reasoning effort maps to each provider's native parameter (Anthropic `thinking`, OpenAI `reasoning_effort`, Gemini `thinkingBudget`).

### Hooks, Permissions, and MCP

- **Hooks** — Run shell commands at lifecycle events: `pre_query`, `post_query`, `post_tool`, `on_error`
- **Permissions** — Per-tool approval policies (`allow`, `ask`, `deny`) scoped by repo or user in `permissions.yaml`
- **MCP** — Connect to Model Context Protocol servers for external tool discovery and invocation
- **Repo memory** — Persistent notes in `~/.config/polycode/memory/` injected into the system prompt
- **Instructions** — Instruction hierarchy: `.polycode/instructions.md` > `~/.config/polycode/instructions.md` > default

### CLI Commands

| Command | Description |
|---------|-------------|
| `polycode` | Launch interactive TUI |
| `polycode init` | Setup wizard |
| `polycode review [--pr N]` | Review code changes via consensus |
| `polycode ci --pr N` | Headless PR review for CI pipelines |
| `polycode serve [--port N]` | Editor bridge HTTP server (default 9876) |
| `polycode export [--format md\|json]` | Export session |
| `polycode import <file>` | Import session |
| `polycode skill list\|install\|remove` | Manage skills |
| `polycode session list\|show\|delete` | Manage saved sessions |
| `polycode auth login\|logout\|status` | Manage credentials |
| `polycode config edit\|show\|path` | Manage configuration |

---

## Installation

### Prerequisites

- Go 1.21 or later

### From source

```bash
go install github.com/izzoa/polycode/cmd/polycode@latest
```

### Build from repo

```bash
git clone https://github.com/izzoa/polycode.git
cd polycode
go build -o polycode ./cmd/polycode/
```

### Cross-platform builds

Polycode includes a [GoReleaser](https://goreleaser.com/) config for building binaries for macOS (amd64/arm64), Linux (amd64/arm64), and Windows (amd64/arm64):

```bash
goreleaser build --snapshot --clean
```

---

## Quick Start

### 1. Initialize

```bash
polycode init
```

The interactive wizard walks you through configuring providers with selectable lists for provider type, auth method, and model. For OpenAI-compatible endpoints (Ollama, vLLM, OpenRouter, etc.), the wizard automatically discovers available models from the server. Other providers show models from litellm's database with capability info. After each provider, the wizard asks if you'd like to add another.

### 2. Launch

```bash
polycode
```

You'll see the polycode splash screen, then the chat interface. Start typing and press Enter. Use `/help` to see all available commands.

### 3. Add more providers

```bash
polycode provider add
```

Or press `Ctrl+S` in the TUI to open settings, then `a`. The more providers you configure, the better the consensus.

### Slash commands

| Command | Action |
|---------|--------|
| `/help` | Show keyboard shortcuts and commands |
| `/clear` | Clear conversation and reset context |
| `/save` | Save session to disk |
| `/export [path]` | Export session as JSON |
| `/mode <name>` | Switch mode: quick, balanced, thorough |
| `/plan <request>` | Run multi-model agent team pipeline |
| `/sessions` | List all saved sessions |
| `/name <name>` | Name the current session |
| `/memory` | View repo memory |
| `/skill [list\|install\|remove]` | Manage installed skills/plugins |
| `/yolo` | Toggle auto-approve for all tool actions |
| `p` | Toggle consensus provenance panel (confidence, agreements, routing) |
| `/exit` | Quit polycode |

---

## Configuration

### Config file

Configuration lives at `~/.config/polycode/config.yaml` (follows XDG base directory spec):

```yaml
providers:
  - name: claude
    type: anthropic
    auth: api_key
    model: claude-sonnet-4-20250514
    primary: true                        # this model synthesizes consensus

  - name: gpt4
    type: openai
    auth: api_key
    model: gpt-4o

  - name: gemini
    type: google
    auth: api_key
    model: gemini-2.5-pro

  - name: deepseek
    type: openai_compatible
    base_url: https://openrouter.ai/api/v1
    auth: api_key
    model: deepseek/deepseek-r1

  - name: local-llama
    type: openai_compatible
    base_url: http://localhost:11434/v1
    model: llama3
    auth: none

consensus:
  timeout: 60s              # max wait per provider before proceeding
  min_responses: 2           # minimum responses needed before synthesizing

metadata:
  cache_ttl: 24h             # how often to refresh litellm model data
  # url: https://...         # optional: override metadata source URL

tui:
  theme: dark
  show_individual: true      # show individual provider panels by default
```

### Provider options

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique identifier for the provider |
| `type` | Yes | `anthropic`, `openai`, `google`, or `openai_compatible` |
| `model` | Yes | Model identifier (e.g., `claude-sonnet-4-20250514`, `gpt-4o`) |
| `auth` | Yes | `api_key`, `oauth`, or `none` |
| `primary` | No | Set to `true` on exactly one provider — the consensus synthesizer |
| `base_url` | Conditional | Required for `openai_compatible` type |
| `max_context` | No | Override the detected context window limit (in tokens) |

### Consensus settings

| Field | Default | Description |
|-------|---------|-------------|
| `timeout` | `60s` | Maximum time to wait for each provider's response |
| `min_responses` | `2` | Minimum successful responses before synthesis proceeds. Set to `1` if you only have one provider. |

---

## Authentication

### CLI commands

```bash
# Authenticate with a provider (prompts for API key or starts OAuth flow)
polycode auth login <provider-name>

# Check authentication status for all providers
polycode auth status

# Remove stored credentials
polycode auth logout <provider-name>
```

### In-TUI authentication

When adding a provider via the settings wizard (`Ctrl+S` → `a`), you'll be prompted for the API key as part of the flow. Keys are stored securely in your OS keyring.

### Credential storage

1. **OS keyring** (preferred) — macOS Keychain, Linux secret-service/libsecret
2. **Encrypted file** (fallback) — `~/.config/polycode/credentials.json` when keyring is unavailable
3. **No credentials** — for `auth: none` providers (local models)

---

## Keyboard Shortcuts

### Chat View

| Key | Action |
|-----|--------|
| `Enter` | Submit prompt |
| `Shift+Enter` | New line in input |
| `Tab` | Toggle individual provider panels |
| `Ctrl+S` | Open settings |
| `?` | Show help overlay |
| `Ctrl+C` | Quit |

### Settings View

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `a` | Add new provider |
| `e` | Edit selected provider |
| `d` | Delete selected provider |
| `t` | Test selected provider connection |
| `Esc` | Return to chat |

### Wizard

| Key | Action |
|-----|--------|
| `Enter` | Advance to next step / confirm |
| `j` / `k` | Navigate selection lists |
| `Esc` | Cancel wizard |

---

## How Consensus Works

### The Pipeline

```
User prompt
    │
    ├──→ Provider A (goroutine) ──→ Response A ─┐
    ├──→ Provider B (goroutine) ──→ Response B ──┤
    ├──→ Provider C (goroutine) ──→ Response C ──┤
    └──→ Primary   (goroutine) ──→ Response P ──┤
                                                 │
                                    ┌────────────┘
                                    ▼
                           Collect all responses
                                    │
                                    ▼
                        Primary model synthesizes
                         (sees all responses)
                                    │
                                    ▼
                          Consensus response
                                    │
                                    ▼
                         Tool execution (if any)
```

### Synthesis prompt

The primary model receives a structured prompt containing every provider's response:

```
You are synthesizing responses from multiple AI models to produce
the best possible answer.

Original question: How should I implement caching here?

Model responses:
---
[Model: claude]
Use an LRU cache with a TTL...
---
[Model: gpt4]
Consider Redis for distributed caching...
---
[Model: gemini]
A simple in-memory map with mutex...
---

Analyze all responses. Identify areas of agreement, unique insights,
and any errors. Produce a single, authoritative response that
represents the best synthesis.
```

### Edge cases

- **Single provider responds** — if only the primary responds, its answer is used directly (no synthesis step)
- **Provider failure** — failed/timed-out providers are excluded; consensus proceeds with available responses
- **Below minimum threshold** — if fewer than `min_responses` providers respond, an error is shown
- **Context overflow** — if combined responses exceed the primary's context window, they're proportionally truncated with `[truncated]` markers

---

## Architecture

```
polycode/
├── cmd/polycode/           # CLI entry point, app wiring
│   ├── main.go             # Cobra commands (root, auth, skill, init, review, ci, serve)
│   ├── app.go              # TUI startup, subsystem wiring, conversation loop
│   ├── setup.go            # Interactive setup wizard
│   ├── review.go           # polycode review subcommand
│   ├── ci.go               # polycode ci (headless PR review)
│   ├── serve.go            # Editor bridge HTTP server
│   └── sharing.go          # Export/import sessions
├── evals/                  # Evaluation framework
│   ├── golden_tasks_test.go    # End-to-end execution pipeline tests
│   └── review_benchmark_test.go # Seeded review quality benchmarks
├── internal/
│   ├── config/             # YAML config types, loading, validation, saving
│   ├── provider/           # Provider interface + 4 adapters + registry
│   ├── consensus/          # Fan-out + synthesis pipeline
│   ├── tokens/             # Token tracking + model metadata (litellm)
│   ├── action/             # Tool execution (file_read, file_write, shell_exec)
│   ├── auth/               # Credential management (keyring, file, OAuth)
│   ├── tui/                # Bubble Tea terminal UI
│   ├── hooks/              # Lifecycle hooks (pre_query, post_query, etc.)
│   ├── permissions/        # Per-tool approval policies (allow/ask/deny)
│   ├── routing/            # Adaptive provider selection by mode + telemetry
│   ├── memory/             # Repo memory + instruction hierarchy
│   ├── mcp/                # Model Context Protocol client
│   ├── skill/              # Skills/plugin system (manifest, registry, execution)
│   ├── agent/              # Agent teams (task graph, role-based workers)
│   └── telemetry/          # Event logging for routing calibration
├── .goreleaser.yaml        # Cross-platform build config
└── openspec/               # Change management artifacts
```

---

## Examples

### Single provider (getting started)

```yaml
providers:
  - name: claude
    type: anthropic
    auth: api_key
    model: claude-sonnet-4-20250514
    primary: true

consensus:
  min_responses: 1    # no consensus needed with one provider
```

### Two cloud providers

```yaml
providers:
  - name: claude
    type: anthropic
    auth: api_key
    model: claude-sonnet-4-20250514
    primary: true
  - name: gpt4
    type: openai
    auth: api_key
    model: gpt-4o

consensus:
  min_responses: 2
```

### Cloud + local model

```yaml
providers:
  - name: claude
    type: anthropic
    auth: api_key
    model: claude-sonnet-4-20250514
    primary: true
  - name: ollama
    type: openai_compatible
    base_url: http://localhost:11434/v1
    model: llama3
    auth: none

consensus:
  min_responses: 2
```

### Everything via OpenRouter

```yaml
providers:
  - name: claude-via-or
    type: openai_compatible
    base_url: https://openrouter.ai/api/v1
    auth: api_key
    model: anthropic/claude-sonnet-4-20250514
    primary: true
  - name: gpt4-via-or
    type: openai_compatible
    base_url: https://openrouter.ai/api/v1
    auth: api_key
    model: openai/gpt-4o
  - name: gemini-via-or
    type: openai_compatible
    base_url: https://openrouter.ai/api/v1
    auth: api_key
    model: google/gemini-2.5-pro

consensus:
  min_responses: 2
```

---

## FAQ

**Does polycode cost more than using a single model?**
Yes — every query hits N providers plus a consensus synthesis call. Cost scales linearly with the number of providers. Use `max_context` overrides and monitor token usage in the status bar.

**Can I use it with just one provider?**
Yes. Set `min_responses: 1` and polycode works like a standard single-model coding assistant — no consensus step.

**What if a provider is down?**
Failed providers are excluded from consensus. As long as the primary and at least `min_responses` providers respond, the query succeeds.

**Does the primary model see its own response in the consensus prompt?**
Yes. The primary participates in fan-out (generating its own independent response), then receives all responses (including its own) for synthesis. This lets it evaluate whether its initial answer was better or worse than alternatives.

**Can I use local models?**
Yes. Any model served via an OpenAI-compatible API (Ollama, vLLM, LM Studio, llama.cpp server) works with `type: openai_compatible` and `auth: none`.

**Where are my API keys stored?**
In your OS keyring (macOS Keychain, Linux secret-service). If the keyring isn't available, they fall back to `~/.config/polycode/credentials.json`. Keys are never stored in the config file.

---

## Contributing

Polycode uses [OpenSpec](https://github.com/fission-ai/openspec) for change management. Each feature goes through a proposal → design → specs → tasks workflow in `openspec/changes/`.

```bash
# Run tests
go test ./... -count=1

# Build
go build ./cmd/polycode/
```

---

## License

AGPLv3 — see [LICENSE](LICENSE) for details.
