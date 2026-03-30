## ADDED Requirements

### Requirement: Task graph executes stages sequentially
The task graph SHALL execute stages in order, passing the output of each stage as input to the next.

#### Scenario: Three-stage pipeline
- **WHEN** a task graph has stages [planner, researcher, reviewer]
- **THEN** the planner runs first, its output goes to the researcher, the researcher's output goes to the reviewer

#### Scenario: Parallel workers within a stage
- **WHEN** a stage has multiple workers
- **THEN** all workers in that stage run concurrently and their outputs are merged before passing to the next stage

### Requirement: Budget cap enforcement
The task graph SHALL enforce a total token budget across all workers in a job.

#### Scenario: Under budget
- **WHEN** all stages complete within the token budget
- **THEN** the job completes normally

#### Scenario: Budget exceeded
- **WHEN** cumulative token usage exceeds the configured budget
- **THEN** the remaining stages are skipped and the output of the last completed stage is returned with a budget warning

### Requirement: Worker checkpoint persistence
The system SHALL save worker outputs after each stage completes so interrupted jobs can resume.

#### Scenario: Resume interrupted job
- **WHEN** a job was interrupted after the planner stage completed
- **THEN** `/plan --resume` loads the planner output from the checkpoint and continues from the researcher stage

#### Scenario: Completed job checkpoint
- **WHEN** a job completes all stages
- **THEN** the checkpoint file contains all worker outputs and is marked as complete

### Requirement: /plan command activates agent teams
The system SHALL activate the agent team pipeline when the user types `/plan <request>` in the chat input.

#### Scenario: /plan invocation
- **WHEN** the user types `/plan refactor the auth module`
- **THEN** the system runs the planner → researcher → reviewer pipeline instead of simple consensus

#### Scenario: Normal prompt uses consensus
- **WHEN** the user types a normal prompt without /plan
- **THEN** the existing fan-out + consensus pipeline is used
