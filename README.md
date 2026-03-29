<p align="center">
  <img src="logo.svg" alt="polycode" width="600"/>
</p>

<p align="center">
  <strong>A multi-model consensus coding assistant for your terminal.</strong>
</p>

> **Alpha software.** Polycode is under active development and things will break. APIs, config formats, and behavior may change between releases. Bug reports and feedback are welcome — but don't rely on it for production workflows yet.

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

Query every configured LLM simultaneously. Responses fan out in parallel (latency = slowest provider, not sum of all), and your designated **primary** model synthesizes them into a single answer — identifying areas of agreement, unique insights, and errors. Providers can read files during fan-out to give codebase-aware answers.

### Supported Providers

| Provider | Type | Streaming | Tool Use | Reasoning | Auth |
|----------|------|-----------|----------|-----------|------|
| **Anthropic Claude** | `anthropic` | SSE | Function calling | Extended thinking | API key |
| **OpenAI (GPT, o-series)** | `openai` | SSE | Function calling | reasoning_effort | API key |
| **Google Gemini** | `google` | SSE | Function calling | thinkingBudget | API key, OAuth |
| **OpenAI-compatible** | `openai_compatible` | SSE | Function calling | reasoning_effort | API key, none |

OpenAI-compatible covers **OpenRouter**, **Ollama**, **vLLM**, **LM Studio**, **Together AI**, and any endpoint that speaks the OpenAI chat completions API.

### Interactive TUI

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss):

- ASCII art splash screen on launch
- Scrollable chat conversation with full multi-turn dialogue
- Real-time streaming from each provider in labeled panels
- **Provider activity traces** — each provider tab shows the full participation trace (fan-out, synthesis, tool execution, verification) with labeled phase boundaries
- Consensus output panel with visual emphasis
- **Command palette** — type `/` to see all commands with descriptions, filter by typing, Tab to accept
- **Input history** — Up/Down arrows cycle through previously submitted prompts
- **Consensus provenance panel** — press `p` to see confidence, agreements, minority reports, and routing decisions
- Status bar with provider health indicators, token usage, and per-provider cost
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

- Per-provider token counts displayed in the tab bar (`12.4K/200K`)
- **Estimated cost** per provider from litellm pricing data (e.g., `$0.12`)
- Color-coded warnings at **80%** (amber) and **95%** (red) of context limits
- Automatic provider exclusion when context limit is reached
- **Context auto-summarization** — when nearing 80% of context limit, early conversation turns are compressed into a summary to free tokens
- Consensus synthesis usage tracked separately

### Dynamic Model Metadata

