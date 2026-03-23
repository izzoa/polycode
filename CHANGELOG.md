# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.14.0] - 2026-03-23

### Added
- **Fan-out tool execution visibility**: Provider tabs now show "Executing file_read..." status messages and truncated tool output during fan-out, mirroring the primary provider's tool loop behavior. Previously tool execution happened silently.

### Fixed
- **Tool capability detection**: `openai_compatible` providers now included in tool fallback. Two-tier approach: trust litellm metadata when the model is known, fall back to type-based default when unknown. Previously openai-compatible providers were silently excluded from tools.
- **Fan-out errors surfaced in TUI**: When a provider's re-query fails during fan-out tool loop, the error is now sent to the TUI callback so the tab shows the failure instead of hanging in a loading state.
- **Anthropic role alternation**: Consecutive tool result messages are coalesced into a single `user` message with multiple `tool_result` content blocks, satisfying Anthropic's strict role alternation requirement. Previously consecutive tool results caused 400 errors.
- **Gemini tool call ID collisions**: Tool call IDs now include a counter (`gemini_call_file_read_1`, `_2`, etc.) to prevent collisions when multiple calls to the same function occur.

## [1.13.3] - 2026-03-23

### Fixed
- **Crash on fan-out tool usage**: Added panic recovery to fan-out provider goroutines so a single provider panic can't crash the entire application. Panics are now captured and surfaced as errors in the provider tab.
- **Gemini empty responses**: Gemini's SSE parser now sends a Done chunk on clean EOF when no STOP/FUNCTION_CALL finish reason was received. Previously the consumer could hang waiting for a terminal chunk, resulting in empty provider tabs.

## [1.13.2] - 2026-03-23

### Fixed
- **Fan-out tool failures masked as success**: When a provider's tool re-query fails or times out, the error is now correctly propagated instead of being masked as a successful response with partial text. Provider tabs now show the error state instead of a misleading checkmark.
- **OpenAI-compatible providers dropping tool calls on EOF**: The SSE parser now flushes buffered tool calls when a provider closes the stream without sending `[DONE]`. This was preventing `file_read` from being executed for providers that omit the `[DONE]` sentinel.

## [1.13.1] - 2026-03-23

### Fixed
- **Fan-out timeout too short for tool loops**: Fan-out timeout is now extended proportionally when tool loops are enabled (timeout × 4 for up to 3 tool rounds). Previously providers using `file_read` during fan-out would hit "context deadline exceeded" because the single-round timeout was shared across multiple LLM calls.
- **Empty provider responses on timeout**: When a provider's tool re-query is interrupted by timeout, any content accumulated from earlier rounds is now preserved and returned instead of being discarded. Fixes empty provider tabs (e.g., Gemini) when tool loops partially complete.

## [1.13.0] - 2026-03-23

### Added
- **Fan-out tool access**: Providers can now use `file_read`, MCP tools, and skill tools during fan-out to inspect the codebase and gather context before responding. Write/exec built-in tools remain synthesis-only. Tools are only sent to providers that litellm metadata confirms support structured tool calling — others get a clean prompt without tools.

### Fixed
- **Tool results lost from conversation history**: Tool calls and results are now preserved as structured messages in the conversation state (assistant+tool_calls → tool results → follow-up). Previously they were flattened to a single text blob, so providers lost tool context on subsequent turns.

## [1.12.1] - 2026-03-22

### Fixed
- **Duplicate traces in primary-only mode**: When only the primary provider responds, the fan-out text is no longer duplicated under a spurious "synthesis" phase in the provider trace and export.
- **Truncated fan-out persistence**: Persisted/exported fan-out traces now match the untruncated text the user saw live, instead of using post-truncation text from the pipeline.

## [1.12.0] - 2026-03-22

### Added
- **Provider activity traces**: Provider tabs now show the full participation trace for each provider — fan-out, synthesis, tool execution, and verification phases — instead of only the initial fan-out response. Phase boundaries are labeled with visible headers (`── Synthesis ──`, `── Tool Execution ──`, etc.).
- **Accurate primary provider lifecycle**: The primary provider tab remains in-progress through synthesis, tool execution, and verification phases, and only marks complete when all work finishes. Previously it showed "done" after fan-out while most of its work happened off-screen.
- **Provider trace persistence**: Session save/load and export now persist structured provider traces (phase + content per provider). Legacy sessions without traces load normally using the existing `Individual` summaries.
- **Trace-aware export**: Markdown export (`/export`) prefers provider traces with phase labels when available, falling back to legacy individual summaries for older sessions.

