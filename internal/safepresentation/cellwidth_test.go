package safepresentation_test

import (
	"testing"

	"vrg/internal/safepresentation"
)

// CellWidth is the shared ANSI-aware cell measurement: grapheme
// clusters measure by the one display-width policy — a combining
// sequence is one cell, a CJK rune or emoji ZWJ sequence two — and
// 7-bit ANSI escape sequences occupy no cells.
func TestCellWidth(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "abc", 3},
		{"wide runes", "世界", 4},
		{"combining sequence is one cell", "é", 1},
		{"combining mid-string", "xéy", 3},
		{"emoji zwj sequence", "👨‍👩‍👧", 2},
		{"sgr styled text", "\x1b[31mabc\x1b[0m", 3},
		{"sgr around a wide rune", "\x1b[4m世\x1b[24m!", 3},
		{"ansi only", "\x1b[37;40m\x1b[0m", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresentation.CellWidth(tc.in); got != tc.want {
				t.Fatalf("CellWidth(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TruncateCells keeps the longest prefix of s fitting w cells: the cut
// lands only on a grapheme-cluster boundary — a wide glyph is never
// halved, a combining sequence is never parted from its base — and ANSI
// escape sequences occupy no cells, with trailing zero-width escapes
// preserved so styling cannot bleed past the truncation.
func TestTruncateCells(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		w    int
		want string
	}{
		{"fits unchanged", "abc", 3, "abc"},
		{"ascii cut", "abcdef", 3, "abc"},
		{"wide glyph never halved", "世界x", 3, "世"},
		{"wide glyph fits", "世界x", 4, "世界"},
		{"combining cluster kept whole", "éx", 1, "é"},
		{"combining cluster dropped whole", "éx", 0, ""},
		{"zwj cluster never split", "👨‍👩‍👧x", 2, "👨‍👩‍👧"},
		{"zero width", "abc", 0, ""},
		{"negative width", "abc", -2, ""},
		{"ansi sequences cost no cells", "\x1b[31mabc\x1b[0m", 2, "\x1b[31mab\x1b[0m"},
		{"ansi styled fits", "\x1b[31mab\x1b[0m", 2, "\x1b[31mab\x1b[0m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresentation.TruncateCells(tc.in, tc.w); got != tc.want {
				t.Fatalf("TruncateCells(%q, %d) = %q, want %q", tc.in, tc.w, got, tc.want)
			}
		})
	}
}

// TruncateLeftGrapheme keeps the widest suffix of s fitting w cells,
// marked with a leading ellipsis; the string is unchanged when it
// already fits. The cut lands only on a grapheme-cluster boundary: the
// kept suffix never opens with a bare combining mark and a wide glyph
// is never halved.
func TestTruncateLeftGrapheme(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		w    int
		want string
	}{
		{"fits unchanged", "a/b/c.txt", 20, "a/b/c.txt"},
		{"exact fit unchanged", "abc", 3, "abc"},
		{"ascii suffix", "dir/name.txt", 8, "…ame.txt"},
		{"zero width", "abc", 0, ""},
		{"ellipsis only", "abc", 1, "…"},
		{"combining cluster not split", "abécd", 3, "…cd"},
		{"combining cluster kept whole", "abécd", 4, "…écd"},
		{"wide glyph never halved", "ab日cd", 4, "…cd"},
		{"wide glyph fits", "ab日cd", 5, "…日cd"},
		{"zwj cluster kept whole", "ab👨‍👩‍👧cd", 4, "…cd"},
		{"zwj cluster fits", "ab👨‍👩‍👧cd", 5, "…👨‍👩‍👧cd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresentation.TruncateLeftGrapheme(tc.in, tc.w); got != tc.want {
				t.Fatalf("TruncateLeftGrapheme(%q, %d) = %q, want %q",
					tc.in, tc.w, got, tc.want)
			}
		})
	}
}

// FirstGrapheme returns the first whole grapheme cluster — the unit a
// truncation fallback emits when even one cluster does not fit the
// width, so a multi-rune cluster is never parted.
func TestFirstGrapheme(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"ascii", "abc", "a"},
		{"wide rune", "世界", "世"},
		{"combining sequence", "éx", "é"},
		{"emoji zwj sequence", "👨‍👩‍👧x", "👨‍👩‍👧"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresentation.FirstGrapheme(tc.in); got != tc.want {
				t.Fatalf("FirstGrapheme(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
