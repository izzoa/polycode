## Why

The "polycode" title in the tab bar is rendered in a single flat color. A subtle gradient across the title characters adds visual polish and brand identity, making the app feel more refined. This is a small, self-contained cosmetic improvement that leverages the theme system's Primary and Secondary colors.

## What Changes

- Apply a perceptual color gradient across the characters of the "polycode" title in the tab bar
- Use go-colorful library for smooth color interpolation in perceptually uniform color space (HCL)
- Interpolate between `theme.Primary` and `theme.Secondary` across the character positions
- Detect true color terminal support via `COLORTERM` environment variable
- Fall back to solid `theme.Primary` color on 256-color or lower terminals

## Capabilities

### New Capabilities
- `gradient-title`: Gradient color rendering for the title text with terminal capability detection

### Modified Capabilities
<!-- No existing spec-level changes -->

## Impact

- **Files modified** (1): `internal/tui/view.go`
- **Files created**: None
- **Dependencies**: `github.com/lucasb-eyer/go-colorful` (already indirect dependency)
- **Scope**: ~50-80 lines
