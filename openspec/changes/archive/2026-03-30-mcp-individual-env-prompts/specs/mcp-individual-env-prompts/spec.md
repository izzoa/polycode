## ADDED Requirements

### Requirement: Known env vars are prompted individually
When the wizard has pre-populated env vars (from registry or template), the env step SHALL prompt each one individually with the var name as the prompt title.

#### Scenario: Registry server with two env vars
- **WHEN** the user selects a server requiring GITHUB_TOKEN (secret, required) and BASE_URL (non-secret, optional)
- **THEN** the wizard first prompts "GITHUB_TOKEN (required, secret):" with masked input, then "BASE_URL:" with plain input

#### Scenario: Template server with one secret env var
- **WHEN** the user selects the GitHub template (GITHUB_TOKEN, isSecret=true)
- **THEN** the wizard prompts "GITHUB_TOKEN:" with masked input

#### Scenario: No pre-populated env vars
- **WHEN** the user chose "Custom server" and no env vars are known
- **THEN** the wizard shows only the freeform KEY=value input

### Requirement: Secret env vars use masked input
During individual prompting, env vars marked as secret SHALL use `textinput.EchoPassword` mode.

#### Scenario: Secret var prompted with masking
- **WHEN** the wizard prompts for GITHUB_TOKEN (isSecret=true)
- **THEN** the text input shows bullets (•) instead of characters

#### Scenario: Non-secret var prompted without masking
- **WHEN** the wizard prompts for BASE_URL (isSecret=false)
- **THEN** the text input shows characters normally

### Requirement: Already-entered vars shown above current prompt
During individual prompting, previously-entered vars SHALL be shown above the current input with their values (secrets masked).

#### Scenario: Second var shows first var's value
- **WHEN** the user has entered GITHUB_TOKEN and is now prompted for BASE_URL
- **THEN** above the input, "GITHUB_TOKEN = •••••••• (configured)" is shown

### Requirement: Freeform input available after known vars
After all known vars are prompted, the wizard SHALL offer freeform KEY=value input for any additional environment variables.

#### Scenario: Freeform after known vars
- **WHEN** all known env vars have been entered
- **THEN** the wizard shows "Add additional env vars? (KEY=value or Enter to continue)"

### Requirement: Esc navigates backward through vars
Pressing Esc SHALL navigate backward: to the previous var if currently prompting a known var, or to the previous wizard step if at the first var.

#### Scenario: Esc on second var
- **WHEN** the user presses Esc while prompted for the second known var
- **THEN** the wizard returns to prompting the first var with its previously-entered value

#### Scenario: Esc on first var
- **WHEN** the user presses Esc while prompted for the first known var
- **THEN** the wizard returns to the previous step (mcpStepURL or mcpStepArgs)
