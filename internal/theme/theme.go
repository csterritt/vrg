package theme

// Theme holds the active style set. The zero value emits no ANSI
// styling — the no-style composition path used to prove that no
// searched-data control byte can reach the terminal. Colour schemes and
// the toggle are Issue 7's; this seam provides the styles Issue 5
// needs.
type Theme struct {
	styled bool
}

// Styled returns the default styled theme.
func Styled() Theme { return Theme{styled: true} }

// Plain returns the no-style theme: every style is the identity, so no
// escape byte may legitimately appear in rendered output.
func Plain() Theme { return Theme{} }

// Inverse renders s in inverse video — the match highlight style.
func (t Theme) Inverse(s string) string {
	if !t.styled || s == "" {
		return s
	}
	return "\x1b[7m" + s + "\x1b[0m"
}

// Underline renders s underlined — the current file-list entry style.
func (t Theme) Underline(s string) string {
	if !t.styled || s == "" {
		return s
	}
	return "\x1b[4m" + s + "\x1b[24m"
}
