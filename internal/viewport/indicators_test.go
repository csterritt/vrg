package viewport_test

import (
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// markRow builds the flat-mode row over content with the given
// highlight cell ranges — the row spans the line's whole cell range.
func markRow(content string, spans ...searchindex.Span) viewport.Row {
	l := filebuffer.Line{
		Mapped:     safepresentation.MapContent([]byte(content)),
		Number:     1,
		Highlights: spans,
	}
	return viewport.Row{Line: l, Start: 0, End: len(l.Cells)}
}

// The left mark is '*' when a match or marker on the line is entirely
// hidden left, '_' when text is hidden left but no match is entirely
// hidden there, and blank when nothing is hidden left — judged by the
// painted-cell visibility, so a clipped grapheme's blanked cells count
// as hidden.
func TestLeftMark(t *testing.T) {
	cases := []struct {
		name    string
		content string
		spans   []searchindex.Span
		off     int
		want    byte
	}{
		{"nothing hidden at the edge", "hello", spans(2, 4), 0, ' '},
		{"text hidden left, no match", "hello", nil, 2, '_'},
		{"empty row hides nothing", "", nil, 2, ' '},
		{"match entirely hidden left", "abcdef", spans(0, 3), 4, '*'},
		{"match partially hidden left", "abcdef", spans(0, 3), 2, '_'},
		{"match painted inside the window", "abcdef", spans(0, 3), 0, ' '},
		{"match hidden right is not hidden left", "abcdef", spans(5, 6), 2, '_'},
		// The 文 cluster spans cells 2–4; at offset 3 both its cells
		// are hidden left even though cell 3 sits inside the window —
		// the clipped blank does not count as visible.
		{"clipped grapheme match hidden left", "ab文xx", spans(2, 4), 3, '*'},
		{"painted grapheme match visible", "ab文xx", spans(2, 4), 2, '_'},
		// An empty span is a one-cell marker position: the marker at
		// cell 2 is hidden left at offset 3.
		{"zero-width marker hidden left", "abcdef", spans(2, 2), 3, '*'},
		{"marker at the window edge painted", "abcdef", spans(3, 3), 3, '_'},
	}
	for _, c := range cases {
		if got := viewport.LeftMark(markRow(c.content, c.spans...), c.off); got != c.want {
			t.Errorf("%s: LeftMark = %q, want %q", c.name, got, c.want)
		}
	}
}

// The right mark is '*' when a match or marker on the line is entirely
// hidden right of the window — a partially visible match, even one
// painted cell, earns no star, and a clipped grapheme's blanked cells
// count as hidden.
func TestRightMark(t *testing.T) {
	cases := []struct {
		name    string
		content string
		spans   []searchindex.Span
		off     int
		width   int
		want    byte
	}{
		{"match fully painted", "abcdef", spans(0, 3), 0, 4, ' '},
		{"match entirely hidden right", "abcdef", spans(4, 6), 0, 4, '*'},
		{"match partially hidden right", "abcdef", spans(3, 6), 0, 4, ' '},
		{"match hidden left is not hidden right", "abcdef", spans(0, 2), 4, 4, ' '},
		{"no match", "abcdef", nil, 0, 4, ' '},
		// The 文 cluster spans cells 3–5; at width 4 it clips to a
		// blank inside the window — entirely hidden right.
		{"clipped grapheme match hidden right", "abc文xx", spans(3, 5), 0, 4, '*'},
		{"painted grapheme match visible", "abc文xx", spans(3, 5), 1, 4, ' '},
		// A match in the last text cell paints; the star still comes
		// from the farther match entirely beyond the edge.
		{"last-cell match painted, farther match hidden", "abcdefghij",
			[]searchindex.Span{{Start: 3, End: 4}, {Start: 8, End: 9}}, 0, 4, '*'},
		{"zero-width marker hidden right", "abcdef", spans(6, 6), 0, 4, '*'},
		{"marker at the window edge painted", "abcdef", spans(3, 3), 0, 4, ' '},
	}
	for _, c := range cases {
		if got := viewport.RightMark(markRow(c.content, c.spans...), c.off, c.width); got != c.want {
			t.Errorf("%s: RightMark = %q, want %q", c.name, got, c.want)
		}
	}
}

// markRowOf loads content through the real FileBuffer path with the
// given recorded byte spans, so the line's highlights arrive as the
// cluster-expanded cell spans production indicators consume — the
// Issue #21 single span source.
func markRowOf(t *testing.T, content string, byteSpans ...[2]int) viewport.Row {
	t.Helper()
	st := searchindex.Stop{Number: 1}
	for _, s := range byteSpans {
		st.Highlights = append(st.Highlights, searchindex.Span{Start: s[0], End: s[1]})
	}
	l := loadBufferStops(t, content, st).Lines()[0]
	return viewport.Row{Line: l, Start: 0, End: len(l.Cells)}
}

// A match whose recorded bytes start mid-cluster is indicator-counted
// from the cluster start — the expanded span is what FileBuffer hands
// to the marks: when that start is hidden left the whole match counts
// hidden left, and symmetrically for a mid-cluster end hidden right.
func TestMidClusterMatchCountsFromClusterBoundary(t *testing.T) {
	// 文 is bytes [2,5) over cells [2,4): a recorded match on bytes
	// [3,4) starts inside the cluster and expands to its whole cell
	// range.
	row := markRowOf(t, "ab文xx", [2]int{3, 4})
	if l := row.Line; len(l.Highlights) != 1 || l.Highlights[0].Start != 2 || l.Highlights[0].End != 4 {
		t.Fatalf("expanded highlights = %v, want the whole 文 cluster [{2 4}]", l.Highlights)
	}
	// At offset 3 the cluster's start is hidden left — the match
	// counts entirely hidden left even though a span cell sits in
	// the window's geometry.
	if got := viewport.LeftMark(row, 3); got != '*' {
		t.Fatalf("LeftMark = %q, want '*' — a mid-cluster start counts from the cluster start", got)
	}
	// The mirror image: a window ending inside the cluster hides the
	// whole match right.
	if got := viewport.RightMark(row, 0, 3); got != '*' {
		t.Fatalf("RightMark = %q, want '*' — a mid-cluster end counts from the cluster end", got)
	}
	// Painted whole, the same expanded span earns no star.
	if got := viewport.LeftMark(row, 0); got != ' ' {
		t.Fatalf("LeftMark at offset 0 = %q, want blank — the cluster is fully painted", got)
	}
	if got := viewport.RightMark(row, 0, 6); got != ' ' {
		t.Fatalf("RightMark over the whole line = %q, want blank — the cluster is painted", got)
	}
}

// The marks hold on a uniform-lines layout too: at a nonzero offset
// every prepared row reports text hidden left, so '_' shows on every
// visible line.
func TestLeftMarkOnEveryFlatRow(t *testing.T) {
	rows := viewport.Prepare(
		loadBuffer(t, strings.Repeat(strings.Repeat("x", 100)+"\n", 30)),
		viewport.Key{TextWidth: 40},
	)
	for i := 0; i < rows.Len(); i++ {
		if got := viewport.LeftMark(rows.At(i), 20); got != '_' {
			t.Fatalf("row %d: LeftMark = %q, want '_'", i, got)
		}
	}
}

func spans(lo, hi int) []searchindex.Span {
	return []searchindex.Span{{Start: lo, End: hi}}
}
