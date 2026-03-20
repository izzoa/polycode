# Next Steps — Consolidated

Synthesized from independent analyses by Claude, Codex, Gemini, and Minimax. Assessed 2026-03-20 against main branch (8c076fe).

## Project Health

Polycode is **breadth-complete but depth-unverified**. All five roadmap phases in `IMPROVEMENTS.md` have their tasks checked off, but:

- **Subsystems aren't connected.** Hooks, permissions, MCP, mode routing, repo memory, and the instruction hierarchy exist as tested packages but are not called from the main app loop in `cmd/polycode/app.go`.
- **Validation gates are unchecked.** Every phase has unmet exit criteria — no golden task evals, no review benchmarks, no cost/latency regression tests.
- **One feature remains.** The skills/plugin system is the only unchecked item in Phase 5.
- **One bug is unfixed.** An unstaged diff in `app.go` fixes context fragmentation during multi-turn tool use.

All four analyses agree: the path to beta is stabilization, not expansion.

---

## Tier 1: Ship Blockers

Everything here must be resolved before polycode can be called beta-ready. All four analyses flag these items.

### 1.1 Commit the app.go Context Fix

**Consensus: 4/4 analyses flag this as P0.**

The unstaged diff in `cmd/polycode/app.go` combines initial consensus text + tool execution output into a single `RoleAssistant` message instead of appending them as separate messages. Without this, multi-turn tool-using conversations suffer context fragmentation — each follow-up turn sees an incomplete picture of prior work.

Action: test the change, commit it.

### 1.2 Wire Subsystems Into the Main Query Loop

**Consensus: 3/4 analyses (Claude, Codex, Minimax) identify this as the highest-leverage work.**

Six subsystems are implemented but not active at runtime:

| Subsystem | Package | Gap |
|-----------|---------|-----|
| **Hooks** | `internal/hooks` | `pre_query`, `post_query`, `post_tool`, `on_error` never fired from the conversation loop |
| **Permissions** | `internal/permissions` | Tool approval in `internal/action` doesn't consult the permission policy |
| **MCP tools** | `internal/mcp` | Discovered tools aren't registered in the tool executor |
| **Mode routing** | `internal/routing` | `/mode` updates UI state only; doesn't change which providers participate |
| **Repo memory** | `internal/memory` | Not loaded into system prompt; `/memory` TUI flow incomplete |
| **Instructions** | `internal/memory` | `.polycode/instructions.md` and user-level instructions not injected |

**Suggested order** (quickest wins first):
1. Hooks — add 4 call sites in app.go
2. Permissions — wire into tool approval path in `internal/action`
3. Mode routing — thread the router into `consensus.Pipeline`
4. Memory + instructions — modify system prompt construction
5. MCP — register discovered tools alongside built-in tools

**Exit criteria** (from Codex):
- `/mode` changes which providers participate in live queries
- `/memory` is functional and affects runtime context
- Hooks, permissions, and MCP are active in the main app flow
- Each subsystem has at least one integration test

### 1.3 Harden the Editor Bridge and CI Mode

**Consensus: 3/4 analyses (Claude, Codex, Minimax) flag security and reliability issues.**

**Editor bridge (`polycode serve`):**
- Verify it binds to loopback (`127.0.0.1`) by default, not `0.0.0.0`
- Add a minimal auth mechanism (bearer token, Unix socket, or trusted-origin check)
- Tighten CORS to match the intended deployment model (local editor only)

**CI mode (`polycode ci`):**
- Replace the `"critical"` substring heuristic for severity detection with structured review parsing
- A model saying "critical path" or "non-critical" currently triggers false positives/negatives

**Exit criteria** (from Codex):
- The editor bridge is not exposed on the local network by default
- CI failures are driven by structured review data, not keyword coincidence
- `review`, `ci`, and `serve` work against realistic local and PR-based workflows

---

## Tier 2: Quality and Confidence

Proving that the product works reliably. All analyses agree this is essential but sequence it after Tier 1.

### 2.1 Build Validation Gate Fixtures

**Consensus: 4/4 analyses call out the skipped validation gates as a critical debt.**

