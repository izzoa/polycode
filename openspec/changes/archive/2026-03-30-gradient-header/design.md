## Context

The tab bar in `view.go` renders the "polycode" title as a styled string with a single color. The theme system provides `theme.Primary` and `theme.Secondary` colors. The go-colorful library is already an indirect dependency (via lipgloss or glamour) and provides perceptual color interpolation. Terminal true color support is indicated by the `COLORTERM` environment variable set to `truecolor` or `24bit`.

## Goals / Non-Goals

**Goals:**
- Smooth perceptual gradient across "polycode" title characters
- Uses theme colors so gradient adapts when theme changes
- Graceful fallback on terminals without true color support
- Zero performance impact (8 characters, computed once per render)

**Non-Goals:**
- Animated gradients or color cycling
- Gradient on other UI elements (tab labels, status bar)
- User-configurable gradient direction or color stops
- Multi-line gradient effects

## Decisions

### 1. HCL interpolation via go-colorful

**Decision**: Convert theme.Primary and theme.Secondary to go-colorful Color, then use `BlendHcl()` for perceptual interpolation at each character position.

**Rationale**: HCL blending produces visually smooth gradients (no brown midpoints that RGB blending causes). go-colorful is already in the dependency tree.

### 2. Per-character lipgloss styling

**Decision**: Render each character of "polycode" with its own `lipgloss.NewStyle().Foreground(lipgloss.Color(hex))` where hex is the interpolated color at that position.

**Rationale**: Lipgloss does not support inline gradients natively. Per-character styling is the standard approach (8 characters = negligible overhead).

### 3. COLORTERM-based detection

**Decision**: Check `os.Getenv("COLORTERM")` for "truecolor" or "24bit". If absent, fall back to solid theme.Primary.

**Rationale**: COLORTERM is the de facto standard for advertising true color support. Avoids terminfo complexity.

## Risks / Trade-offs

- **[Risk] COLORTERM not set in some true-color terminals** -> Mitigation: Fallback is graceful (solid color, not broken). User can set COLORTERM manually.
- **[Risk] go-colorful becomes a direct dependency** -> Mitigation: It's already indirect; promoting to direct is low cost.
- **[Risk] Gradient looks bad with certain theme color pairs** -> Mitigation: All built-in themes have complementary Primary/Secondary colors chosen for smooth blending.
