## Context

Polycode's TUI uses ~85 hardcoded `lipgloss.Color()` calls across 15 files. Colors were chosen ad-hoc as features were built (orange=214 for accents, blue=63 for interactive elements, cyan=86 for active states, etc.). The `Styles` struct in `model.go` captures some styles centrally but most rendering functions inline their own color references. There is no user-configurable palette.

## Goals / Non-Goals

**Goals:**
- Single source of truth for all colors via a `Theme` struct
- Zero visual regression when switching from hardcoded to theme-derived colors (default theme matches current palette exactly)
- User can switch themes at runtime without restart
- Theme persists across sessions via config
- Markdown rendering (glamour) respects the active theme

**Non-Goals:**
- Custom theme JSON/YAML files (start with built-in themes only)
- Per-element style customization beyond color (font weight, borders, padding are not theme concerns)
- LSP/syntax highlighting theme integration (that's chroma's domain)
- Transparent/translucent mode

## Decisions

### 1. Theme as a flat struct, not an interface

**Decision**: `Theme` is a plain struct with ~35 public `lipgloss.Color` fields. No interface abstraction, no inheritance, no nested sub-themes.

**Rationale**: A flat struct is the simplest thing that works. Each field maps 1:1 to a semantic use case. Compile-time safety ensures no missing colors. OpenCode uses 60+ slots with a similar pattern.

**Alternative rejected**: Interface-based themes (more flexible but unnecessary overhead for 6 built-in themes).

### 2. Theme stored on Model, passed to Styles constructor

**Decision**: `Model` holds a `theme Theme` field. `defaultStyles(theme Theme) Styles` derives all styles from the theme. The `Styles` struct remains the primary style cache used by renderers.

**Rationale**: This keeps the existing rendering pattern (renderers use `m.styles.X`) while making the styles theme-derived. Switching themes = rebuild `m.styles` from new theme.

### 3. Direct color replacement, file by file

**Decision**: Replace each `lipgloss.Color("214")` with `m.theme.Primary` (or the semantic equivalent) in a single pass per file. No intermediate adapter layer.

**Rationale**: The mapping from raw color numbers to semantic names is straightforward (214→Primary, 63→Secondary, 86→Tertiary, etc.). An adapter would add indirection without benefit.

### 4. Theme picker as overlay, not settings screen

**Decision**: `Ctrl+T` or `/theme` opens a list overlay (reusing the mode picker pattern) with live preview.

**Rationale**: Theme switching should be instant and visual. A settings-screen approach would be slower and harder to preview.

### 5. Glamour style override via ansi.StyleConfig

**Decision**: When theme changes, rebuild the glamour renderer with theme-derived `ansi.StyleConfig` values for headings, code blocks, links, and blockquotes.

**Rationale**: Glamour already supports custom `StyleConfig`. This ensures markdown rendering matches the active theme without replacing glamour.

## Risks / Trade-offs

- **[Risk] Missed color references** → Mitigation: grep for remaining `lipgloss.Color(` calls after migration; CI lint rule to prevent new hardcoded colors.
- **[Risk] Theme colors look wrong together** → Mitigation: Use established theme palettes (Catppuccin, Dracula, etc.) that are already color-balanced.
- **[Risk] Performance on theme switch** → Mitigation: Rebuilding `Styles` struct is ~1ms; rebuilding glamour renderer is ~5ms. Negligible.
- **[Risk] Breaking cached renders** → Mitigation: Theme switch invalidates `chatLogCache` and calls `rebuildChatLogCache()`.
