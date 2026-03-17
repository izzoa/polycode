package routing

// Mode represents an operating mode that controls cost/quality trade-offs.
type Mode string

const (
	ModeQuick    Mode = "quick"
	ModeBalanced Mode = "balanced"
	ModeThorough Mode = "thorough"
)

// ParseMode converts a string to a Mode, returning the mode and whether the
// string was a valid mode name.
func ParseMode(s string) (Mode, bool) {
	switch Mode(s) {
	case ModeQuick, ModeBalanced, ModeThorough:
		return Mode(s), true
	default:
		return "", false
	}
}
