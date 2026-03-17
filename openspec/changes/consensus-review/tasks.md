## 1. Structured Consensus Prompt

- [x] 1.1 Update `BuildConsensusPrompt()` in `internal/consensus/consensus.go` to use the enhanced structured prompt template requesting: Recommendation, Confidence (high/medium/low), Agreement, Minority Report, Evidence sections
- [x] 1.2 Create `internal/consensus/review.go` with types: `ConsensusAnalysis` (Recommendation, Confidence, Agreements []string, MinorityReports []MinorityReport, Evidence []string), `MinorityReport` (ProviderID, Position, Reasoning)
- [x] 1.3 Implement `ParseConsensusAnalysis(rawOutput string) *ConsensusAnalysis` — extract structured sections from the primary model's synthesis output using header-based parsing
- [x] 1.4 Handle graceful degradation: if structured sections aren't found in the output, return a `ConsensusAnalysis` with the full text as Recommendation and empty structured fields

## 2. Minority Reports & Provenance TUI

- [x] 2.1 Add `consensusAnalysis *ConsensusAnalysis` field to the TUI model (imported from consensus package via a TUI-local type or interface)
- [x] 2.2 After consensus completes in the submit handler, parse the synthesis output and send a `ConsensusAnalysisMsg` to the TUI
- [x] 2.3 Add `showProvenance bool` field to the TUI model, toggled by `p` key in chat mode
- [x] 2.4 Implement `renderProvenance()` in view.go — displays confidence indicator (green/yellow/red), agreement summary, and minority reports if present
- [x] 2.5 Add `p` key handling in `updateChat()` to toggle provenance panel
- [x] 2.6 Add confidence indicator to the consensus panel header (e.g., "◆ Consensus [high confidence]")

## 3. Verifier Lane

- [x] 3.1 Add `Verify bool` and `VerifyCommand string` fields to `ConsensusConfig` in config.go
- [x] 3.2 Implement `DetectVerifyCommand(workDir string) string` in `internal/action/verify.go` — checks for go.mod, package.json, Makefile, Cargo.toml and returns the appropriate test command
- [x] 3.3 After tool execution applies a file change and `consensus.verify` is true, run the verify command and capture output
- [x] 3.4 If verification fails, send the failure output back to the primary model as context for a follow-up response
- [x] 3.5 Display verification result in the TUI: "✓ Verification passed" or "✕ Verification failed" with output

## 4. Review Command

- [x] 4.1 Add `review` subcommand to Cobra in `cmd/polycode/main.go` with flags: `--pr <number>`, `--comment`, `-- <files...>`
- [x] 4.2 Create `cmd/polycode/review.go` with `runReview()` implementation
- [x] 4.3 Implement diff acquisition: `git diff --cached` (or `git diff` if nothing staged), or `gh pr diff <number>` for PRs
- [x] 4.4 Construct a review-specific prompt: "Review the following code changes. For each issue found, specify severity (critical/warning/info), file location, and description."
- [x] 4.5 Run the fan-out + consensus pipeline headlessly (no TUI) using the review prompt + diff content
- [x] 4.6 Format the consensus review output with sections: Summary, Issues (severity + location + description), Suggestions, Assessment
- [x] 4.7 Output to stdout by default; if `--comment` flag with `--pr`, post via `gh pr comment`
- [x] 4.8 Check for `gh` availability before PR operations; show install instructions if missing

## 5. Testing & Benchmarks

- [x] 5.1 Unit test: `ParseConsensusAnalysis` correctly extracts all sections from well-formed synthesis output
- [x] 5.2 Unit test: `ParseConsensusAnalysis` gracefully degrades when sections are missing
- [x] 5.3 Unit test: `DetectVerifyCommand` returns correct commands for go.mod, package.json, Makefile, and unknown projects
- [x] 5.4 Unit test: review prompt construction includes the diff content
- [x] 5.5 Create a benchmark fixture: seeded diff with a known bug → verify the structured review catches it across mock providers
