package safepresentation

import (
	"strings"
	"unicode/utf8"

	"github.com/clipperhouse/displaywidth"
)

// cellMeasure is the shared display-geometry policy: text segments into
// grapheme clusters by the upstream UAX #29 tables, each cluster
// measures its terminal cell width, and 7-bit ECMA-48 (ANSI) escape
// sequences occupy no cells.
var cellMeasure = displaywidth.Options{ControlSequences: true}

// CellWidth returns s's terminal cell width: the sum of its grapheme
// clusters' measured widths, with ANSI escape sequences contributing
// nothing. Every display-width consumer — padding, centring, fitting,
// overlay sizing — measures through here so one policy decides.
func CellWidth(s string) int {
	return cellMeasure.String(s)
}

// TruncateCells returns the longest prefix of s fitting w cells, or ""
// for a nonpositive width. The cut lands only on a grapheme-cluster
// boundary — a wide cluster is never halved and a combining sequence
// never parts from its base. ANSI escape sequences occupy no cells, and
// a zero-width escape past the cut — a style reset — is preserved so
// styling cannot bleed.
func TruncateCells(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return cellMeasure.TruncateString(s, w, "")
}

// TruncateLeftGrapheme keeps the widest suffix of s fitting w cells,
// marked with a leading ellipsis; s is unchanged when it already fits.
// The cut lands only on a grapheme-cluster boundary: a kept suffix
// never opens with a bare combining mark and a wide glyph is never
// halved.
func TruncateLeftGrapheme(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if CellWidth(s) <= w {
		return s
	}
	g := cellMeasure.StringGraphemes(s)
	var clusters []string
	var widths []int
	for g.Next() {
		clusters = append(clusters, g.Value())
		widths = append(widths, g.Width())
	}
	width := 1 // the ellipsis
	i := len(clusters)
	for i > 0 && width+widths[i-1] <= w {
		width += widths[i-1]
		i--
	}
	return "…" + strings.Join(clusters[i:], "")
}

// FirstGrapheme returns s's first grapheme cluster, or "" for an empty
// string. It is the fallback unit a wrap or truncation emits when even
// one cluster cannot fit the width, so a multi-rune cluster is never
// parted.
func FirstGrapheme(s string) string {
	g := cellMeasure.StringGraphemes(s)
	if g.Next() {
		return g.Value()
	}
	return ""
}

// decodeRuneInString decodes the first rune of s — the one production
// utf8.DecodeRuneInString call site, confined to this file by the
// static allow-list guard so no other package can build a
// rune-decoding width or truncation loop. Every other file decodes
// through for range or the helpers above.
func decodeRuneInString(s string) (r rune, size int) {
	return utf8.DecodeRuneInString(s)
}
