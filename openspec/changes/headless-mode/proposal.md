## Why

Polycode can only be used interactively through its TUI. There is no way to integrate it into CI pipelines, shell scripts, or automation workflows. Users who want multi-model consensus on a one-shot prompt must launch the full TUI, type the query, wait for results, and manually copy the output. A headless `polycode run "prompt"` command unlocks scripting, CI integration, and composability with other tools.

## What Changes

- Implement `polycode run "prompt"` as a non-interactive command that streams consensus output to stdout
- Reads config from the standard `~/.config/polycode/config.yaml`
- Tool execution defaults to auto-approve (yolo mode) for scripting; `--confirm` flag enables interactive prompts on stderr
- Exit codes: 0 for success, 1 for error, 2 for rejected tool call
- Extracts the query pipeline from the TUI layer into a reusable `RunPipeline()` function that both the TUI and headless mode call
- Supports `--provider` flag to query a single provider (skip consensus)
- Supports `--no-tools` flag to disable tool execution entirely

## Capabilities

### New Capabilities
- `headless-runner`: Non-interactive run command with stdout streaming, exit codes, and CLI flags
- `pipeline-extraction`: Reusable query pipeline function extracted from TUI-coupled code

### Modified Capabilities
<!-- No existing spec-level changes -- TUI continues using the extracted pipeline -->

## Impact

- **Files modified** (1): `cmd/polycode/app.go` (extract pipeline)
- **Files rewritten** (1): `cmd/polycode/run.go` (from stub to full implementation)
- **Dependencies**: None new
- **Scope**: ~300-500 lines