### Fixed
- **Primary tab stuck loading on pipeline failure**: When the pipeline fails before synthesis starts, the primary provider tab is now explicitly marked failed instead of remaining in a loading state indefinitely.
- **Tool-loop failure masked as success**: A tool-loop error no longer gets overwritten by an unconditional completion message — the primary tab correctly shows the failed state.
- **Delta+Done chunk content dropped**: Synthesis stream chunks carrying both content and a done signal now process the content before handling completion, fixing blank responses in single-provider mode.

## [1.11.1] - 2026-03-21

### Fixed
- **TUI freeze on `/mode` and `/save`**: `program.Send()` was called synchronously from inside Bubble Tea's `Update` via handler callbacks, deadlocking the event loop. All synchronous callback paths (`onModeChange`, `onSave`) now dispatch via goroutines. Found by Codex code review.
- **Race conditions on shared state**: `currentMode` protected by mutex with query-time snapshot; `yoloEnabled` changed from `bool` to `atomic.Bool`. Prevents stale routing/approval state when mode is changed mid-query.
- **Command palette no longer blocks typing**: Changed from modal overlay (intercepted all keys) to non-modal suggestion panel. Typing `/mode thorough` now works — text goes into the textarea normally, palette shows filtered suggestions alongside. Tab accepts the first match.

## [1.11.0] - 2026-03-21

### Added
- **Command palette**: Typing `/` opens a bordered overlay with all available commands, descriptions, and shortcuts. Supports fuzzy filtering by typing, arrow key navigation, and auto-submit for argument-free commands.
- **Reasoning effort control**: Modes now set provider-level reasoning parameters — quick (low), balanced (medium), thorough (high). Maps to Anthropic `thinking.budget_tokens`, OpenAI `reasoning_effort`, and Gemini `thinkingConfig.thinkingBudget`. Models without reasoning support silently ignore the parameter.

### Changed
- **All modes query all providers**: quick/balanced/thorough no longer control which providers are queried — all healthy providers are always fanned out. Modes now control synthesis depth (concise vs structured vs deep analysis with step-by-step reasoning, trade-offs, and cross-model verification).
- **Tool loop has no iteration limit**: The tool loop runs until the model stops issuing tool calls or the 5-minute timeout expires, rather than capping at 10 iterations.

### Fixed
- **Verification only runs after file_write**: Previously ran after every tool loop; now skips when no files were written or when the tool loop errored out.
- **Arrow-up tab bar focus**: Pressing up at the oldest input history entry now exits history mode and focuses the tab bar instead of getting stuck.

## [1.10.0] - 2026-03-21

