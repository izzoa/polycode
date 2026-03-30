## 1. Worker & Role Definitions

- [x] 1.1 Create `internal/agent/` package with `worker.go`: `RoleType` enum (RolePlanner, RoleResearcher, RoleImplementer, RoleTester, RoleReviewer)
- [x] 1.2 Define `Worker` struct: Role, Provider (provider.Provider), SystemPrompt, MaxTokens
- [x] 1.3 Implement `Worker.Run(ctx, input string) (output string, usage tokens.Usage, err error)` — constructs messages [system prompt, user input], calls provider.Query(), drains stream, returns full text + usage
- [x] 1.4 Define role-specific system prompts in `internal/agent/prompts.go`

## 2. Config: Roles Section

- [x] 2.1 Add `RolesConfig` struct to `internal/config/config.go`: map of role name → provider name
- [x] 2.2 Add `Roles RolesConfig` field to the top-level `Config` struct
- [x] 2.3 Implement `ResolveProvider(role RoleType, registry, cfg)` — looks up the role in config, finds the provider by name, falls back to primary

## 3. Task Graph Executor

- [x] 3.1 Create `internal/agent/graph.go` with `Stage` struct and `TaskGraph` struct
- [x] 3.2 Implement `TaskGraph.Run(ctx, input, onStageComplete)` — sequential stages, parallel workers, merge, budget cap
- [x] 3.3 Define `JobResult` struct with per-stage outputs, total usage, completion status
- [x] 3.4 Implement default merge function

## 4. Checkpoint Persistence

- [x] 4.1 Define `JobCheckpoint` struct in `internal/agent/checkpoint.go`
- [x] 4.2 Implement `SaveCheckpoint(jobID, checkpoint)` — saves to ~/.config/polycode/jobs/
- [x] 4.3 Implement `LoadCheckpoint(jobID)` — loads from file
- [x] 4.4 Update `TaskGraph.Run()` to save checkpoint after each stage
- [x] 4.5 Implement `TaskGraph.Resume(ctx, jobID)` — loads checkpoint, skips completed stages

## 5. /plan Command Integration

- [x] 5.1 Detect `/plan <request>` in `updateChat()` input handler
- [x] 5.2 Add `onPlan func(request string)` callback to the TUI model with `SetPlanHandler()`
- [x] 5.3 Wire the plan handler in `app.go`: build default task graph (planner → researcher → reviewer), resolve providers from config roles, run the graph
- [x] 5.4 Stream stage completions to the TUI via `WorkerProgressMsg` messages
- [x] 5.5 When the final stage completes, send the reviewer output as a `PlanDoneMsg` to display in the chat
- [x] 5.6 Handle `/plan --resume` to resume the most recent incomplete job

## 6. TUI Worker Progress

- [x] 6.1 Add `WorkerProgressMsg` type: StageName, Role, Status, Summary
- [x] 6.2 Add `agentStages []agentStageDisplay` field to the TUI model
- [x] 6.3 Add `planRunning bool` field to the model
- [x] 6.4 Implement `renderWorkerProgress()` — shows each stage and its workers with status icons
- [x] 6.5 Display worker progress panel during `/plan` execution

## 7. Testing

- [x] 7.1 Unit test: Worker.Run with mock provider returns expected output
- [x] 7.2 Unit test: TaskGraph.Run executes 3 stages sequentially, output chains correctly
- [x] 7.3 Unit test: Budget cap stops execution when exceeded
- [x] 7.4 Unit test: Checkpoint save/load round-trip
- [x] 7.5 Unit test: TaskGraph.Resume skips completed stages
- [x] 7.6 Unit test: ResolveProvider returns configured provider or falls back to primary