Every phase in `IMPROVEMENTS.md` has unchecked gates. Close the most impactful ones first:

1. **Golden task eval** (Phase 1 gate) — 5 repository tasks: file read, file edit, shell exec, fix-and-test, session resume. Run against mocked providers in CI; against real APIs in a nightly job. Measure pass rate.
2. **Review benchmark** (Phase 2 gate) — 10 git diffs with seeded bugs (security vulnerabilities, logic errors, hallucinated file references). Run `polycode review` and measure whether consensus catches more issues than single-model review. This is the existential proof point.
3. **Cost/latency regression** (Phase 4 gate) — Track average tokens and wall-clock time per mode (`quick` / `balanced` / `thorough`). Prove `quick` actually reduces cost.

Gemini's framing: *"We cannot prove we are winning without benchmarks."* If consensus doesn't consistently beat single-model, that's the most important signal — it means the synthesis pipeline needs work, not more features.

### 2.2 Add Tests for Untested Packages

**Consensus: 2/4 analyses (Claude, Codex) specifically flag these.**

Three packages have zero dedicated tests:

| Package | What to test |
|---------|-------------|
| `internal/provider/` | SSE parsing per adapter, error handling, auth header injection (use `httptest` servers) |
| `internal/auth/` | File fallback path, credential serialization, keyring error handling |
| `internal/tui/` | Key routing per view mode, message handling, view mode transitions (Bubble Tea models are testable via `Update`/`View`) |

### 2.3 Session Fidelity

**Consensus: 3/4 analyses (Claude, Codex, Minimax) flag session persistence gaps.**

- Exported sessions must include tool call parameters and results, not just assistant text
- Importing a session with tool history must allow coherent follow-up turns
- Crash recovery (`session.json`) must restore pending tool state
- Tool output shown in the TUI must match what is persisted

---

## Tier 3: Complete the Roadmap

### 3.1 Skills/Plugin System

**Consensus: 4/4 analyses identify this as the sole remaining Phase 5 feature.**

Priority placement varies: Gemini ranks it P1 (before validation), Claude and Minimax rank it P2 (after stabilization), Codex defers it entirely. The consolidated recommendation: complete it after Tier 1 integration and alongside Tier 2 validation work.

**Suggested minimal design** (Gemini + Claude agree on structure):

```
~/.config/polycode/skills/
  my-skill/
    skill.yaml          # name, version, description, slash command, tools
    system_prompt.md    # injected when skill is active
    tools/              # tool definitions (JSON schema + handler script)
```

**Implementation:**
1. `internal/skill/manifest.go` — parse `skill.yaml`
2. `internal/skill/registry.go` — load from disk, validate, deduplicate
3. Wire into the tool registry and slash command router
4. CLI: `polycode skill list`, `polycode skill install <path>`, `polycode skill remove <name>`
5. TUI: show active skills in settings, allow toggling

Keep the first version local-only — no remote registry or marketplace. Validate with 2-3 built-in skills before designing extensibility.

### 3.2 Calibrate the Adaptive Router

**Consensus: 2/4 analyses (Claude, Gemini) recommend this.**

The heuristic router in `internal/routing` uses static assumptions. To make it useful:
- Feed user signals (accepted/rejected tool calls, re-prompts) back as quality telemetry
- Use accumulated telemetry to weight provider selection per task type
- Add the periodic full-consensus fallback described in `IMPROVEMENTS.md` — even in `quick` mode, occasionally fan out to recalibrate

### 3.3 Close OpenSpec Verification Debt

**Source: Codex (unique recommendation).**

Pending manual verification items across OpenSpec changes:
- `native-tool-execution` — normal approval flow, yolo flow, multi-provider tool calls
- `openai-compatible-model-discovery` — endpoint discovery against real servers
- `wizard-selectable-inputs` — wizard flow walkthrough
- `polycode` — end-to-end smoke test

Record outcomes in the relevant task files; archive or close changes where appropriate.

---

## Tier 4: Strategic Differentiation

Not blockers, but would strengthen polycode's competitive position after stabilization.

### 4.1 Prove Consensus Beats Single-Model

