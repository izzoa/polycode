## ADDED Requirements

### Requirement: Press e to enter edit mode during confirmation
The system SHALL allow the user to press `e` during a tool confirmation prompt to enter edit mode, which displays a textarea pre-filled with the tool's primary content.

#### Scenario: Enter edit mode for shell command
- **WHEN** a shell_exec confirmation is shown and the user presses `e`
- **THEN** a textarea SHALL appear containing the proposed command string, with cursor at the end

#### Scenario: Enter edit mode for file write
- **WHEN** a file_write confirmation is shown and the user presses `e`
- **THEN** a textarea SHALL appear containing the proposed file content

### Requirement: Edited content replaces original on submission
When the user presses Enter (or Ctrl+S) in edit mode, the edited content SHALL be sent to the executor in place of the original content.

#### Scenario: Modified command is executed
- **WHEN** the user edits a shell_exec command from "rm -rf /tmp/old" to "rm -rf /tmp/old --verbose" and submits
- **THEN** the executor SHALL run "rm -rf /tmp/old --verbose"

#### Scenario: Original used when no edits made
- **WHEN** the user enters edit mode but submits without changes
- **THEN** the executor SHALL receive the original content unchanged

### Requirement: Esc cancels edit mode without executing
Pressing Esc in edit mode SHALL return to the normal confirmation prompt without executing or modifying the tool call.

#### Scenario: Cancel edit returns to confirm
- **WHEN** the user presses Esc while in edit mode
- **THEN** the confirmation prompt SHALL reappear in its original state with y/n/e options

### Requirement: Confirm channel carries edited content
The confirm channel SHALL be typed as `chan ConfirmResult` carrying both an approval boolean and an optional edited content string.

#### Scenario: Executor receives edited content
- **WHEN** the user edits and submits modified content
- **THEN** the ConfirmResult SHALL have Approved=true and EditedContent pointing to the modified string
