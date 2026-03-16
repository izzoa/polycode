## 1. Project Scaffolding

- [x] 1.1 Initialize Go module (`go mod init`) and set up directory structure: `cmd/polycode/`, `internal/provider/`, `internal/consensus/`, `internal/tui/`, `internal/config/`, `internal/auth/`, `internal/action/`
- [x] 1.2 Add core dependencies: Bubble Tea, Lip Gloss, `go-keyring`, `gopkg.in/yaml.v3`, and HTTP client libraries
- [x] 1.3 Create `main.go` entry point with CLI argument parsing (root command, `auth` subcommands) using Cobra or similar

## 2. Configuration System

- [x] 2.1 Define config structs: `Config`, `ProviderConfig`, `ConsensusConfig`, `TUIConfig` with YAML tags
- [x] 2.2 Implement config loader: read from `~/.config/polycode/config.yaml` with XDG fallback, validate required fields, enforce exactly one primary provider
- [x] 2.3 Implement config validation: check for missing fields, duplicate names, multiple primaries, and unknown provider types
- [x] 2.4 Create interactive setup wizard that runs when no config file exists (prompt for at least one provider)

## 3. Provider Interface & Adapters

- [x] 3.1 Define `Provider` interface with `ID()`, `Query()`, `Authenticate()`, `Validate()` methods and shared types (`Message`, `StreamChunk`, `QueryOpts`)
- [x] 3.2 Implement Anthropic Claude adapter: API key and OAuth auth, streaming chat completions via Anthropic API
- [x] 3.3 Implement OpenAI adapter: API key auth, streaming chat completions via OpenAI API
- [x] 3.4 Implement Google Gemini adapter: API key and OAuth auth, streaming via Gemini API
- [x] 3.5 Implement OpenAI-compatible adapter: configurable base URL, API key or no-auth, streaming chat completions
- [x] 3.6 Implement provider registry that instantiates adapters from config and exposes list of healthy providers

## 4. Authentication System

- [x] 4.1 Implement OS keyring storage: store/retrieve/delete API keys and OAuth tokens using `go-keyring`
- [x] 4.2 Implement encrypted file fallback for when keyring is unavailable
- [x] 4.3 Implement OAuth 2.0 device authorization flow: initiate flow, display URL+code, poll for token, store tokens
- [x] 4.4 Implement OAuth token refresh logic: detect expired access token, use refresh token, re-initiate flow if refresh token expired
- [x] 4.5 Implement CLI auth subcommands: `polycode auth login <provider>`, `polycode auth logout <provider>`, `polycode auth status`

## 5. Fan-Out Query System

- [x] 5.1 Implement fan-out dispatcher: launch goroutines for each healthy provider, send user prompt concurrently
- [x] 5.2 Implement response collector: aggregate streaming chunks per provider, track completion status, enforce timeout
- [x] 5.3 Implement error isolation: catch per-provider errors without affecting other providers, log warnings
- [x] 5.4 Implement minimum response threshold: wait for `min_responses` before allowing consensus to proceed, handle fallback when threshold not met

## 6. Consensus Engine

- [x] 6.1 Implement consensus prompt builder: construct structured prompt with original user question + all labeled provider responses
- [x] 6.2 Implement context window overflow detection and proportional truncation of long responses
- [x] 6.3 Implement consensus synthesis call: send consensus prompt to primary model with tool definitions, stream response back
- [x] 6.4 Handle edge cases: single-provider fallback (skip synthesis), all-non-primary-fail fallback, primary-only response

## 7. Action Execution

- [x] 7.1 Define tool schemas for `file_read`, `file_write`, `shell_exec` in the format expected by the primary model's API
- [x] 7.2 Implement file read tool: read file contents, return to conversation context
- [x] 7.3 Implement file write tool: generate diff, display in TUI, wait for user confirmation, apply changes
- [x] 7.4 Implement shell exec tool: display command, wait for user confirmation, execute with timeout, capture stdout/stderr
- [x] 7.5 Implement safety guardrails: detect destructive commands, require explicit confirmation, block without consent
- [x] 7.6 Implement tool-use loop: parse tool calls from primary model response, execute, feed results back to primary

## 8. TUI Implementation

- [x] 8.1 Set up Bubble Tea application structure: Model, Update, View with Lip Gloss styling
- [x] 8.2 Implement text input component for user prompts with multi-line support (Shift+Enter for newline)
- [x] 8.3 Implement provider response panels: labeled, scrollable panels per provider with streaming token display
- [x] 8.4 Implement status indicators per provider panel: loading spinner, checkmark (done), X (failed/timed out)
- [x] 8.5 Implement consensus output panel with visual emphasis (highlighted border, distinct label)
- [x] 8.6 Implement provider status bar: list all providers with health indicators and primary marker
- [x] 8.7 Implement conversation history: scrollable list of previous prompt/response pairs
- [x] 8.8 Implement keyboard navigation: Ctrl+C quit, Tab toggle individual/consensus view, arrow key scroll, copy support
- [x] 8.9 Implement file diff display and confirmation prompts for action execution within the TUI

## 9. Integration & Testing

- [x] 9.1 Write unit tests for config parsing and validation
- [x] 9.2 Write unit tests for consensus prompt building and truncation logic
- [x] 9.3 Write unit tests for fan-out dispatcher and response collector (using mock providers)
- [x] 9.4 Write integration test: end-to-end query with mock providers → consensus → action execution
- [ ] 9.5 Manual smoke test with real providers (Anthropic, OpenAI, Gemini)

## 10. Polish & Distribution

- [x] 10.1 Add `--version` flag and build metadata
- [x] 10.2 Create Goreleaser config for cross-platform binary builds (macOS, Linux, Windows)
- [x] 10.3 Write README with installation instructions, configuration guide, and usage examples
