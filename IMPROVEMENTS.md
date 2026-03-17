# Polycode Improvement Roadmap

Prioritized phases for making polycode a production-ready, differentiated multi-model coding assistant. Each phase builds on the previous — do not skip ahead.

**Strategic positioning:** Do not chase Claude Code, Codex, or OpenCode feature-for-feature. Win by turning multi-model consensus from a single synthesis pass into a trustworthy execution and review system.

---

## Phase 1: Execution Core & Eval Harness

**Goal:** Make polycode trustworthy as an acting coding agent.

**Why it matters:** The current architecture produces consensus text but doesn't reliably execute tool calls from that output. Without a hardened execution loop, polycode is a demo, not a tool.

### Tasks

- [x] Wire the existing tool loop (`internal/action/loop.go`) into the main app path in `cmd/polycode/app.go` — consensus output with tool calls must trigger actual file edits and shell commands
- [x] Carry full working state (conversation + tool results + file context) into the synthesis prompt, not just the original user prompt + raw provider responses
- [x] Define a canonical execution state machine: `prompt → fan-out → collect → synthesize → tool-loop → confirm → save-session`
- [x] Add per-provider telemetry: latency (time-to-first-token, total), token counts, error rates, timeout rates — logged to a local telemetry file
- [x] Build golden-task end-to-end eval fixtures that test: file read, file edit, shell exec, test-run-and-fix, and session resume from disk
- [x] Add deterministic behavior for timeouts, provider failures, and session resume (no silent data loss)
- [x] Ensure tool execution results feed back into the conversation state and are persisted in the session file

### Validation Gates

- [ ] Golden tasks pass consistently across Anthropic, OpenAI, and at least one OpenAI-compatible provider
- [ ] Timeouts and provider failures produce inspectable, deterministic behavior
- [ ] Session resume after crash/quit restores full working state including pending tool results

---

## Phase 2: Evidence-Backed Consensus Review

**Goal:** Turn consensus into a reliability feature, not just a summary layer.

**Why it matters:** This is polycode's primary differentiator. Competitors have strong single-model agent loops. Polycode wins if it becomes the best reviewer, verifier, and synthesizer of code changes.

### Tasks

- [x] Design a structured response envelope schema that every provider response maps into: `{ proposed_action, evidence, assumptions, confidence, disagreements }`
- [x] Update the consensus prompt to request structured output from providers (action + reasoning, not just prose)
- [x] Add a verifier lane: after consensus, run the proposed change through tests, lint, and/or a security review pass before presenting to the user
- [x] Implement minority reports: when models disagree, surface the dissenting view with its evidence so the user can evaluate
- [x] Build `polycode review` CLI subcommand: takes a `git diff` (or staged changes), fans out to all providers for review, synthesizes findings
- [x] Extend `polycode review` to accept a GitHub PR URL (via `gh` CLI) and post the consensus review as a comment
- [x] Build a benchmark set with seeded bugfix, refactor, and security-review cases to measure review quality
- [x] Show review provenance in the TUI: which models agreed, which disagreed, and what evidence was cited

### Validation Gates

- [ ] Review benchmarks catch more seeded regressions and hallucinated file references than the current single-synthesis flow
- [ ] Users can inspect why a consensus answer was chosen, which models disagreed, and what evidence was used

---

## Phase 3: Consensus-Native Agent Teams

**Goal:** Evolve from one ensemble answer into an orchestrated multi-model task graph.

**Why it matters:** Competitors already have subagents, but they use the same model for every role. Polycode can assign different models to planner, researcher, implementer, tester, and reviewer — using each model's strengths.

### Tasks

- [ ] Define the worker contract: each worker has a role, a provider assignment, its own context window, an input schema, and an output schema
- [ ] Implement role types: `planner`, `researcher`, `implementer`, `tester`, `reviewer` — each with a tailored system prompt
- [ ] Build a task graph executor: sequential and parallel branches, merge semantics, iteration limits per branch, and budget caps
- [ ] Start with a minimal pipeline: `planner → researcher → reviewer` before adding implementer/tester
- [ ] Persist worker checkpoints so interrupted jobs can resume without replaying everything
- [ ] Expose worker progress, branch outputs, and merge decisions in the TUI (new view mode or panel)
- [ ] Allow users to configure which provider handles which role via config:
  ```yaml
  roles:
    planner: claude
    researcher: gemini    # large context window
    implementer: claude
    reviewer: gpt4
  ```