### Added
- **Anthropic & Gemini tool support**: Both providers now handle `tool_use`/`tool_result` content blocks (Anthropic) and `functionCall`/`functionResponse` parts (Gemini). Either can serve as the primary model for tool execution.
- **Response truncation**: Fan-out responses are automatically truncated to fit the primary model's context window before synthesis, preventing context overflow with large multi-provider responses.
- **Consensus provenance display**: `ParseConsensusAnalysis()` output now wired into the TUI provenance panel (toggle with `p`). Shows confidence, agreements, minority reports, and evidence extracted from synthesis.
- **Verification after tool execution**: If a `verify_command` is configured or auto-detected (Go/Node/Rust/Python/Make), runs verification after tool loop and streams pass/fail to TUI.
- **Input history**: Up/Down arrow keys cycle through previously submitted prompts. Saves draft when entering history browsing.
- **File diff preview**: File write confirmations now show a unified diff (`+`/`-` lines) for existing files instead of raw content preview. New files still show content preview.
- **Destructive command hardening**: Added detection for pipe-to-shell (`|sh`, `|bash`), `rm-rf` (no-space), clobber (`>|`), `curl|`/`wget|` piping, and `/dev/sd*`, `/sys/`, `/proc/` paths.
- **Path validation**: `file_read` and `file_write` reject relative paths that escape the working directory via traversal attacks.
- **OAuth token auto-refresh**: Expired OAuth tokens are automatically refreshed using stored refresh tokens before queries fail with 401.
- **Cost tracking**: Per-provider estimated cost displayed in tab bar alongside token counts, calculated from litellm pricing data (`input_cost_per_token`, `output_cost_per_token`).
- **Router observability**: `SelectProvidersWithReason()` returns human-readable routing explanations shown in the provenance panel (e.g., "balanced: claude (primary) + gpt (score: 0.42)").
- **Per-provider latency telemetry**: Fan-out results now include wall-clock latency per provider, logged to telemetry for router calibration.
- **Multi-session management**: `/sessions` lists all saved sessions. `/name <name>` names the current session. `polycode session list|show|delete` CLI commands. Sessions support user-assigned names.
- **Replayable consensus traces**: Each exchange persists a `ConsensusTrace` with routing mode/reason, provider list, per-provider latencies, token usage, errors, and skipped providers. Available in session exports.
- **Context auto-summarization**: When the primary model reaches 80% context usage, early conversation turns are compressed into a dense summary preserving the last 4 messages.
- **Canonical skills**: Three example skills in `examples/skills/`: `git-review` (automated diff review), `test-runner` (detect and run project tests), `security-audit` (scan for secrets, vulnerable deps, injection patterns).
- **Example files**: `examples/permissions.yaml`, `examples/instructions.md`, and `examples/skill-manifest.yaml` templates for new users.
- **OpenAI-compat token tracking**: `stream_options: {include_usage: true}` added to OpenAI-compatible provider, enabling token tracking for Ollama, vLLM, OpenRouter, etc.

### Fixed
- **Goroutine leak in consensus synthesis**: Channel sends in `Synthesize()` now use `select` with context cancellation. Same fix applied to all 5 channel sends in the tool loop.
- **Atomic session/checkpoint writes**: `SaveSession()` and `SaveCheckpoint()` now write to temp files then atomically rename, preventing corruption from crashes during write.

### Changed
- **Code modernization**: `interface{}` → `any` throughout (67 replacements across 11 files). `sb.WriteString(fmt.Sprintf(...))` → `fmt.Fprintf(&sb, ...)`. `for i := 0; i < N; i++` → `for i := range N`. `HasPrefix + TrimPrefix` → `CutPrefix`. `maps.Copy` replaces manual loops. Dead code removed (`var _ = json.Marshal`, unused methods `hasContent`, `prevWizardStep`). Silent `_ = SaveSession(...)` calls replaced with logged errors.

## [1.9.0] - 2026-03-20

### Added
- **Runtime subsystem integration**: Hooks (pre_query, post_query, post_tool, on_error), permission policies, mode-based routing, repo memory, instruction hierarchy, and MCP tools are now wired into the main conversation loop — previously these existed as packages but were not active at runtime.
- **Skills/plugin system**: Installable extensions that add slash commands, system prompts, and tool definitions. Skills live in `~/.config/polycode/skills/` with YAML manifests. CLI: `polycode skill list|install|remove`. TUI: `/skill` command. Completes Phase 5 of the roadmap.
- **Adaptive router calibration**: User feedback signals (tool accept/reject) logged as telemetry and factored into provider scoring. Periodic full-consensus calibration in quick mode every 10th query. Providers selected per query instead of statically.
- **Live markdown rendering**: Streaming output is re-rendered through glamour every 500ms, so users see formatted headers, code blocks, and lists as the response arrives.
- **Eval framework**: `evals/` directory with 6 golden task tests (file read/write, shell exec, fix-and-test, consensus pipeline, timeout behavior) and 10 seeded review benchmark cases (SQL injection, hardcoded creds, race conditions, etc.).
- **Test coverage for previously untested packages**: 21 provider tests (SSE parsing, auth headers, tool call accumulation), 18 auth tests (MemStore, fileStore, concurrent access), 23 TUI tests (message handling, state transitions, confirmation flow), 20 skill tests.
- **Session fidelity**: Tool calls and tool results now round-trip through session save/restore via `toSessionMessages`/`fromSessionMessages`.

