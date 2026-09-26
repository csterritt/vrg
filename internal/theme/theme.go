package theme

// Theme is the minimal style seam for Issue #5: the dark scheme's
// inverse-video match and underline treatments plus Plain, the no-style
// composition path the sink-safety tests render through so no escape
// byte may legitimately appear. Scheme toggling and the full style set
// arrive with Issue #7.
type Theme struct {
	plain bool
}

// Dark returns the default white-on-black scheme with real styling.
func Dark() Theme { return Theme{} }

// Plain returns the no-style composition path: every style is the
// identity, so rendered output contains no SGR bytes at all.
func Plain() Theme { return Theme{plain: true} }

// Inverse renders s in inverse video, or unchanged under Plain.
func (t Theme) Inverse(s string) string {
	if t.plain || s == "" {
		return s
	}
	return "\x1b[7m" + s + "\x1b[27m"
}

// Underline renders s underlined, or unchanged under Plain.
func (t Theme) Underline(s string) string {
	if t.plain || s == "" {
		return s
	}
	return "\x1b[4m" + s + "\x1b[24m"
}
