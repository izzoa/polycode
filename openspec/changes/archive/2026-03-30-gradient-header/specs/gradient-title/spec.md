## ADDED Requirements

### Requirement: Title renders with gradient between theme colors
The "polycode" title in the tab bar SHALL render with a perceptual color gradient interpolated between `theme.Primary` and `theme.Secondary` across its characters.

#### Scenario: Gradient applies to title
- **WHEN** the tab bar is rendered on a true color terminal
- **THEN** each character of "polycode" SHALL have a distinct color interpolated between theme.Primary (first character) and theme.Secondary (last character) using HCL color space

### Requirement: Gradient adapts to active theme
The gradient colors SHALL update when the theme changes.

#### Scenario: Theme switch updates gradient
- **WHEN** the user switches from Polycode theme to Dracula theme
- **THEN** the title gradient SHALL interpolate between Dracula's Primary and Secondary colors

### Requirement: Fallback to solid color on non-true-color terminals
On terminals that do not advertise true color support, the title SHALL render in a solid `theme.Primary` color.

#### Scenario: 256-color terminal fallback
- **WHEN** the COLORTERM environment variable is not set or is not "truecolor"/"24bit"
- **THEN** the title SHALL render in solid theme.Primary color without gradient

### Requirement: True color detection uses COLORTERM
The system SHALL detect true color support by checking if the `COLORTERM` environment variable is set to "truecolor" or "24bit".

#### Scenario: COLORTERM detection
- **WHEN** COLORTERM is set to "truecolor"
- **THEN** the gradient rendering path SHALL be used