**Consensus: 3/4 analyses (Claude, Gemini, Minimax) recommend building this evidence.**

Automated benchmark suite comparing consensus output vs best-single-model output on identical tasks. Publish results internally. If consensus doesn't consistently win, that signal redirects all further investment toward improving the synthesis pipeline.

### 4.2 TUI Provenance Display

**Consensus: 2/4 analyses (Claude, Minimax) recommend this as a differentiator.**

The consensus engine already produces minority reports and agreement data. Surface it:
- Show which models agreed/disagreed on each recommendation
- Allow expanding a consensus answer to see individual model responses
- Color-code confidence levels

No competitor shows users *why* the AI gave that answer.

### 4.3 `/plan` Improvements

**Source: Claude + Codex.**

- Resume interrupted plans without replaying completed stages
- Show stage-by-stage progress with expandable output
- Allow editing the plan mid-execution (drop/re-run stages)
- Replayable review artifacts with run-to-run comparison

### 4.4 Documentation and Beta Release Prep

**Source: Codex (dedicated phase).**

- Update `README.md` with the real command surface: `review`, `ci`, `serve`, `export`, `import`, `/plan`, `/memory`, `/mode`, tool execution
- Distinguish stable features from experimental ones
- Add usage examples for editor bridge, CI mode, and multi-provider review
- Confirm onboarding works from a clean machine: init → auth → launch → query → tool use → export

---

## What NOT to Do

All four analyses agree on these guardrails (aligned with `IMPROVEMENTS.md`):

- **No new provider adapters** — 4 types cover the market; OpenAI-compatible handles the long tail
- **No GUI/web/mobile surfaces** — TUI-first until core is proven
- **No plugin marketplace** — ship local skills first, validate demand
- **No TUI polish before runtime integration** — features need to work before they look better
- **No features that bypass consensus** — every new capability should flow through the multi-model path
- **No model-trained routing yet** — start with heuristics, graduate to ML only with enough telemetry
- **Stabilization over expansion** — close integration gaps before starting new top-level features

---

## Execution Summary

| Priority | Work | Effort | Impact | Agreement |
|----------|------|--------|--------|-----------|
| **P0** | Commit app.go context fix | 30 min | Unblocks multi-turn tool use | 4/4 |
| **P0** | Wire hooks + permissions into app loop | 1-2 days | Subsystems go from "exist" to "work" | 3/4 |
| **P0** | Wire mode routing into pipeline | 1 day | `/mode` becomes functional | 3/4 |
| **P0** | Wire memory + instructions into system prompt | 1 day | Context quality improves | 3/4 |
| **P0** | Wire MCP tools into executor | 1 day | External tools become usable | 3/4 |
| **P0** | Harden serve (loopback, auth, CORS) | 1 day | Security baseline | 3/4 |
| **P0** | Fix CI severity detection | 0.5 day | CI mode becomes reliable | 3/4 |
| **P1** | Golden task eval fixtures | 2-3 days | Proves execution works | 4/4 |
| **P1** | Review benchmark suite | 2-3 days | Proves the value proposition | 4/4 |
| **P1** | Provider/auth/TUI tests | 2-3 days | Confidence for refactoring | 2/4 |
| **P1** | Session fidelity for tool calls | 1-2 days | Multi-turn reliability | 3/4 |
| **P1** | Close OpenSpec verification debt | 1-2 days | Status matches reality | 1/4 |
| **P2** | Skills/plugin system | 3-5 days | Completes Phase 5 | 4/4 |
| **P2** | Router calibration with telemetry | 2-3 days | Cost efficiency | 2/4 |
| **P3** | Consensus vs single-model benchmarks | 2-3 days | Strategic evidence | 3/4 |
| **P3** | TUI provenance display | 2-3 days | Differentiation | 2/4 |
| **P3** | `/plan` resume + progress UX | 2-3 days | Agent team usability | 2/4 |
| **P3** | README + docs for beta | 1-2 days | Onboarding | 1/4 |

**The critical path is Tier 1.** Wiring the existing subsystems into the main runtime loop is where four independent analyses converge: it transforms polycode from a collection of well-built packages into an integrated product. Everything else follows.
