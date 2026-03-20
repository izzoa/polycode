# Next Steps — Claude Analysis

Assessed 2026-03-20 against main branch (8c076fe).

## Project Health

Polycode has impressive breadth: 4 provider adapters, streaming consensus, tool execution, agent teams, adaptive routing, TUI with splash/settings/wizard, CLI with review/ci/serve/export/import, MCP, hooks, permissions, telemetry, token tracking, and repo memory. All of this compiles and passes tests (~13 packages, ~190+ tests).

The core problem is that many subsystems are **implemented but not wired into the main runtime loop**. The codebase has the pieces; the app doesn't use them all yet. This is the primary blocker to beta.

---

## Tier 1: Ship Blockers (Do These First)

### 1.1 Commit the app.go Context Fix

The unstaged diff in `cmd/polycode/app.go` fixes a real bug: tool execution results were appended as separate assistant messages instead of being combined with the initial consensus text. This causes context fragmentation in multi-turn tool-using conversations. Test it, commit it.

### 1.2 Wire Subsystems Into the Main Query Loop

This is the single highest-leverage workstream. The following subsystems exist as packages with tests but are **not called from the main app path** in `cmd/polycode/app.go`:

| Subsystem | Package | What's Missing |
|-----------|---------|----------------|
| **Hooks** | `internal/hooks` | `pre_query`, `post_query`, `post_tool`, `on_error` not fired from the conversation loop |
| **Permissions** | `internal/permissions` | Tool approval decisions in `action/` don't consult the permission policy |
| **MCP tools** | `internal/mcp` | Discovered MCP tools aren't registered in the tool executor's registry |
| **Mode routing** | `internal/routing` | `/mode` updates UI state but doesn't change which providers participate in queries |
| **Repo memory** | `internal/memory` | Memory isn't loaded into the system prompt; `/memory` TUI flow is incomplete |
| **Instruction hierarchy** | `internal/memory` | `.polycode/instructions.md` and user-level instructions aren't injected |

**Suggested order:** Hooks and permissions are the quickest wins (a few call sites in app.go). Mode routing requires threading the router into the pipeline. Memory/instructions require system prompt construction changes.

### 1.3 Harden the Editor Bridge and CI Mode

Two security and reliability issues:

- **`polycode serve`** binds to a network port. Verify it defaults to loopback-only (`127.0.0.1`), not `0.0.0.0`. Add a minimal auth mechanism (bearer token, Unix socket, or trusted-origin check) so other processes on the machine can't hijack sessions.
- **`polycode ci`** detects review severity by looking for the substring `"critical"` in review text. This is fragile — a model mentioning "critical path" or "non-critical" could trigger or miss real issues. Parse structured review output instead.

---

## Tier 2: Quality and Confidence

### 2.1 Add Tests for Untested Packages

Three packages have no dedicated tests:

- **`internal/provider/`** — Provider adapters are only exercised via integration tests. Add unit tests with HTTP test servers for each adapter's SSE parsing, error handling, and auth header injection.
- **`internal/auth/`** — Keyring + file fallback + OAuth flows are untested in isolation. At minimum, test the file fallback path and credential serialization.
- **`internal/tui/`** — Bubble Tea models can be unit-tested by sending messages and asserting on model state. Test key routing per view mode, message handling, and view mode transitions.

### 2.2 Build Validation Gate Fixtures

Every phase has unchecked validation gates in `IMPROVEMENTS.md`. The most impactful ones to close first:

1. **Golden task eval** (Phase 1 gate): 5 repository tasks (read file, edit file, run shell, fix-and-test, session resume). Run against mocked providers in CI; run against real APIs in a nightly job.
2. **Review benchmark** (Phase 2 gate): 10 git diffs with seeded bugs. Measure if consensus catches more issues than single-model review. This is the existential proof point for polycode's value proposition.
3. **Cost/latency regression** (Phase 4 gate): Track average tokens and wall-clock time per mode. Ensure `quick` mode actually reduces cost vs `thorough`.

### 2.3 Session Fidelity

Tool calls and tool results should round-trip through session export/import. Verify:
- Exported sessions include tool call parameters and results, not just assistant text.
- Importing a session with tool history allows coherent follow-up turns.
- Crash recovery (`session.json`) restores pending tool state.

---

