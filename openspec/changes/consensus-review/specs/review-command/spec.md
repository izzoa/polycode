## ADDED Requirements

### Requirement: polycode review command for git diffs
The system SHALL provide a `polycode review` CLI subcommand that reviews code changes by fanning out to all providers and producing a structured consensus review.

#### Scenario: Review staged changes
- **WHEN** the user runs `polycode review`
- **THEN** the system gets the diff from `git diff --cached` (or `git diff` if nothing staged), sends it to all providers for review, synthesizes the findings, and outputs the structured review to stdout

#### Scenario: Review specific files
- **WHEN** the user runs `polycode review -- path/to/file.go`
- **THEN** the system reviews only the diff for the specified files

#### Scenario: No changes to review
- **WHEN** the user runs `polycode review` and there are no staged or unstaged changes
- **THEN** the system prints "No changes to review" and exits

### Requirement: polycode review for GitHub PRs
The system SHALL allow `polycode review --pr <number>` to review a GitHub PR and optionally post the review as a comment.

#### Scenario: Review PR and output to stdout
- **WHEN** the user runs `polycode review --pr 42`
- **THEN** the system fetches the PR diff via `gh pr diff 42`, reviews it, and outputs the consensus review to stdout

#### Scenario: Review PR and post comment
- **WHEN** the user runs `polycode review --pr 42 --comment`
- **THEN** the system posts the consensus review as a PR comment via `gh pr comment 42`

#### Scenario: gh CLI not available
- **WHEN** the user runs `polycode review --pr 42` and `gh` is not installed
- **THEN** the system prints an error with instructions to install the GitHub CLI

### Requirement: Structured review output format
The system SHALL format review output with sections for: summary, issues found (with severity), suggestions, and an overall assessment.

#### Scenario: Review with issues found
- **WHEN** the consensus review identifies problems in the diff
- **THEN** the output includes an "Issues" section with each issue's severity (critical/warning/info), file location, and description

#### Scenario: Clean review
- **WHEN** the consensus review finds no issues
- **THEN** the output includes an "Assessment" section stating the changes look good