### Fixed
- **CORS origin validation**: Editor bridge now parses the Origin URL and checks exact hostname, preventing spoofed origins like `http://localhost.evil.com`.
- **Editor bridge binds to loopback**: `polycode serve` defaults to `127.0.0.1` instead of all interfaces.
- **CI severity detection**: `ReviewHasCritical` uses structured severity markers and negation filtering instead of naive substring matching. "No critical issues found" no longer triggers a false positive.
- **Permission check per tool**: `ConfirmFunc` now receives the actual tool name for each call, so multi-tool responses get per-tool policy checks instead of inheriting the first tool's policy.
- **Tool context fix**: Consensus text and tool execution output combined into a single assistant message for coherent multi-turn conversations.
- **TUI rendering performance**: Chat log cached and rebuilt only on history changes. Markdown rendered once per exchange instead of on every View() frame. Eliminates lag when scrolling through long conversations.
- **Router telemetry**: Provider responses now logged with `Success` field so the adaptive router scores providers correctly.

### Changed
- `ConfirmFunc` signature changed from `func(description string) bool` to `func(toolName, description string) bool` for per-tool permission checks.
- System prompt now built from instruction hierarchy (`.polycode/instructions.md` > `~/.config/polycode/instructions.md` > default) plus repo memory, instead of a hardcoded string.
- Provider selection happens per query via the adaptive router, not from a static pipeline rebuilt only on mode/config changes.

## [1.8.0] - 2026-03-19

### Added
- **Native tool call protocol**: `provider.Message` now supports `ToolCalls`, `ToolCallID`, and `RoleTool` for correct OpenAI-compatible tool continuation. All provider adapters updated.
- **Live tool output streaming**: Tool execution results (truncated to 500 chars) display inline as fenced code blocks in the consensus stream. Follow-up model responses stream live instead of buffering.
- **Status chunks**: `StreamChunk.Status` flag distinguishes progress/tool output from model text — status is displayed but not persisted to conversation history.

### Fixed
- **Tool execution now works end-to-end**: Fixed conversation threading (tool loop gets synthesis context, not raw chat history), separate 5-minute timeout for tool loop, native tool messages instead of fake user text.
- **Ghost tool calls filtered**: SSE parser skips empty tool call buffers created when providers index tool calls starting at 1 instead of 0.
- **Duplicate assistant tool_call messages**: Multi-iteration tool loops no longer double-append the assistant message with tool calls.
- **`content: null` for tool-call-only messages**: `openaiMsg.Content` is now `*string`, serializing as `null` when empty with tool_calls present (required by OpenAI API).
- **Fan-out no longer sends tools to individual providers**: Tools are stripped from fan-out queries so providers respond with text analysis instead of empty tool-call-only responses.
- **Viewport overflow**: Improved height calculations so the input area stays visible.
- **Empty provider tabs explained**: Providers that respond with tool calls only now show an explanatory message instead of a blank panel.

## [1.7.0] - 2026-03-19

### Added
- **Yolo mode as a toggle**: `/yolo` toggles auto-approve independently of the consensus mode. Tab bar shows `[balanced|yolo]` when both are active. Also accessible from the mode picker dropdown.
- **Mode picker dropdown**: Navigate to the mode badge in the tab bar (↑ then ← to highlight `[balanced]`, Enter to open). Shows quick/balanced/thorough with descriptions plus a yolo checkbox toggle.
- **Mode badge selectable in tab bar**: The mode indicator is now a navigable position (activeTab = -1) in the tab bar — press Enter to open the mode picker.

## [1.6.2] - 2026-03-19

### Added
- **Focusable tab bar navigation**: Press ↑ (when input is empty) to focus the tab bar, then ←/→ to switch tabs. Press ↓, Enter, or Esc to return focus to input. Tab bar shows visual focus indicator (▸ prefix, underlined active tab, blurred textarea).

### Fixed
- **Arrow keys no longer hijack textarea**: Left/right arrows only switch tabs when the tab bar is focused, not when typing.
- **Removed grey cursor line highlight** from the textarea input.