## Tier 3: Complete the Roadmap

### 3.1 Skills/Plugin System

The only unchecked Phase 5 feature. Suggested minimal design:

```
~/.config/polycode/skills/
  my-skill/
    skill.yaml          # name, version, description, slash command, tools
    system_prompt.md    # injected when skill is active
    tools/              # tool definitions (JSON schema + handler script)
```

Implementation:
1. `internal/skill/manifest.go` — parse `skill.yaml`
2. `internal/skill/registry.go` — load from disk, validate, deduplicate
3. Wire into the tool registry and slash command router
4. CLI: `polycode skill list`, `polycode skill install <path>`, `polycode skill remove <name>`
5. TUI: show active skills in settings, allow toggling

Keep the first version local-only (no remote registry or marketplace). Validate with 2-3 built-in skills (e.g., a "git-review" skill that wraps common review workflows).

### 3.2 Calibrate the Adaptive Router

The heuristic router in `internal/routing` uses static assumptions. To make it useful:
- Feed user signals (accepted/rejected tool calls, re-prompts, explicit thumbs up/down) back as quality telemetry.
- Use accumulated telemetry to weight provider selection per task type.
- Add the periodic full-consensus fallback described in IMPROVEMENTS.md (even in `quick` mode, occasionally fan out to recalibrate).

---

## Tier 4: Strategic Differentiation

These are not blockers but would strengthen polycode's competitive position.

### 4.1 Prove Consensus Beats Single-Model

The entire value proposition is that multi-model consensus produces better results. Build the evidence:
- Automated benchmark suite comparing consensus output vs best-single-model output on the same tasks.
- Publish results (even internal) to guide development priorities.
- If consensus doesn't consistently win, that's the most important signal — it means the synthesis prompt or pipeline needs work, not more features.

### 4.2 Streaming Provenance in the TUI

The consensus engine already produces minority reports and agreement data. Surface this in the TUI:
- Show which models agreed/disagreed on each recommendation.
- Allow expanding a consensus answer to see individual model responses.
- Color-code confidence levels.

This is a differentiator — no competitor shows you *why* the AI gave that answer.

### 4.3 `/plan` Improvements

Agent teams exist but `/plan` could be more useful:
- Resume interrupted plans without replaying completed stages.
- Show stage-by-stage progress with expandable output.
- Allow editing the plan mid-execution (drop a stage, re-run a stage with different instructions).

---

## What NOT to Do

Aligned with IMPROVEMENTS.md, avoid:
- Adding new provider adapters (4 types cover the market; OpenAI-compatible handles the long tail)
- Building a GUI/web frontend (TUI-first until the core is proven)
- Designing a plugin marketplace (ship local skills first, validate demand)
- Over-investing in TUI polish before the runtime integration gaps are closed
- Adding features that bypass the consensus pipeline (every new capability should flow through the multi-model path)

---

## Execution Summary

| Priority | Work | Effort | Impact |
|----------|------|--------|--------|
| **P0** | Commit app.go fix | 30 min | Unblocks multi-turn tool use |
| **P0** | Wire hooks + permissions into app loop | 1-2 days | Features go from "exist" to "work" |
| **P0** | Wire mode routing into pipeline | 1 day | `/mode` becomes functional |
| **P0** | Wire memory + instructions into system prompt | 1 day | Context quality improves |
| **P0** | Harden serve (loopback, auth) | 1 day | Security baseline |
| **P1** | Fix CI severity detection | 0.5 day | CI mode becomes reliable |
| **P1** | Provider/auth/TUI tests | 2-3 days | Confidence for refactoring |
| **P1** | Golden task eval fixtures | 2-3 days | Proves execution works |
| **P1** | Session fidelity for tool calls | 1-2 days | Multi-turn reliability |
| **P2** | Skills/plugin system | 3-5 days | Completes Phase 5 |
| **P2** | Review benchmarks | 2-3 days | Proves the value proposition |
| **P2** | Router calibration | 2-3 days | Cost efficiency |
| **P3** | TUI provenance display | 2-3 days | Differentiation |
| **P3** | `/plan` improvements | 2-3 days | Agent team UX |

**The critical path is Tier 1.** Closing the runtime integration gaps transforms polycode from a collection of well-built packages into an actual product. Everything else follows from that.
