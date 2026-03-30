## Why

Polycode's consensus currently works like a summary: all provider responses go to the primary model, which produces a single merged answer. The user has no way to see *why* that answer was chosen, where models disagreed, what evidence each model cited, or how confident the consensus is. This makes the consensus opaque — no better than trusting a single model. Phase 2 turns consensus into a verifiable reliability feature by adding structured reasoning, minority reports, verification, and a dedicated `polycode review` command.

## What Changes

- **Structured response envelope**: Each provider's response is parsed into a schema with `proposed_action`, `evidence`, `assumptions`, `confidence`, and `disagreements` — giving the synthesis step richer input than raw prose
- **Updated consensus prompt**: The synthesis prompt requests structured analysis, explicitly asking the primary to identify agreement areas, disagreements, minority positions worth preserving, and confidence level
- **Minority reports**: When providers disagree, the TUI surfaces the dissenting view alongside the consensus so users can evaluate both
- **Verifier lane**: After consensus proposes a code change, polycode can optionally run tests, lint, or a security-focused review pass before presenting the result
- **`polycode review` CLI command**: A headless code review mode that takes a `git diff` or GitHub PR URL, fans out to all providers, and produces a structured consensus review
- **Review provenance in TUI**: A new expandable panel showing which models agreed/disagreed and key evidence cited
- **Review benchmarks**: A seeded test suite to measure review quality

## Capabilities

### New Capabilities
- `structured-consensus`: Structured response envelope, enhanced synthesis prompt, minority reports, confidence scoring
- `verifier-lane`: Post-consensus verification via tests, lint, or security review
- `review-command`: `polycode review` CLI subcommand for code review of diffs and PRs

### Modified Capabilities
_(none — the existing consensus pipeline is enhanced, not replaced)_

## Impact

- **`internal/consensus/consensus.go`**: New structured prompt, response envelope parsing
- **`internal/consensus/review.go`**: New file — structured review types, minority report extraction
- **`cmd/polycode/main.go`**: New `review` subcommand
- **`cmd/polycode/review.go`**: New file — review command implementation
- **`internal/tui/view.go`**: New provenance panel for consensus details
- **`internal/tui/model.go`**: New fields for minority reports and agreement data
