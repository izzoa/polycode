## Context

Polycode's current pipeline is flat: every query fans out to all providers, collects responses, and synthesizes. There is no concept of stages, roles, or specialization. Every model sees the same full conversation. For simple questions this works well, but for complex multi-step tasks (refactor a module, add a feature, debug an issue across files) it wastes tokens and misses the opportunity to use models strategically.

Phase 1 wired tool execution. Phase 2 added structured consensus and review. Phase 3 introduces the orchestration layer that ties it all together.

## Goals / Non-Goals

**Goals:**
- Define a worker abstraction with role, provider, system prompt, and isolated context
- Build a task graph executor that runs workers in sequence or parallel
- Start with a minimal 3-worker pipeline: planner → researcher → reviewer
- Persist worker checkpoints for resume
- Show worker progress in the TUI
- Let users map roles to providers via config

**Non-Goals:**
- Dynamic graph modification at runtime (fixed graph per invocation for v1)
- Training or fine-tuning models for specific roles
- Distributed execution across machines
- More than 5 role types in v1 (planner, researcher, implementer, tester, reviewer)
- Automatic role-to-provider optimization (that's Phase 4)

## Decisions

### 1. Worker as a thin wrapper around a provider query

**Choice**: A `Worker` is not a new abstraction over `Provider`. It's a struct that holds: a role name, a reference to a `Provider`, a role-specific system prompt, and an input/output contract. When executed, it constructs messages (system prompt + input), calls `provider.Query()`, and returns the output.

```go
type Worker struct {
    Role       RoleType
    Provider   provider.Provider
    SystemPrompt string
    MaxTokens  int
}

func (w *Worker) Run(ctx context.Context, input string) (string, error)
```

**Rationale**: Workers don't need their own streaming or auth — they delegate to existing providers. This keeps the abstraction thin and reuses all existing infrastructure.

### 2. Task graph: simple DAG with stages

**Choice**: A `TaskGraph` is an ordered list of `Stage`s. Each stage has one or more workers that run in parallel. Stages execute sequentially. The output of one stage becomes the input to the next.

```go
type Stage struct {
    Name    string
    Workers []*Worker  // run in parallel within the stage
    Merge   MergeFunc  // combines parallel worker outputs into one input for the next stage
}

type TaskGraph struct {
    Stages []Stage
    Budget int  // max total tokens across all workers
}
```

**Rationale**: A full DAG executor is over-engineered for v1. Stages (sequential groups of parallel workers) cover the planner → researcher → reviewer pattern and can express most useful pipelines. No need for arbitrary edges.

### 3. Default pipeline: planner → researcher → reviewer

**Choice**: When the user sends a complex prompt (detected by length, presence of multi-step language, or a `/plan` command), polycode uses the agent team pipeline instead of simple consensus:

1. **Planner** (provider: configurable, default primary): Receives the user's request, breaks it into steps, identifies what information is needed
2. **Researcher** (provider: configurable, default largest-context model): Takes the plan, reads relevant files, gathers context
3. **Reviewer** (provider: configurable, default primary): Reviews the plan + research, produces a final validated response or implementation plan

The output of the reviewer flows back to the main conversation as the "answer."

### 4. Activation: explicit `/plan` command

**Choice**: Agent teams don't activate automatically. The user types `/plan <request>` to explicitly invoke the multi-worker pipeline. Normal prompts continue using the existing consensus pipeline.

**Rationale**: Auto-detection of "complex" prompts is unreliable and surprising. Explicit activation gives the user control over cost and latency. The `/plan` command is easy to remember and clearly signals a different workflow.

### 5. Checkpoints: JSON file per job

**Choice**: Each agent team run gets a job ID. Worker outputs are saved to `~/.config/polycode/jobs/<job_id>.json` as each stage completes. If polycode crashes mid-job, `/plan --resume` picks up from the last completed stage.

### 6. TUI: worker progress panel

**Choice**: During a `/plan` run, the TUI shows a worker progress panel replacing the provider panels:
```
◆ Agent Team: refactor-auth-module
  ✓ Planner (claude) — 3 steps identified
  ● Researcher (gemini) — reading auth/ files...
  ○ Reviewer (gpt4) — pending
```

This uses the existing panel infrastructure with a new render function.

### 7. Config: roles section

```yaml
roles:
  planner: claude       # uses the provider named "claude"
  researcher: gemini    # large context window
  implementer: claude
  tester: claude
  reviewer: gpt4
```

If a role isn't mapped, it defaults to the primary provider.

## Risks / Trade-offs

- **Cost multiplication**: A 3-stage pipeline costs ~3x a single query. → **Mitigation**: Budget caps per job. `/plan` is explicit, not automatic. Users opt in knowing the cost.

- **Latency**: Sequential stages add latency. → **Mitigation**: Parallel workers within a stage help. The planner → researcher → reviewer pattern is only 3 serial calls.

- **Context isolation can lose information**: Summarized handoffs between stages may drop details. → **Mitigation**: Each stage receives the full output of the previous stage, not a summary. Summaries are only used when output exceeds the next worker's context limit.

- **Role system prompts may conflict with provider behavior**: Some models handle system prompts differently. → **Mitigation**: Role prompts are tested against all major providers. Keep them generic and instruction-focused.
