## ADDED Requirements

### Requirement: polycode run streams consensus to stdout
The `polycode run "prompt"` command SHALL execute the query pipeline and stream the consensus output to stdout without any TUI rendering.

#### Scenario: Basic headless execution
- **WHEN** the user runs `polycode run "explain this codebase"`
- **THEN** the consensus response SHALL be streamed to stdout AND the process SHALL exit with code 0

#### Scenario: Piping output
- **WHEN** the user runs `polycode run "summarize README.md" | head -5`
- **THEN** the first 5 lines of the consensus response SHALL be printed

### Requirement: Exit codes indicate outcome
The process SHALL exit with code 0 on success, 1 on error, and 2 when a tool call is rejected.

#### Scenario: Provider error exits 1
- **WHEN** all providers fail during a headless run
- **THEN** the process SHALL print an error to stderr AND exit with code 1

#### Scenario: Tool rejection exits 2
- **WHEN** running with `--confirm` and the user rejects a tool call
- **THEN** the process SHALL exit with code 2

### Requirement: Tool execution defaults to auto-approve
In headless mode, tool calls SHALL be automatically approved by default (yolo mode).

#### Scenario: Auto-approve in headless
- **WHEN** `polycode run "delete temp files"` is executed without --confirm
- **THEN** tool calls SHALL execute without prompting

### Requirement: --confirm flag enables interactive approval
The `--confirm` flag SHALL enable interactive tool approval prompts printed to stderr with input read from stdin.

#### Scenario: Interactive confirmation on stderr
- **WHEN** `polycode run --confirm "run tests"` triggers a shell_exec tool call
- **THEN** a confirmation prompt SHALL appear on stderr AND the user can type y/n on stdin

### Requirement: --provider flag for single-provider mode
The `--provider` flag SHALL query only the named provider, skipping consensus synthesis.

#### Scenario: Single provider query
- **WHEN** `polycode run --provider anthropic "explain X"` is executed
- **THEN** only the anthropic provider SHALL be queried AND its response SHALL be streamed directly to stdout

### Requirement: --no-tools flag disables tool execution
The `--no-tools` flag SHALL prevent any tool calls from being executed.

#### Scenario: Tools disabled
- **WHEN** `polycode run --no-tools "what files are here"` is executed
- **THEN** the model SHALL respond based on its knowledge only AND no tool calls SHALL be made
