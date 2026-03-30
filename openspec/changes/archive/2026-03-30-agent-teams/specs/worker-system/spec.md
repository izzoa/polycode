## ADDED Requirements

### Requirement: Worker executes a role-specific query
Each worker SHALL execute a provider query with a role-specific system prompt and return the output as a string.

#### Scenario: Planner worker produces steps
- **WHEN** a planner worker receives a user request
- **THEN** it queries its assigned provider with the planner system prompt and returns a structured plan

#### Scenario: Researcher worker gathers context
- **WHEN** a researcher worker receives a plan from the planner stage
- **THEN** it queries its assigned provider with the researcher system prompt, including the plan as input, and returns gathered context

#### Scenario: Worker uses assigned provider
- **WHEN** a worker has a specific provider assigned via config roles
- **THEN** it queries that provider, not the primary or all providers

### Requirement: Role-specific system prompts
Each role type SHALL have a tailored system prompt that guides the model's behavior for that role.

#### Scenario: Planner prompt
- **WHEN** the planner worker is created
- **THEN** its system prompt instructs the model to break the request into concrete steps, identify required information, and produce a structured plan

#### Scenario: Reviewer prompt
- **WHEN** the reviewer worker is created
- **THEN** its system prompt instructs the model to validate the plan and research, identify gaps, and produce a final assessment

### Requirement: Role-to-provider mapping via config
The system SHALL allow users to configure which provider handles each role via a `roles` section in the config file.

#### Scenario: Configured role mapping
- **WHEN** the config contains `roles: { planner: claude, researcher: gemini, reviewer: gpt4 }`
- **THEN** each role uses its assigned provider

#### Scenario: Unmapped role defaults to primary
- **WHEN** a role is not specified in the config
- **THEN** that role uses the primary provider
