## Why

Polycode currently uses all providers equally for every query — the same fan-out, same synthesis, regardless of whether the task is planning, researching, coding, or reviewing. This wastes tokens (sending implementation details to a model only needed for review) and misses the opportunity to leverage each model's strengths. Claude excels at careful reasoning, Gemini has a massive context window for research, GPT is strong at structured output. Phase 3 introduces role-based workers that assign different models to different stages of a multi-step task, with a task graph that orchestrates them.

## What Changes

- **Worker contract**: Define a `Worker` with a role (planner, researcher, implementer, tester, reviewer), an assigned provider, its own system prompt, isolated context, and input/output schemas
- **Role-based system prompts**: Each role gets a tailored prompt — the planner breaks down tasks, the researcher gathers context, the implementer writes code, the tester validates, the reviewer checks quality
- **Task graph executor**: Orchestrate workers in a directed graph with sequential and parallel branches, merge points, iteration limits, and budget caps
- **Minimal initial pipeline**: Start with `planner → researcher → reviewer` — the planner breaks the user's request into steps, the researcher gathers relevant code/docs, the reviewer validates the plan before implementation
- **Worker checkpoints**: Persist each worker's output so interrupted multi-step jobs can resume from the last completed worker
- **TUI worker progress**: A new panel showing which workers are active, completed, or pending, with expandable output per worker
- **Role-to-provider config**: Users map roles to providers in config — different models for different jobs
- **Context isolation**: Workers operate in their own context windows; only summarized output flows back to the main conversation

## Capabilities

### New Capabilities
- `worker-system`: Worker contract, role types, system prompts, and provider assignment
- `task-graph`: Directed graph executor with sequential/parallel branches, merge, checkpoints, and budget limits
- `worker-tui`: TUI panel for worker progress and output display

### Modified Capabilities
_(none — the existing consensus pipeline remains the default for simple queries; the agent team system activates for complex multi-step tasks)_

## Impact

- **New `internal/agent/`**: Worker, role definitions, task graph executor, checkpoint persistence
- **`internal/config/`**: New `roles` config section mapping role names to provider names
- **`internal/tui/`**: New worker progress view mode and panel
- **`cmd/polycode/app.go`**: Detection of complex tasks that should use agent teams vs simple consensus
