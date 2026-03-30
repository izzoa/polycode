## Why

Polycode queries multiple LLMs per turn, but users have no visibility into how many tokens they're consuming or how close they are to each model's context window limit. Without token tracking, users risk hitting context limits mid-conversation (causing silent truncation or errors) and cannot estimate API costs. Every major coding assistant (Claude Code, Codex, opencode) displays token usage — polycode should too.

## What Changes

- **Token usage tracking per provider**: Each provider adapter reports input/output token counts from API responses
- **Per-model context window limits**: A registry of known model context limits (e.g., Claude Sonnet = 200K, GPT-4o = 128K, Gemini 2.5 Pro = 1M) with the ability to override via config
- **Session-wide token accumulator**: Running totals of input/output tokens per provider across the entire conversation session
- **TUI token usage display**: A live display in the status bar showing per-provider token usage as a fraction of the model's limit (e.g., `claude: 12.4K/200K`)
- **Context window warning**: Visual warning when a provider approaches its context limit (e.g., >80% usage) and automatic exclusion when a provider would exceed its limit on the next query
- **Consensus cost awareness**: The consensus synthesis call's token usage is tracked separately so users can see the overhead of the consensus step

## Capabilities

### New Capabilities
- `token-tracking`: Per-provider token counting, session accumulation, context limit awareness, and usage reporting

### Modified Capabilities
_(none — this adds new functionality without changing existing spec-level requirements)_

## Impact

- **`internal/provider/`**: `StreamChunk` and `Response` types need token usage fields; each adapter must parse usage data from API responses
- **`internal/tui/`**: Status bar needs token usage rendering; new warning display for approaching limits
- **`internal/config/`**: Optional `max_context` override per provider in config
- **`internal/consensus/`**: Pipeline needs to check remaining context budget before dispatching and before synthesis
- **New package `internal/tokens/`**: Token tracker, model limits registry
