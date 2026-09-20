package safepresentation

import (
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
	return MeasureText(s).TruncateLeft(w)
}

// measuredCluster is one grapheme cluster's start byte inside a
// measured string and the cluster's terminal cell width; contiguous
// segmentation makes a cluster's end the next cluster's start.
type measuredCluster struct {
	start, width int
}

// MeasuredText is a display string segmented once under the shared cell
// policy: its grapheme-cluster boundaries and total cell width are
// recorded up front so repeated width queries and left-truncations —
// file-list entries refitted to a changing column every frame — reuse
// the segmentation instead of re-deriving it (Issue 40).
type MeasuredText struct {
	s        string
	clusters []measuredCluster
	width    int
}

// MeasureText records s's grapheme-cluster boundaries and total
// terminal cell width.
func MeasureText(s string) MeasuredText {
	t := MeasuredText{s: s}
	g := cellMeasure.StringGraphemes(s)
	pos := 0
	for g.Next() {
		c := g.Value()
		w := g.Width()
		t.clusters = append(t.clusters, measuredCluster{start: pos, width: w})
		t.width += w
		pos += len(c)
	}
	return t
}

// String returns the measured text.
func (t MeasuredText) String() string { return t.s }

// Width returns the text's total terminal cell width.
func (t MeasuredText) Width() int { return t.width }

// TruncateLeft keeps the widest suffix of the text fitting w cells,
// marked with a leading ellipsis, and returns the text unchanged when
// it already fits. The cut is evaluated against the recorded clusters,
// so it still lands only on a grapheme-cluster boundary.
func (t MeasuredText) TruncateLeft(w int) string {
	if w <= 0 {
		return ""
	}
	if t.width <= w {
		return t.s
	}
	width := 1 // the ellipsis
	i := len(t.clusters)
	for i > 0 && width+t.clusters[i-1].width <= w {
		width += t.clusters[i-1].width
		i--
	}
	if i == len(t.clusters) {
		return "…"
	}
	return "…" + t.s[t.clusters[i].start:]
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
