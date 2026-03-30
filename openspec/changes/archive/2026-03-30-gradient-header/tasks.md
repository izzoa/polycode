## 1. Gradient Rendering

- [x] 1.1 Add `github.com/lucasb-eyer/go-colorful` as direct dependency (`go get`)
- [x] 1.2 Implement `renderGradientTitle(text string, from, to lipgloss.Color) string` function in view.go
- [x] 1.3 Parse lipgloss.Color hex values into go-colorful Color objects
- [x] 1.4 Interpolate using BlendHcl at position `i / (len-1)` for each character
- [x] 1.5 Style each character with lipgloss using the interpolated hex color

## 2. Terminal Detection

- [x] 2.1 Implement `supportsTrueColor() bool` checking COLORTERM env var for "truecolor" or "24bit"
- [x] 2.2 Cache the result at startup (env var does not change during execution)

## 3. Integration

- [x] 3.1 Replace the flat-colored title rendering in the tab bar with `renderGradientTitle` when true color is supported
- [x] 3.2 Fall back to solid theme.Primary when true color is not supported
- [x] 3.3 Ensure gradient recalculates when theme changes (uses current theme.Primary and theme.Secondary)

## 4. Verification

- [x] 4.1 Run `go build ./...` and `go test ./...` -- all pass
- [x] 4.2 Manual: verify gradient appearance on true color terminal (iTerm2, kitty, etc.)
- [x] 4.3 Manual: set COLORTERM="" and verify solid color fallback
- [x] 4.4 Manual: switch themes and verify gradient updates