## [1.6.1] - 2026-03-19

### Fixed
- **Typing `p`, `?`, and arrow keys in input no longer triggers shortcuts**: Single-character shortcuts (`p` for provenance, `?` for help) now only fire when the textarea is empty. Left/right arrow keys move the cursor; use **Shift+←/→** to switch tabs.

### Added
- **Per-directory session scoping**: Chat sessions are now stored per working directory. Starting polycode in different project folders resumes separate conversations. Sessions live in `~/.config/polycode/sessions/<hash>.json`.

## [1.6.0] - 2026-03-19

### Added
- **Tabbed TUI interface**: Each provider gets its own tab. The Consensus tab (default) shows the synthesized response, while provider tabs show individual responses. Navigate with ←/→ arrow keys.
- **Unified tab bar**: App title, mode, provider status icons, and token usage are all shown in a single tab bar — replaces the separate status bar.
- **Scrollable chat viewport**: PgUp/PgDn, Ctrl+U/Ctrl+D, Home/End for keyboard scrolling. Mouse scroll wheel enabled via `WithMouseCellMotion`.
- **Output height constrained**: Chat view no longer overflows the terminal height.

### Fixed
- **Provider tab content now populated**: `ProviderChunkMsg` with `Done: true` was discarding the `Delta` content. Individual provider responses now appear in their tabs.

## [1.5.3] - 2026-03-18

### Added
- **Inline slash command autocomplete**: Matching commands appear as hints above the input as you type (e.g., typing `/cl` shows `/clear`). Press Tab to accept the highlighted completion.

## [1.5.2] - 2026-03-18

### Fixed
- **Fix panic: "strings: illegal use of non-zero Builder copied by value"**: `strings.Builder` fields in the TUI model were copied by value when Bubble Tea copies the model on each Update. Changed `consensusContent` and `ProviderPanel.Content` to pointer types (`*strings.Builder`) to avoid the copy-after-use panic.

### Added
- **Edit base URL in config editor**: `polycode config edit` now shows a "Change base URL" option for providers that have one configured

## [1.5.1] - 2026-03-18

### Added
- **Rename provider in config editor**: `polycode config edit` now has a "Rename" option that migrates credentials to the new name
- **`polycode auth logout` interactive picker**: Running without args shows a selectable list of providers instead of requiring the exact name

### Fixed
- **`polycode auth logout` no longer errors on missing credentials**: "Credential not found" is now treated as success (already removed) instead of a hard error

## [1.5.0] - 2026-03-18

### Added
- **Slash command autocomplete**: Type `/` and press Tab to cycle through matching commands (e.g., `/cl` + Tab → `/clear`)
- **`polycode config edit`**: Interactive config editor — add, remove, or edit providers (change model, API key, primary designation) from the command line
- **`polycode config show`**: Print current configuration summary
- **`polycode config path`**: Print config file location
- **Prominent error display**: Provider and consensus errors now show as bold red text in the main chat area instead of being hidden in provider panels
- **Version-aware base URL handling**: OpenAI-compatible providers with versioned base URLs (e.g., `/v4`) no longer get `/v1` doubled in the request path

## [1.4.1] - 2026-03-18

### Fixed
- **TUI chat submission now works**: `tea.NewProgram(model)` was called before `Set*Handler()`, so Bubble Tea ran a copy of the model with all callbacks nil. Pressing Enter cleared the input but never sent a query. Fixed by moving `NewProgram` after all handler wiring so the model copy has all callbacks set.

## [1.4.0] - 2026-03-18

### Added
- **Multi-provider wizard loop**: `polycode init` now asks "Add another provider?" after each provider, letting users configure multiple providers in one session. Auto-sets `min_responses` based on provider count.
- **`polycode provider add` command**: Add a new provider to an existing config without re-running init. Asks about primary designation and auto-bumps `min_responses`.
- **Slash commands**: `/help`, `/save`, `/exit`, `/quit`, `/export [path]` added to the TUI chat input alongside existing `/clear`, `/settings`, `/mode`, `/memory`, `/plan`.
- **"Thinking..." indicator**: Chat area shows an animated spinner with "Thinking..." while waiting for the first response chunk from providers.
- **Status bar phase indicator**: Shows "querying providers..." while providers are processing and "synthesizing..." during consensus, replacing the generic "querying..." label.
- **"Edit base URL" on connection failure**: `openai_compatible` providers now show an "Edit base URL" option in the connection test failure menu.

