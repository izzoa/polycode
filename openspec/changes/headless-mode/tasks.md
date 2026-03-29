## 1. Pipeline Extraction

- [ ] 1.1 Define `PipelineOptions` struct: ToolMode (auto/confirm/none), ProviderOverride, Writer io.Writer, Context
- [ ] 1.2 Extract pipeline logic from TUI into `consensus.RunPipeline(ctx, cfg, prompt, opts) (string, tokens.TokenUsage, error)`
- [ ] 1.3 RunPipeline builds providers from config, dispatches, collects, synthesizes
- [ ] 1.4 Stream consensus chunks to opts.Writer when non-nil
- [ ] 1.5 Support single-provider mode: skip consensus when ProviderOverride is set
- [ ] 1.6 Support no-tools mode: suppress tool dispatch when ToolMode is "none"

## 2. TUI Integration

- [ ] 2.1 Refactor TUI query initiation to call RunPipeline with a message-emitting writer
- [ ] 2.2 Writer emits ConsensusChunkMsg for each chunk, QueryDoneMsg on completion
- [ ] 2.3 Verify all existing TUI streaming behavior preserved after refactor

## 3. Headless Run Command

- [ ] 3.1 Rewrite `cmd/polycode/run.go` with Cobra command: `polycode run "prompt"`
- [ ] 3.2 Add `--confirm` flag (default false): enable interactive tool approval
- [ ] 3.3 Add `--provider` flag: single-provider mode
- [ ] 3.4 Add `--no-tools` flag: disable tool execution
- [ ] 3.5 Load config from standard path, validate, build pipeline options
- [ ] 3.6 Call RunPipeline with os.Stdout as writer
- [ ] 3.7 Implement exit codes: 0 success, 1 error, 2 tool rejected

## 4. Interactive Confirmation (--confirm mode)

- [ ] 4.1 Implement stderr-based tool confirmation: print prompt to stderr, read y/n from stdin
- [ ] 4.2 On rejection: return exit code 2
- [ ] 4.3 Wire confirm function into PipelineOptions

## 5. Verification

- [ ] 5.1 Run `go build ./...` and `go test ./...` -- all pass
- [ ] 5.2 Test: `polycode run "hello"` streams response to stdout
- [ ] 5.3 Test: `polycode run "hello" | wc -l` works (piping)
- [ ] 5.4 Test: `polycode run --no-tools "hello"` produces output without tool calls
- [ ] 5.5 Test: bad config produces stderr error and exit code 1
- [ ] 5.6 Manual: verify TUI still works identically after pipeline extraction
