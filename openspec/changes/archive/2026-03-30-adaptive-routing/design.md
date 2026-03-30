## Context

Polycode has telemetry logging (Phase 1) that captures per-provider latency, token counts, and errors to `~/.config/polycode/telemetry.jsonl`. This data exists but isn't used for anything yet. The router will consume it. The consensus pipeline already accepts a provider list — routing just changes which providers are in that list.

## Goals / Non-Goals

**Goals:**
- Three operating modes with different cost/quality trade-offs
- Heuristic routing that improves provider selection over time
- Persistent repo memory for project-specific knowledge
- Instruction hierarchy for customizing polycode's behavior per repo/user
- TUI integration for mode switching and memory inspection

**Non-Goals:**
- ML-based routing (heuristics only for v1)
- Automatic instruction generation from code analysis
- Cloud-synced memory across machines
- Per-file or per-function granularity in routing decisions

## Decisions

### 1. Operating modes: enum on the conversation state

**Choice**: A `Mode` enum with three values:

| Mode | Providers Used | Consensus | Verifier |
|------|---------------|-----------|----------|
| `quick` | Primary only | No | No |
| `balanced` | Primary + best secondary | Yes | No |
| `thorough` | All healthy | Yes | If configured |

The mode is stored on the conversation state and can be changed mid-session via `/mode quick`, `/mode balanced`, or `/mode thorough`. Default is `balanced`. The status bar shows the current mode.

**Rationale**: Explicit modes give users direct control over cost vs quality. No surprises.

### 2. Router: telemetry-based heuristic scorer

**Choice**: `internal/routing/router.go` reads the telemetry JSONL file, aggregates per-provider stats (avg latency, error rate, total tokens), and scores each provider. In `balanced` mode, it picks the primary + the secondary with the best score. Scoring formula:

```
score = (1 / avg_latency_ms) * (1 - error_rate) * log(total_successful_queries + 1)
```

Providers with zero history get a neutral score so they still get tried.

**Rationale**: Simple, interpretable, no training needed. The formula rewards fast, reliable providers that have been used successfully. New providers aren't penalized.

### 3. Repo memory: markdown files in a directory

**Choice**: `~/.config/polycode/memory/` contains markdown files, one per memory type:
- `build.md` — build and test commands
- `architecture.md` — project structure, key patterns
- `conventions.md` — coding style, naming, preferred libraries
- `providers.md` — per-provider notes (e.g., "claude good at Go, gemini good at search")

These are loaded and appended to the system prompt. The `/memory` command opens them in the TUI for viewing/editing, or the user can edit them directly.

**Rationale**: Markdown files are human-readable, easy to version control, and trivially loaded. No database needed.

### 4. Instruction hierarchy: file-based precedence

**Choice**: Three instruction sources, merged in precedence order:
1. **Repo-level**: `.polycode/instructions.md` in the current working directory (committed to the repo, shared with team)
2. **User-level**: `~/.config/polycode/instructions.md` (personal preferences)
3. **Built-in**: The default system prompt

Higher-precedence instructions are prepended to the system prompt. All three are concatenated (not replaced).

**Rationale**: This matches how Claude Code (CLAUDE.md), Codex (AGENTS.md), and Cursor (.cursorrules) handle project instructions. The file-based approach requires zero new infrastructure.

### 5. Periodic full-consensus calibration

**Choice**: In `quick` and `balanced` modes, every 10th query (configurable) triggers a silent full-consensus run in the background. The results are logged to telemetry but not shown to the user. This keeps the router's data fresh.

**Rationale**: Without periodic recalibration, the router's heuristics would go stale as provider quality changes. The background run adds minimal cost (1 in 10 queries).

## Risks / Trade-offs

- **Telemetry file reading on every query adds latency**: → **Mitigation**: Cache aggregated stats in memory, re-read the file only every 5 minutes.

- **Repo memory gets stale**: → **Mitigation**: Memory files are user-edited, not auto-generated. Users maintain them like documentation.

- **Calibration queries add hidden cost**: → **Mitigation**: Configurable interval, default 10. Can be disabled with `routing.calibration_interval: 0`.