- [ ] Isolate large-output work to workers instead of the main conversation to keep context growth bounded

### Validation Gates

- [ ] Multi-step tasks complete more reliably than the current one-shot consensus pipeline on complex repository tasks
- [ ] Context growth stays bounded because worker output is summarized before merging into the main thread

---

## Phase 4: Adaptive Routing & Repo Memory

**Goal:** Make multi-model quality affordable and cumulative.

**Why it matters:** Blind fan-out to all providers does not scale on cost or latency. A production product needs to learn which models work best for which tasks.

### Tasks

- [ ] Define telemetry schema needed for routing decisions: task type, provider, latency, token cost, quality signal (user accepted/rejected), error rate
- [ ] Implement heuristic router: choose providers per task based on type (debug → models good at debugging, review → models good at review) using historical data
- [ ] Add operating modes that users can switch between:
  - `quick` — primary model only, no consensus (lowest cost/latency)
  - `balanced` — primary + one secondary, consensus synthesis (default)
  - `thorough` — all providers, full consensus + verifier lane
- [ ] Implement repo memory (`~/.config/polycode/memory/`): build commands, test commands, architecture notes, preferred patterns, provider performance by domain
- [ ] Add instruction hierarchy: repo-level (`.polycode/instructions.md`) > user-level (`~/.config/polycode/instructions.md`) > session-level
- [ ] Make repo memory editable and inspectable via `/memory` command or settings screen
- [ ] Add periodic full-consensus fallback: even in `quick` mode, occasionally run full consensus to recalibrate routing

### Validation Gates

- [ ] Average task cost and latency drop without regressing benchmark quality
- [ ] Users stop repeating repository instructions and common commands across sessions

---

## Phase 5: Workflow Platform & Team Adoption

**Goal:** Add the ecosystem hooks required for real team adoption.

**Why it matters:** MCP, hooks, skills, permissions, and CI review are now table stakes. They are not the wedge, but they are required for adoption once the core is strong.

### Tasks

- [ ] Implement MCP (Model Context Protocol) server support: allow polycode to connect to external tool servers
- [ ] Add a hook system: pre-commit, post-query, on-error hooks that run user-defined shell commands
- [ ] Add skill/plugin packages: installable extensions that add new slash commands or tool definitions
- [ ] Design and implement a permission model: per-tool approval policies (always allow, always ask, deny), scoped by repo/user
- [ ] Add `polycode ci` mode: headless agent that runs in CI, reviews PRs, posts consensus findings as PR comments
- [ ] Implement session sharing: export a consensus trace as a shareable artifact (JSON or markdown) so teammates can see the multi-model review
- [ ] Add a minimal editor bridge: VS Code extension or LSP server that sends selections/files to the polycode TUI
- [ ] Ship replayable review artifacts so teams can compare model ensembles over time

### Validation Gates

- [ ] At least 2-3 real integrations (MCP + CI review + one hook) work end-to-end without bespoke support
- [ ] Teams can share instructions, skills, and review artifacts repeatably across repos

---

## Deprioritized (Do Not Start Yet)

These are explicitly out of scope until the phases above are validated:

- Adding more provider adapters as a primary focus (the 4 current types cover the market)
- Desktop, browser, or mobile surfaces (TUI-first until the core is strong)
- Large TUI redesign that doesn't improve evidence, trust, or workflow throughput
- More slash commands without first improving execution semantics and verification
- Model-trained routing (start with heuristics, graduate to ML only with enough telemetry)
- General plugin marketplace (ship MCP and CI review first)

---

## How to Use This Document

1. **Pick the current phase** — work top-to-bottom, don't skip
2. **Use `/opsx:propose`** to create an openspec change for each major deliverable within a phase
3. **Hit the validation gates** before moving to the next phase
4. **Update this file** as tasks are completed: `- [ ]` → `- [x]`
5. **Re-evaluate** after each phase — the roadmap should adapt based on what you learn
