// Package theme owns the active colour scheme and styles.
//
// Issue #7 lands the full seam: the dark and light schemes with the c
// colour toggle (no persistence), the true-inverse match styles, the
// underlined current file-list entry, and the style set rendering
// consumes — base, gutter, match, current match, indicator, overlay,
// filename rule, and file list. Plain remains the no-style composition
// path the sink-safety tests require.
package theme

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Theme is the active scheme's style set. The zero value is the
// no-style composition path: every decorator is the identity, so no
// escape byte may legitimately appear in composed output.
//
// fg and bg are the scheme's base colours as SGR colour parameters:
// dark is white on black (37/40), light black on white (30/47). Every
// styled run closes by re-asserting the base colours and clearing
// underline, so runs compose inside a Base-styled frame without
// leaking attributes into the cells that follow.
type Theme struct {
	fg, bg byte
	styled bool
}

// Dark returns the initially active scheme: white on black.
func Dark() Theme { return Theme{fg: 37, bg: 40, styled: true} }

// Light returns the alternate scheme: black on white.
func Light() Theme { return Theme{fg: 30, bg: 47, styled: true} }

// Plain returns the no-style theme used by the sink-safety raw-output
// tests: with no decorator emitting bytes, a surviving control byte in
// composed output can only have come from external data.
func Plain() Theme { return Theme{} }

// Toggled returns the theme with the other scheme active — dark↔light.
// The c key flips it for the session only; nothing persists. The
// no-style theme has no scheme to flip and is unchanged.
func (t Theme) Toggled() Theme {
	switch t {
	case Dark():
		return Light()
	case Light():
		return Dark()
	}
	return t
}

// Base renders s in the scheme's base colours — the outermost style of
// every frame, so padding cells carry the background too.
func (t Theme) Base(s string) string {
	if !t.styled || s == "" {
		return s
	}
	return "\x1b[" + t.baseParams() + "m" + s + "\x1b[0m"
}

// Gutter renders the line-number gutter in the base colours.
func (t Theme) Gutter(s string) string { return t.style(s, t.baseParams()) }

// Match renders s in the true inverse of the base colours — the match
// style.
func (t Theme) Match(s string) string { return t.style(s, t.inverseParams()) }

// CurrentMatch renders a match on the current matched line: the true
// inverse plus underline.
func (t Theme) CurrentMatch(s string) string { return t.style(s, t.inverseParams()+";4") }

// Indicator renders a hidden-content indicator in the inverse style.
func (t Theme) Indicator(s string) string { return t.style(s, t.inverseParams()) }

// FilenameRule renders the filename rule in the base colours.
func (t Theme) FilenameRule(s string) string { return t.style(s, t.baseParams()) }

// FileList renders a file-list entry in the base colours.
func (t Theme) FileList(s string) string { return t.style(s, t.baseParams()) }

// CurrentFile renders the current file-list entry: the base colours
// plus underline.
func (t Theme) CurrentFile(s string) string { return t.style(s, t.baseParams()+";4") }

// Overlay frames rows in the overlay style — the base colours inside a
// plain single-line border — and returns the bordered rows, every row
// the same cell width. Rows are the interior content, already wrapped
// and clipped to fit; short rows pad to the widest. On the no-style
// path the border still draws: it is structure, not styling.
func (t Theme) Overlay(rows []string) []string {
	w := 0
	for _, r := range rows {
		if cw := cellWidth(r); cw > w {
			w = cw
		}
	}
	edge := strings.Repeat("─", w+2)
	out := make([]string, 0, len(rows)+2)
	out = append(out, t.style("┌"+edge+"┐", t.baseParams()))
	for _, r := range rows {
		row := "│ " + r + strings.Repeat(" ", w-cellWidth(r)) + " │"
		out = append(out, t.style(row, t.baseParams()))
	}
	return append(out, t.style("└"+edge+"┘", t.baseParams()))
}

// style renders s inside the given SGR parameters, then restores the
// base colours and clears underline so the next cells resume the frame.
// The no-style theme and the empty string pass through unchanged.
func (t Theme) style(s, params string) string {
	if !t.styled || s == "" {
		return s
	}
	return "\x1b[" + params + "m" + s + "\x1b[" + t.baseParams() + ";24m"
}

// baseParams is the scheme's colour pair as SGR parameters.
func (t Theme) baseParams() string {
	return strconv.Itoa(int(t.fg)) + ";" + strconv.Itoa(int(t.bg))
}

// inverseParams is the true inverse of the base pair — the background
// colour as foreground and vice versa; SGR colour parameters map
// 3x↔4x by ±10.
func (t Theme) inverseParams() string {
	return strconv.Itoa(int(t.bg-10)) + ";" + strconv.Itoa(int(t.fg+10))
}

// cellWidth measures s in terminal cells for overlay sizing.
func cellWidth(s string) int { return utf8.RuneCountInString(s) }
