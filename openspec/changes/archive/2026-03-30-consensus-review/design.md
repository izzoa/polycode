## Context

The current consensus pipeline in `internal/consensus/` works in three steps: fan-out to all providers, collect raw text responses, synthesize via the primary model with a prompt that says "analyze all responses and produce the best synthesis." The output is unstructured prose. Users see only the final answer — not which models agreed, disagreed, or what evidence was considered.

Phase 1 (execution-core) wired tool execution into this pipeline. Phase 2 enriches the *quality* of the consensus itself.

## Goals / Non-Goals

**Goals:**
- Parse provider responses into a structured envelope before synthesis
- Give the primary model richer synthesis instructions that produce minority reports
- Show agreement/disagreement provenance in the TUI
- Add a `polycode review` command for code review workflows
- Optional post-consensus verification (run tests/lint on proposed changes)

**Non-Goals:**
- Requiring all providers to return JSON (we parse structure from prose)
- Replacing the current synthesis approach (we enhance it)
- Automated PR merging or CI gating (review produces output, doesn't take action beyond commenting)
- Training or fine-tuning models for better review quality

## Decisions

### 1. Response envelope: extracted by the primary, not by each provider

**Choice**: Rather than asking each provider to output structured JSON (unreliable across diverse models), the consensus prompt asks the primary model to *analyze* the raw responses and produce a structured synthesis that includes: the recommended action, confidence level (high/medium/low), areas of agreement, minority positions, and key evidence.

```
Analyze all responses and produce a synthesis with this structure:

## Recommendation
[Your synthesized answer]

## Confidence: [high/medium/low]

## Agreement
[Points where all or most models agree]

## Minority Report
[Dissenting views worth considering, with the model name and reasoning]

## Evidence
[Key facts, code references, or documentation cited by any model]
```

**Rationale**: Asking the primary model to extract structure is far more reliable than hoping every provider returns valid JSON. The primary model is the one we trust most — that's why it's the synthesizer.

### 2. Minority reports: parsed from synthesis output

**Choice**: After synthesis, parse the primary's response for `## Minority Report` and `## Confidence` sections. Store these as structured data on the TUI model for display in an expandable provenance panel.

**Rationale**: Simple regex/string parsing from the structured prompt output. No extra API calls needed.

### 3. `polycode review`: headless fan-out on git diff

**Choice**: `polycode review [--pr <url>]` runs outside the TUI:
1. Gets the diff via `git diff` (or `gh pr diff <number>`)
2. Constructs a review prompt with the diff content
3. Fans out to all providers
4. Synthesizes a consensus review (with the structured prompt)
5. Outputs the review to stdout (or posts as PR comment with `--pr`)

**Rationale**: Code review is the highest-value use case for multi-model consensus. A headless command makes it CI-friendly. PR commenting via `gh` requires no additional auth.

### 4. Verifier lane: opt-in post-consensus check

**Choice**: After consensus produces a file change, if the config has `consensus.verify: true`, polycode runs a configurable verify command (default: `go test ./...` or detected from the repo) and includes the result in the final output. If tests fail, the failure is shown alongside the consensus.

Config:
```yaml
consensus:
  verify: true
  verify_command: "go test ./..."  # auto-detected if omitted
```

**Rationale**: The verifier adds a concrete reliability signal. It's opt-in because not every prompt produces testable output. Auto-detection looks for `go.mod`, `package.json`, `Makefile`, etc.

### 5. Provenance panel in TUI: toggleable with `p`

**Choice**: Pressing `p` in the chat view toggles a provenance panel below the consensus output showing: confidence level, agreement summary, and minority report (if any). This reuses the existing panel rendering infrastructure.

**Rationale**: Provenance should be visible but not intrusive. Toggle keeps the default view clean.

## Risks / Trade-offs

- **Structured prompt increases token usage**: The enhanced synthesis prompt is longer. → **Mitigation**: The extra tokens are small relative to the N provider responses already in the prompt.

- **Parsing structured output from prose is fragile**: If the primary model doesn't follow the format exactly, parsing may miss sections. → **Mitigation**: Graceful degradation — if sections aren't found, show raw consensus as before.

- **PR commenting requires `gh` CLI**: Not everyone has it installed. → **Mitigation**: Check for `gh` availability, show a clear error with install instructions if missing.
