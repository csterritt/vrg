package theme

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// scheme is one colour scheme's base foreground/background SGR pair:
// the dark scheme is white on black, the light scheme black on white.
type scheme struct{ fg, bg int }

var (
	dark  = scheme{fg: 37, bg: 40}
	light = scheme{fg: 30, bg: 47}
)

// inverse returns the pair's true inverse. A standard SGR foreground
// colour n is the background colour n+10, so the inverse of white on
// black is black on white.
func (s scheme) inverse() scheme {
	return scheme{fg: s.bg - 10, bg: s.fg + 10}
}

// Theme carries the active colour scheme and the style set every
// rendered element draws through. The dark scheme is initially active;
// Toggle swaps schemes in memory only — nothing is persisted. Plain is
// the no-style composition path used by the sink-safety tests: every
// style is the identity, so rendered output contains no SGR bytes.
//
// Styles emit explicit colour pairs rather than bare SGR 7 so each is
// correct independent of ambient state, and every style restores the
// scheme's base pair afterwards so nested styles compose inside a
// Base-painted frame.
type Theme struct {
	s     scheme
	plain bool
}

// Dark returns a Theme with the dark scheme active: white on black.
func Dark() Theme { return Theme{s: dark} }

// Plain returns the no-style composition path: every style is the
// identity, so rendered output contains no SGR bytes at all.
func Plain() Theme { return Theme{s: dark, plain: true} }

// Toggle returns the Theme with the other scheme active.
func (t Theme) Toggle() Theme {
	if t.s == dark {
		t.s = light
	} else {
		t.s = dark
	}
	return t
}

// Light reports whether the light scheme is active.
func (t Theme) Light() bool { return t.s == light }

// pair renders a scheme's SGR parameter list.
func (t Theme) pair(s scheme) string {
	return fmt.Sprintf("%d;%d", s.fg, s.bg)
}

// paint wraps s in the SGR parameter list on, then restores off.
// Identity under Plain and for empty input.
func (t Theme) paint(s, on, off string) string {
	if t.plain || s == "" {
		return s
	}
	return "\x1b[" + on + "m" + s + "\x1b[" + off + "m"
}

// Base paints s in the active scheme's colours and resets all styling
// at the end. It wraps each composed frame.
func (t Theme) Base(s string) string {
	return t.paint(s, t.pair(t.s), "0")
}

// Gutter paints line-number gutter text in the base colours.
func (t Theme) Gutter(s string) string {
	return t.paint(s, t.pair(t.s), t.pair(t.s))
}

// FileList paints a file-list entry in the base colours.
func (t Theme) FileList(s string) string {
	return t.paint(s, t.pair(t.s), t.pair(t.s))
}

// FilenameRule paints the filename rule in the base colours.
func (t Theme) FilenameRule(s string) string {
	return t.paint(s, t.pair(t.s), t.pair(t.s))
}

// Match paints a match in the true inverse of the active scheme's base
// colours.
func (t Theme) Match(s string) string {
	return t.paint(s, t.pair(t.s.inverse()), t.pair(t.s))
}

// CurrentMatch paints a match on the current matched line: the inverse
// pair plus underline.
func (t Theme) CurrentMatch(s string) string {
	return t.paint(s, t.pair(t.s.inverse())+";4", "24;"+t.pair(t.s))
}

// Indicator paints a hidden-content indicator in the inverse pair.
func (t Theme) Indicator(s string) string {
	return t.paint(s, t.pair(t.s.inverse()), t.pair(t.s))
}

// CurrentFile underlines the current file-list entry. Underline is an
// attribute, so the ambient base colours are untouched.
func (t Theme) CurrentFile(s string) string {
	return t.paint(s, "4", "24")
}

// Overlay frames inner lines in a plain single-line border painted in
// the base colours: ┌─┐ above, │ sides, └─┘ below, with each line
// padded to the widest. Under Plain the box is drawn without styling.
func (t Theme) Overlay(inner []string) string {
	w := 0
	for _, l := range inner {
		if n := ansi.StringWidth(l); n > w {
			w = n
		}
	}
	var b strings.Builder
	b.WriteString("┌" + strings.Repeat("─", w) + "┐")
	for _, l := range inner {
		b.WriteString("\n│" + l + strings.Repeat(" ", w-ansi.StringWidth(l)) + "│")
	}
	b.WriteString("\n└" + strings.Repeat("─", w) + "┘")
	return t.paint(b.String(), t.pair(t.s), t.pair(t.s))
}
