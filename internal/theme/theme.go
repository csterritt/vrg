// Package theme owns the visual styles for the browse view. Issue #5
// lands a minimal seam; later issues expand the style set. The no-style
// theme disables all ANSI sequences for sink-safety testing.
package theme

// Theme holds the visual style configuration for the browse view.
type Theme struct {
	noStyle bool
}

// New returns a default theme with styling enabled.
func New() Theme { return Theme{} }

// NoStyle returns a theme with all styling disabled. Rendering through
// the no-style theme produces no ANSI escape sequences, so any control
// byte in the output must come from unsanitized external data.
func NoStyle() Theme { return Theme{noStyle: true} }

// IsNoStyle reports whether all styling is disabled.
func (t Theme) IsNoStyle() bool { return t.noStyle }

// Underline wraps s in the ANSI underline sequence. With the no-style
// theme, s is returned unchanged.
func (t Theme) Underline(s string) string {
	if t.noStyle {
		return s
	}
	return "\x1b[4m" + s + "\x1b[0m"
}

// Reverse wraps s in the ANSI inverse-video sequence. With the no-style
// theme, s is returned unchanged.
func (t Theme) Reverse(s string) string {
	if t.noStyle {
		return s
	}
	return "\x1b[7m" + s + "\x1b[0m"
}