### Fixed
- **Connection test passes base URL**: `openai_compatible` connection tests no longer fail with "unsupported protocol scheme" due to missing base URL.
- **Lightweight connection test for openai_compatible**: Uses `GET /models` instead of a chat completion, avoiding 404s from unrecognized placeholder model names.

## [1.3.2] - 2026-03-18

### Fixed
- **Remove broken OAuth defaults for Anthropic and OpenAI**: These providers do not support OAuth for third-party apps. The wizard no longer offers `oauth` as an auth method for Anthropic or OpenAI — use `api_key` instead. Google retains OAuth support (requires explicit `oauth_client_id` in config).
- **Clear error when OAuth is misconfigured**: Selecting `auth: oauth` without providing `oauth_client_id`/`oauth_device_url`/`oauth_token_url` now gives an actionable error message instead of a circular "run polycode auth login" loop.

## [1.3.1] - 2026-03-18

### Fixed
- **`polycode auth login` now actually authenticates**: Previously was a stub that printed "Authentication successful" without running any auth flow. Now creates the provider and calls `Authenticate()`, which triggers the OAuth device flow or API key lookup as appropriate.
- **`polycode auth logout` now removes credentials**: Previously was a stub. Now calls `store.Delete()` to remove the credential from the OS keyring.

## [1.3.0] - 2026-03-18

### Added
- **Endpoint model discovery for OpenAI-compatible providers**: The setup wizard now queries `GET /models` on the configured base URL to discover available models, with automatic `/v1` path fallback
- Discovered models are cross-referenced against litellm metadata to show capability info (context window, tools, vision, reasoning)
- Wizard step reordering for `openai_compatible`: base URL is now collected before auth and model selection to enable discovery

### Changed
- `selectModel()` now accepts base URL and API key parameters for endpoint discovery

## [1.2.1] - 2026-03-17

### Fixed
- **litellm model matching**: Provider prefix matching now uses the actual litellm key format — bare keys for Anthropic (`claude-*`) and OpenAI (`gpt-*`, `o1*`, `o3*`), dot-prefixed for Bedrock (`anthropic.*`), and slash-prefixed for Gemini (`gemini/`). Previously returned zero models for Anthropic and OpenAI.

## [1.2.0] - 2026-03-17

### Added
- **Interactive selectable inputs for CLI setup wizard**: Replaced plain text input with arrow-key navigable `huh.Select` lists for provider type, auth method, and connection test recovery
- Filterable model picker pre-populated from litellm metadata with inline capabilities display
- "Custom model..." escape hatch for manual model name entry
- Password-masked API key input via `huh.Input`
- Added `charmbracelet/huh` dependency (Charm ecosystem forms library)

## [1.1.0] - 2026-03-17

### Added
- Smart wizard: litellm model browsing with numbered list, filtered auth methods by provider type, connection testing with retry/skip/quit
- ASCII logo banner on `polycode init` and CLI commands

## [1.0.0] - 2026-03-17

### Added
- Workflow platform and team adoption features (Phase 5)
- Adaptive routing and repo memory (Phase 4)
- Consensus-native agent teams (Phase 3)
- Evidence-backed consensus review (Phase 2)
- Execution core and eval harness (Phase 1)

## [0.2.0]

### Added
- `/clear` command, markdown rendering, and auto-resumable sessions
- 5-phase product roadmap

## [0.1.1]

### Fixed
- Module path corrected to match repo URL
- Removed AI editor config dirs from repo

## [0.1.0]

### Added
- Initial release: multi-model consensus coding assistant TUI
- Anthropic, OpenAI, Google Gemini, and OpenAI-compatible provider support
- Fan-out consensus pipeline with primary model synthesis
- Bubble Tea TUI with streaming provider panels
- Tool execution (file read/write, shell exec)
- OS keyring credential storage with encrypted file fallback
- CI/CD pipeline with GoReleaser cross-platform builds
