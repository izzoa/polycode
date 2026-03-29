## Context

Polycode's query pipeline is currently embedded in the TUI layer. `consensus.Pipeline.Run()` dispatches to providers and returns results via Bubble Tea messages (`ProviderChunkMsg`, `ConsensusChunkMsg`, `QueryDoneMsg`). The TUI Model owns the pipeline instance and handles its lifecycle. There is a stub `cmd/polycode/run.go` file that is not yet functional. The pipeline itself is provider-agnostic and could operate without a TUI if decoupled from Bubble Tea message passing.

## Goals / Non-Goals

**Goals:**
- `polycode run "prompt"` streams consensus output to stdout
- Works with existing config file -- no separate config
- Sensible defaults for scripting: auto-approve tools, no TUI chrome
- Structured exit codes for scripting (0/1/2)
- Extract pipeline into a reusable function callable by both TUI and headless
- `--confirm` flag for interactive tool approval via stderr prompts
- `--provider` flag for single-provider mode (skip consensus)
- `--no-tools` flag to disable tool execution

**Non-Goals:**
- Multi-turn conversation in headless mode (single prompt, single response)
- Streaming individual provider responses (only consensus output)
- JSON output format (plain text to stdout; JSON is a future enhancement)
- Session persistence for headless runs
- MCP tool execution in headless mode (built-in tools only for now)

## Decisions

### 1. Extract RunPipeline() into consensus package

**Decision**: Create `consensus.RunPipeline(ctx, config, prompt, opts) (string, TokenUsage, error)` that encapsulates: build providers, dispatch, collect, synthesize. Returns the final consensus text. The TUI calls this internally; headless calls it directly.

**Rationale**: The pipeline logic is already in the consensus package. Extracting it into a top-level function is a natural refactor. The TUI adapts by wrapping RunPipeline with Bubble Tea message emission.

### 2. Streaming via io.Writer callback

**Decision**: RunPipeline accepts an optional `io.Writer` for streaming chunks. Headless passes `os.Stdout`. TUI passes a writer that emits `ConsensusChunkMsg`.

**Rationale**: io.Writer is the standard Go streaming interface. Both callers can provide their own writer without coupling to Bubble Tea.

### 3. Tool confirmation via stderr in confirm mode

**Decision**: In headless `--confirm` mode, tool approval prompts are printed to stderr and user input is read from stdin. This keeps stdout clean for piping.

**Rationale**: Standard Unix convention: interactive prompts on stderr, data on stdout.

### 4. Exit codes follow convention

**Decision**: Exit 0 = success, Exit 1 = runtime error (provider failure, config error), Exit 2 = tool call rejected by user (in --confirm mode).

**Rationale**: Distinct exit codes enable scripting: `if polycode run "fix lint" ; then ...`. Code 2 specifically signals that the human intervened.

## Risks / Trade-offs

- **[Risk] Pipeline extraction breaks TUI streaming** -> Mitigation: TUI wraps RunPipeline with a message-emitting writer; streaming behavior preserved.
- **[Risk] Auto-approve tools in scripting is dangerous** -> Mitigation: Default yolo mode only runs when explicitly using `run` command. Document the risk. `--no-tools` is available.
- **[Risk] Config validation fails in CI (no providers configured)** -> Mitigation: Clear error message on stderr with exit code 1.
