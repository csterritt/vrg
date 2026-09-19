// Package theme owns the active colour scheme and styles.
//
// Issue #5 lands the seam's first styles: inverse-video match runs and
// the underlined current file-list entry, plus the no-style composition
// path the sink-safety tests require. The `c` colour toggle and the
// full style set are Issue #7's.
package theme

// Theme is the active scheme's style set. The zero value is the
// no-style composition path: every decorator is the identity, so no
// escape byte may legitimately appear in composed output.
type Theme struct{ styled bool }

// Dark returns the initial white-on-black scheme.
func Dark() Theme { return Theme{styled: true} }

// Plain returns the no-style theme used by the sink-safety raw-output
// tests: with no decorator emitting bytes, a surviving control byte in
// composed output can only have come from external data.
func Plain() Theme { return Theme{} }

// Inverse renders s in inverse video — the match style.
func (t Theme) Inverse(s string) string {
	if !t.styled || s == "" {
		return s
	}
	return "\x1b[7m" + s + "\x1b[27m"
}

// Underline renders s underlined — the current file-list entry style.
func (t Theme) Underline(s string) string {
	if !t.styled || s == "" {
		return s
	}
	return "\x1b[4m" + s + "\x1b[24m"
}
