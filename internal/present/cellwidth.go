package present

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// This file is the single authority for terminal display geometry:
// every consumer that measures, truncates, cuts, or wraps text for the
// screen routes through these functions so the whole renderer shares
// one ANSI-aware grapheme/cell policy. The policy: text is segmented
// into grapheme clusters (the rivo/uniseg rules, including
// base-plus-combining-mark and emoji ZWJ sequences); each cluster
// occupies the cells of its widest glyph; escape and control sequences
// paint no cells. No display-geometry code anywhere else decodes
// runes, so utf8.DecodeRuneInString is permitted in this file alone
// and the test suite enforces that mechanically.

// CellWidth reports how many terminal display cells s paints, summing
// grapheme-cluster widths and skipping escape and control sequences.
func CellWidth(s string) int {
	return ansi.StringWidth(s)
}

// TruncateLeft fits s into n display cells. When s already fits it is
// returned unchanged; otherwise whole leading clusters drop until the
// remainder fits behind a one-cell "…" marker. The cut lands only on a
// grapheme boundary — a cluster straddling it drops whole rather than
// painting half a glyph — and escape sequences are preserved wherever
// they fall so kept text retains its styling state.
func TruncateLeft(s string, n int) string {
	if n <= 0 {
		return ""
	}
	w := CellWidth(s)
	if w <= n {
		return s
	}
	// The "…" marker takes one cell; the kept suffix fits in n-1.
	drop := w - n + 1
	var b strings.Builder
	b.Grow(len(s) + 3)
	b.WriteString("…")
	acc := 0
	for i := 0; i < len(s); {
		r, _ := utf8.DecodeRuneInString(s[i:])
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			// Control or escape start: sequences paint no cells but
			// stay in the output so the kept text's styling state
			// survives the cut.
			if seq, _, n, _ := ansi.DecodeSequence(s[i:], 0, nil); n > 0 {
				b.WriteString(seq)
				i += n
				continue
			}
		}
		cl, cw := ansi.FirstGraphemeCluster(s[i:], ansi.GraphemeWidth)
		i += len(cl)
		if acc < drop {
			acc += cw
			continue
		}
		b.WriteString(cl)
	}
	return b.String()
}

// Truncate fits s into length display cells, appending tail when it
// must cut, under the shared grapheme/cell policy.
func Truncate(s string, length int, tail string) string {
	return ansi.Truncate(s, length, tail)
}

// Cut returns the part of s painting cells in [left, right) under the
// shared grapheme/cell policy; a cluster straddling a window edge does
// not paint.
func Cut(s string, left, right int) string {
	return ansi.Cut(s, left, right)
}

// Wrap wraps s to lines of at most limit cells, breaking at
// breakpoints, under the shared grapheme/cell policy.
func Wrap(s string, limit int, breakpoints string) string {
	return ansi.Wrap(s, limit, breakpoints)
}