Polycode fetches model information from [litellm's model database](https://github.com/BerriAI/litellm) — a community-maintained registry of **1,000+ models** with context window sizes, pricing, and capability flags. No manual updates needed when new models ship.

- Cached locally with configurable TTL (default 24h)
- Three-tier limit resolution: config override > litellm data > built-in fallback
- Works offline using cached or hardcoded data
- Capability awareness: function calling, vision, reasoning support
- **Pricing data** used for per-provider cost estimation (`input_cost_per_token`, `output_cost_per_token`)
- **Endpoint discovery** for OpenAI-compatible providers: queries `GET /models` on the configured base URL to list available models, cross-referenced with litellm for capability metadata

### Tool Execution

The consensus output can drive coding actions, just like single-model assistants:

- **File read** — read file contents with optional line ranges (`start_line`/`end_line`). Directories return a listing. No confirmation needed.
- **File edit** — targeted search-and-replace with unified diff preview. Models send just the changed text instead of rewriting whole files.
- **File write** — create or overwrite files with **unified diff preview**, user confirms before applying
- **File delete** — delete files or empty directories with confirmation. Non-recursive by design.
- **File rename** — move or rename files with confirmation. Prevents accidental overwrites.
- **File info** — get file metadata (size, line count, text/binary type) without reading contents. Available during fan-out.
- **Find files** — glob-based file search (e.g., `**/*_test.go`). Returns paths only. Available during fan-out.
- **List directory** — list directory contents, optionally recursive (up to 3 levels). Available during fan-out.
- **Grep search** — regex/text search with context lines, case-insensitive mode, include/exclude filters, files-only mode, configurable result limits. Available during fan-out.
- **Shell exec** — run commands with confirmation, **hardened destructive command detection** (`rm`, `sudo`, pipe-to-shell, `/dev/` paths, clobber operators, and more)
- **Tool-use loop** — the primary model can chain tool calls until done (no iteration limit, bounded by the 5-minute context timeout)
- **Auto-verification** — after file writes, auto-detected test suites (`go test`, `npm test`, `cargo test`, `pytest`, `make test`) run automatically and report pass/fail

### Authentication

- **API keys** stored in your OS keyring (macOS Keychain, Linux secret-service)
- **Encrypted file fallback** when keyring is unavailable
- **OAuth 2.0 device flow** for providers that support it (Gemini)
- **Automatic token refresh** — expired OAuth tokens are refreshed using stored refresh tokens before queries fail
- **No-auth mode** for local models (Ollama, etc.)

### Session Management

Sessions persist across restarts and can be named, listed, and switched:

- Auto-saved after each query with full conversation state
- **Named sessions** — `/name my-feature` to tag the current session
- **List sessions** — `/sessions` shows all saved sessions with exchange counts and timestamps
- **Session export** — `/export [path]` writes the session as JSON
- **Consensus traces** — each exchange records a full trace: routing mode, provider latencies, token usage, errors, and synthesis model
- **CLI management** — `polycode session list|show|delete` for headless workflows

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

Three canonical example skills are included in `examples/skills/`:
- **git-review** — `/review` for automated diff review with structured output
- **test-runner** — `/test` for detecting and running project test suites
- **security-audit** — `/audit` for scanning secrets, vulnerable deps, and injection patterns

### Operating Modes

All modes query every configured provider. The mode controls **synthesis depth** and **reasoning effort**:

| Mode | Synthesis | Reasoning |
|------|-----------|-----------|
| **`quick`** | Concise, direct answer — no structured sections | Low |
| **`balanced`** | Structured synthesis — confidence, agreements, minority reports, evidence | Medium |
| **`thorough`** | Deep analysis — step-by-step reasoning, trade-offs, verification, alternatives | High |

Switch modes with `/mode quick`, `/mode balanced`, or `/mode thorough`, or open the mode picker from the tab bar. Reasoning effort maps to each provider's native parameter:

| Provider | Low | Medium | High |
|----------|-----|--------|------|
| Anthropic | `thinking.budget_tokens: 4096` | `10000` | `32000` |
| OpenAI | `reasoning_effort: "low"` | `"medium"` | `"high"` |
| Gemini | `thinkingBudget: 4096` | `10000` | `32000` |

Models without reasoning support silently ignore the parameter.

### Hooks, Permissions, and MCP

- **Hooks** — Run shell commands at lifecycle events: `pre_query`, `post_query`, `post_tool`, `on_error`
- **Permissions** — Per-tool approval policies (`allow`, `ask`, `deny`) scoped by repo or user in `permissions.yaml`
- **MCP** — Connect to [Model Context Protocol](https://modelcontextprotocol.io) servers for external tools, resources, and prompts. Full TUI wizard with curated server registry, auto-reconnect, per-call timeouts, debug logging, and runtime reconfiguration. Manage via `/mcp` command or settings view.
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
| `polycode mcp list\|add\|remove\|test` | Manage MCP servers |
| `polycode mcp search <query>` | Search the MCP server registry |
| `polycode mcp browse` | Browse registry and install a server interactively |
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

You'll see the polycode splash screen, then the chat interface. Start typing and press Enter. Type `/` to open the command palette with all available commands.

### 3. Add more providers

```bash
polycode provider add
```

Or press `Ctrl+S` in the TUI to open settings, then `a`. The more providers you configure, the better the consensus.

### Slash commands

Type `/` to open the command palette, or type commands directly:

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
| `/mcp [list\|status\|reconnect\|tools\|resources\|prompts\|search\|add\|remove]` | Manage MCP servers |
| `/settings` | Open provider settings (+ MCP servers with Tab) |
| `/yolo` | Toggle auto-approve for all tool actions |
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
  verify_command: ""         # optional: override auto-detected test command

metadata:
  cache_ttl: 24h             # how often to refresh litellm model data
  # url: https://...         # optional: override metadata source URL

tui:
  theme: dark
  show_individual: true      # show individual provider panels by default

mcp:
  servers:
    - name: filesystem
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path"]
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
| `verify_command` | auto-detected | Test command to run after file writes. Auto-detects `go test`, `npm test`, `cargo test`, `pytest`, `make test`. |

### MCP server settings

Add MCP servers under the `mcp` key in config.yaml, or use the TUI wizard (`/mcp add` or `a` in the MCP settings section):

```yaml
mcp:
  debug: false              # log JSON-RPC traffic to ~/.config/polycode/mcp-debug.log
  servers:
    - name: filesystem
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/project"]
      read_only: true       # skip confirmation for this server's tools

    - name: github
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
      env:
        GITHUB_TOKEN: $KEYRING:mcp_github_GITHUB_TOKEN

    - name: remote-server
      url: http://localhost:3000/mcp    # HTTP/SSE transport
      timeout: 60                        # per-call timeout in seconds (default 30)
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique server identifier |
| `command` | Conditional | Command to spawn (stdio transport). Required if `url` not set. |
| `args` | No | Arguments for the command |
| `url` | Conditional | Server URL (HTTP/SSE transport). Required if `command` not set. |
| `env` | No | Environment variables. Use `$KEYRING:key_name` for secrets. |
| `read_only` | No | Skip confirmation for this server's tools (default `false`) |
| `timeout` | No | Per-call timeout in seconds (default `30`) |

### How MCP tools are distributed across providers

MCP tools follow a **split allocation** based on safety:

- **Primary model** (the consensus synthesizer) gets **all** MCP tools — both read-only and mutating. Mutating tool calls go through the confirmation gate (permission policies, yolo mode, or user prompt). This is the model that calls tools during synthesis and tool-loop execution.

- **Fan-out providers** (non-primary models queried in parallel) get only **read-only** MCP tools. A tool is read-only if the server is configured with `read_only: true` or the tool has `readOnlyHint` in its MCP annotation. This lets secondary models safely query databases, read files, or search via MCP during the fan-out phase without side effects.

The consensus flow:
1. **Fan-out** — all providers query in parallel, each with read-only MCP tools available
2. **Collect** — responses gathered from all providers
3. **Synthesize** — primary model gets the full tool set (including mutating MCP tools) and can call them during synthesis/tool loops

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

OAuth tokens are automatically refreshed when they expire — no manual re-authentication needed.

---

## Keyboard Shortcuts

### Chat View

| Key | Action |
|-----|--------|
| `Enter` | Submit prompt |
| `Shift+Enter` | New line in input |
| `↓` | Clear input (on last line) |
| `Tab` | Accept command palette suggestion / toggle provider panels |
| `↑` / `↓` | Cycle through input history (when input is empty) |
| `Ctrl+P` | Open command palette (search commands and files) |
| `Ctrl+E` | Open external editor to compose prompt |
| `Ctrl+T` | Switch color theme |
| `Ctrl+G` | Toggle auto-scroll lock |
| `Ctrl+H` | Toggle tool call concealment |
| `Ctrl+S` | Open settings |
| `@` | Attach file to prompt (fuzzy search) |
| `!command` | Run shell command, inject output as context |
| `j`/`k` (tab bar) | Scroll one line |
| `d`/`u` (tab bar) | Half-page scroll |
| `g`/`G` (tab bar) | Jump to top/bottom |
| `y` (tab bar) | Copy last response to clipboard |
| `t` (tab bar) | Toggle trace expansion |
| `c` (tab bar) | Cancel selected provider during query |
| `m` (tab bar) | Toggle MCP dashboard |
| `p` (tab bar) | Toggle consensus provenance panel |
| `?` | Show help overlay |
| `Ctrl+C` | Quit |

### Approval Prompt

| Key | Action |
|-----|--------|
| `y` | Approve |
| `n` / `Esc` | Reject |
| `a` | Allow for session (blocked for destructive tools) |
| `e` | Edit command/content before execution |

### Settings View

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `a` | Add new provider |
| `e` | Edit selected provider |
| `d` | Delete selected provider |
| `x` | Disable/enable selected provider |
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
                        Truncate to fit context
                                    │
                                    ▼
                        Primary model synthesizes
                         (mode-aware prompt)
                                    │
                                    ▼
                          Consensus response
                                    │
                                    ▼
                         Tool execution (if any)
                                    │
                                    ▼
                        Verification (if files written)
```

### Synthesis prompt (balanced mode)

The primary model receives a structured prompt containing every provider's response. The prompt varies by mode — `quick` asks for a concise direct answer, `balanced` (shown below) asks for structured analysis, and `thorough` adds trade-off analysis, step-by-step reasoning, and cross-model verification:

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

Analyze all responses and produce a synthesis with this structure:

## Recommendation
[Your synthesized answer]

## Confidence: [high/medium/low]

## Agreement
[Points where all or most models agree]

## Minority Report
[Dissenting views worth considering]

## Evidence
[Key facts, code references, or documentation cited]
```

### Edge cases

- **Single provider responds** — if only the primary responds, its answer is used directly (no synthesis step)
- **Provider failure** — failed/timed-out providers are excluded; consensus proceeds with available responses
- **Below minimum threshold** — if fewer than `min_responses` providers respond, an error is shown
- **Context overflow** — if combined responses exceed the primary's context window, they're proportionally truncated with `[truncated]` markers
- **Auto-summarization** — if conversation context reaches 80% of the primary model's limit, early turns are compressed into a summary

---

## Architecture

```
polycode/
├── cmd/polycode/           # CLI entry point, app wiring
│   ├── main.go             # Cobra commands (root, auth, skill, session, init, review, ci, serve)
│   ├── app.go              # TUI startup, subsystem wiring, conversation loop
│   ├── setup.go            # Interactive setup wizard
│   ├── review.go           # polycode review subcommand
│   ├── ci.go               # polycode ci (headless PR review)
│   ├── serve.go            # Editor bridge HTTP server
│   └── sharing.go          # Export/import sessions
├── examples/               # Example skills and config templates
│   ├── skills/             # Canonical skills (git-review, test-runner, security-audit)
│   ├── permissions.yaml    # Example permission policies
│   ├── instructions.md     # Example project instructions
│   └── skill-manifest.yaml # Annotated skill manifest template
├── evals/                  # Evaluation framework
│   ├── golden_tasks_test.go    # End-to-end execution pipeline tests
│   └── review_benchmark_test.go # Seeded review quality benchmarks
├── internal/
│   ├── config/             # YAML config types, loading, validation, session persistence
│   ├── provider/           # Provider interface + 4 adapters (Anthropic, OpenAI, Gemini, OpenAI-compat)
│   ├── consensus/          # Fan-out, truncation, mode-aware synthesis pipeline
│   ├── tokens/             # Token tracking, cost estimation, model metadata (litellm)
│   ├── action/             # Tool execution (10 tools), safety guardrails, project context
│   ├── auth/               # Credential management (keyring, file, OAuth + auto-refresh)
│   ├── tui/                # Bubble Tea terminal UI (command palette, provenance, input history)
│   ├── hooks/              # Lifecycle hooks (pre_query, post_query, etc.)
│   ├── permissions/        # Per-tool approval policies (allow/ask/deny)
│   ├── routing/            # Mode-based routing with telemetry scoring
│   ├── memory/             # Repo memory + instruction hierarchy
│   ├── mcp/                # Model Context Protocol client
│   ├── skill/              # Skills/plugin system (manifest, registry, execution)
│   ├── agent/              # Agent teams (task graph, role-based workers, checkpoints)
│   └── telemetry/          # Event logging (latency, errors) for routing calibration
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
Yes — every query hits N providers plus a consensus synthesis call. Cost scales linearly with the number of providers. Per-provider costs are shown in the tab bar (from litellm pricing data). Use `max_context` overrides to limit token consumption.

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

**Do all modes query all providers?**
Yes. Quick, balanced, and thorough modes all fan out to every configured provider. The mode controls how deeply the primary model analyzes and synthesizes the responses, and how much reasoning effort each provider applies.

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
