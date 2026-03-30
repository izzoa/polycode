## ADDED Requirements

### Requirement: polycode ci reviews PRs in CI environments
The system SHALL provide a `polycode ci` subcommand for automated PR review in CI pipelines.

#### Scenario: CI reviews PR and posts comment
- **WHEN** `polycode ci --pr 42` runs in a CI environment
- **THEN** the system reviews the PR diff, posts the consensus review as a comment, and exits with 0 if no critical issues

#### Scenario: CI exits non-zero on critical issues
- **WHEN** the consensus review identifies critical issues
- **THEN** the CI command exits with status 1

#### Scenario: CI uses repo-level config
- **WHEN** `.polycode/config.yaml` exists in the repo
- **THEN** CI mode uses it instead of the user-level config

#### Scenario: CI reads API keys from environment
- **WHEN** environment variables like `POLYCODE_ANTHROPIC_KEY` are set
- **THEN** CI mode uses them instead of keyring storage
