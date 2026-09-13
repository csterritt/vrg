// Package theme owns the active colour scheme and styles for the browse
// view. The dark scheme is initially active as white on black; the
// light scheme is black on white. The c key toggles between them with
// no persistence. Matches use the true inverse of the active scheme's
// base colours; current-line matches add underline. Indicators use the
// inverse style. Overlays use base colours with a plain single-line
// border. The no-style theme disables all ANSI sequences for
// sink-safety testing.
package theme

import (
	"strings"
	"unicode/utf8"
)

// Scheme identifies a colour scheme.
type Scheme int

const (
	// SchemeDark is the initially active scheme: white on black.
	SchemeDark Scheme = iota
	// SchemeLight is the alternate scheme: black on white.
	SchemeLight
)

// ANSI SGR sequences.
const (
	reset     = "\x1b[0m"
	underline = "\x1b[4m"
	// Dark scheme: white on black.
	darkBase = "\x1b[37;40m"
	// Dark match: black on white (true inverse of dark base).
	darkMatch = "\x1b[30;47m"
	// Light scheme: black on white.
	lightBase = "\x1b[30;47m"
	// Light match: white on black (true inverse of light base).
	lightMatch = "\x1b[37;40m"
)

// Theme holds the active colour scheme and derived styles. The zero
// value is the dark scheme with styling enabled.
type Theme struct {
	scheme  Scheme
	noStyle bool
}

// New returns a default theme with the dark scheme active (white on
// black) and styling enabled.
func New() Theme { return Theme{scheme: SchemeDark} }

// NoStyle returns a theme with all styling disabled. Rendering through
// the no-style theme produces no ANSI escape sequences, so any control
// byte in the output must come from unsanitized external data.
func NoStyle() Theme { return Theme{noStyle: true} }

// IsNoStyle reports whether all styling is disabled.
func (t Theme) IsNoStyle() bool { return t.noStyle }

// Scheme returns the active colour scheme.
func (t Theme) Scheme() Scheme { return t.scheme }

// Toggle flips the active scheme between dark and light with no
// persistence. The no-style theme is unchanged.
func (t Theme) Toggle() Theme {
	if t.noStyle {
		return t
	}
	t.scheme ^= 1
	return t
}

// baseSeq returns the ANSI sequence for the active scheme's base
// colour pair.
func (t Theme) baseSeq() string {
	if t.scheme == SchemeDark {
		return darkBase
	}
	return lightBase
}

// matchSeq returns the ANSI sequence for the true-inverse match colour
// pair of the active scheme.
func (t Theme) matchSeq() string {
	if t.scheme == SchemeDark {
		return darkMatch
	}
	return lightMatch
}

// currentMatchSeq returns the ANSI sequence for the true-inverse match
// colour pair with underline added.
func (t Theme) currentMatchSeq() string {
	if t.scheme == SchemeDark {
		return "\x1b[30;47;4m"
	}
	return "\x1b[37;40;4m"
}

// Base wraps s in the base colour pair for the active scheme and
// resets. With the no-style theme, s is returned unchanged.
func (t Theme) Base(s string) string {
	if t.noStyle {
		return s
	}
	return t.baseSeq() + s + reset
}

// Gutter wraps s in the gutter style, which uses the active scheme's
// base colours. With the no-style theme, s is returned unchanged.
func (t Theme) Gutter(s string) string {
	if t.noStyle {
		return s
	}
	return t.baseSeq() + s + reset
}

// Match wraps s in the true-inverse match style and restores the base
// colours so text after the match remains in base. With the no-style
// theme, s is returned unchanged.
func (t Theme) Match(s string) string {
	if t.noStyle {
		return s
	}
	return t.matchSeq() + s + t.baseSeq()
}

// CurrentMatch wraps s in the true-inverse match style with underline
// added, and restores the base colours. With the no-style theme, s is
// returned unchanged.
func (t Theme) CurrentMatch(s string) string {
	if t.noStyle {
		return s
	}
	return t.currentMatchSeq() + s + t.baseSeq()
}

// Indicator wraps s in the inverse indicator style (same colour pair
// as Match) and restores the base colours. With the no-style theme, s
// is returned unchanged.
func (t Theme) Indicator(s string) string {
	if t.noStyle {
		return s
	}
	return t.matchSeq() + s + t.baseSeq()
}

// Underline wraps s in the ANSI underline sequence and restores the
// base colours. With the no-style theme, s is returned unchanged.
func (t Theme) Underline(s string) string {
	if t.noStyle {
		return s
	}
	return underline + s + t.baseSeq()
}

// FileList wraps s in the file-list style, which uses the active
// scheme's base colours. With the no-style theme, s is returned
// unchanged.
func (t Theme) FileList(s string) string {
	if t.noStyle {
		return s
	}
	return t.baseSeq() + s + reset
}

// FilenameRule embeds the escaped filename in a horizontal rule using
// the active scheme's base colours. With the no-style theme, the rule
// is produced without ANSI sequences.
func (t Theme) FilenameRule(name string) string {
	rule := "── " + name + " ──"
	if t.noStyle {
		return rule
	}
	return t.baseSeq() + rule + reset
}

// Overlay wraps s in the overlay style: base colours with a plain
// single-line border. With the no-style theme, the content is returned
// without ANSI sequences or a border.
func (t Theme) Overlay(s string) string {
	if t.noStyle {
		return s
	}
	lines := strings.Split(s, "\n")
	maxW := 0
	for _, l := range lines {
		if w := cellWidth(l); w > maxW {
			maxW = w
		}
	}
	var b strings.Builder
	b.WriteString(t.baseSeq())
	b.WriteString("\u250c" + strings.Repeat("\u2500", maxW+2) + "\u2510\n")
	for _, l := range lines {
		pad := maxW - cellWidth(l)
		b.WriteString("\u2502 " + l + strings.Repeat(" ", pad) + " \u2502\n")
	}
	b.WriteString("\u2514" + strings.Repeat("\u2500", maxW+2) + "\u2518")
	b.WriteString(reset)
	return b.String()
}

// cellWidth returns the number of visible terminal cells in s,
// excluding ANSI escape sequences. Each rune is one cell (first-pass
// mapping pending Issue #16's width policy).
func cellWidth(s string) int {
	var w int
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			i++
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		w++
		i += size
	}
	return w
}
