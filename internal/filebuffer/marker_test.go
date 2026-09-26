package filebuffer

import (
	"testing"

	"vrg/internal/present"
	"vrg/internal/searchindex"
)

// A zero-width submatch — `^`, `$`, an empty look-around — validates
// like any other recorded match (its Bytes are the empty string at
// the position) and becomes a marker position rather than a coverage
// span: one marked cell at the mapped location, never shifting
// following text. The position is a raw byte offset into the line's
// retained bytes, mapped through the byte→cell map: inside a cluster
// it lands on the cluster's first cell, and on removed terminator
// bytes — or at the end of the line — it lands on the display
// end-of-line position.
func TestZeroWidthMarkerPositions(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name       string
		data       string
		start, end int
		want       present.Span
	}{
		{"bol on a text line", "hit\n", 0, 0, present.Span{Start: 0, End: 0}},
		{"bol on an empty line", "\n", 0, 0, present.Span{Start: 0, End: 0}},
		{"eol on a text line", "hit\n", 3, 3, present.Span{Start: 3, End: 3}},
		{"mid-line position", "hit\n", 1, 1, present.Span{Start: 1, End: 1}},
		// 世 is bytes 1–3 and cells 1–2 of "a世b\n": a position on any
		// of its bytes marks the cluster's first cell — the glyph is
		// never split.
		{"inside a wide cluster's first byte", "a世b\n", 1, 1, present.Span{Start: 1, End: 1}},
		{"inside a wide cluster's middle byte", "a世b\n", 2, 2, present.Span{Start: 1, End: 1}},
		{"inside a wide cluster's last byte", "a世b\n", 3, 3, present.Span{Start: 1, End: 1}},
		// The emoji ZWJ sequence is one two-cell cluster starting at
		// cell 1: an interior position marks cell 1.
		{"inside a zwj cluster", "a👨‍👩‍👧b\n", 6, 6, present.Span{Start: 1, End: 1}},
		// A position on removed terminator bytes — or solely covering
		// them — is an end-of-line marker: byte 4 of "hit\r\n" is the
		// LF, bytes 3–5 the whole CRLF; all land on display column 3.
		{"at the lf of a crlf", "hit\r\n", 4, 4, present.Span{Start: 3, End: 3}},
		{"covering a crlf", "hit\r\n", 3, 5, present.Span{Start: 3, End: 3}},
		{"covering an lf", "hit\n", 3, 4, present.Span{Start: 3, End: 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			match := tc.data[tc.start:tc.end]
			b := load(t, writeFile(t, dir, "f", tc.data), searchindex.Stop{
				Path: []byte("f"), Line: 1,
				Submatches: []searchindex.Submatch{
					{Start: tc.start, End: tc.end, Bytes: []byte(match)},
				},
			})
			got := b.Spans(0)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("Spans(0) for %q[%d:%d] = %+v, want the single marker %+v",
					tc.data, tc.start, tc.end, got, tc.want)
			}
		})
	}
}

// `vrg '^' file` marks column 0 of every line — empty lines included —
// and `vrg '$' file` marks each line's end-of-line position: the
// per-line markers of one stop set accumulate per line.
func TestMarkersOnEveryLine(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "ab\n\ncd\n"),
		searchindex.Stop{Path: []byte("f"), Line: 1,
			Submatches: []searchindex.Submatch{{Start: 0, End: 0}}},
		searchindex.Stop{Path: []byte("f"), Line: 2,
			Submatches: []searchindex.Submatch{{Start: 0, End: 0}}},
		searchindex.Stop{Path: []byte("f"), Line: 3,
			Submatches: []searchindex.Submatch{{Start: 0, End: 0}}})
	for i := 0; i < 3; i++ {
		want := present.Span{Start: 0, End: 0}
		if got := b.Spans(i); len(got) != 1 || got[0] != want {
			t.Fatalf("Spans(%d) = %+v, want the BOL marker %+v", i, got, want)
		}
	}
}
