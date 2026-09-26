package filebuffer

import (
	"testing"

	"vrg/internal/present"
	"vrg/internal/searchindex"
)

// A recorded submatch whose mapped span lands inside a grapheme
// cluster expands outward to the whole cluster's cells, so a
// highlight never splits a glyph. Single-unit clusters — a wide rune,
// an emoji ZWJ sequence — already map whole through the byte→cell map;
// a standalone zero-width cluster instead receives the Issue #43
// fallback cell of its own, so a match on it highlights exactly that
// cell with nothing to expand into.
func TestSpansExpandToWholeClusters(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name       string
		data       string
		start, end int
		want       present.Span
	}{
		// Decomposed "café": the combining mark's own cluster joins
		// the 'e' cell, so a combining-only match covers that whole
		// cell — the entire é glyph.
		{"combining-only match in a base cluster", "café\n", 4, 6, present.Span{Start: 3, End: 4}},
		// "a^Áx": the mark's standalone zero-width cluster takes
		// the ◌ fallback cell after the ^A escape's two cells, so the
		// match covers exactly that one cell.
		{"standalone mark after an escape", "a\x01́x\n", 2, 4, present.Span{Start: 3, End: 4}},
		// The same match continuing past the fallback cell covers the
		// following cluster as well.
		{"standalone mark plus following text", "a\x01́x\n", 2, 5, present.Span{Start: 3, End: 5}},
		// "a\t́b": the mark's fallback cell stands at cell 8, after
		// the tab's seven expansion cells.
		{"standalone mark after a tab expansion", "a\t́b\n", 2, 4, present.Span{Start: 8, End: 9}},
		// A standalone zero-width rune after a wide glyph takes its
		// own fallback cell rather than the glyph's trailing cell.
		{"standalone zero-width rune after a wide glyph", "x世\u2028y\n", 4, 7, present.Span{Start: 3, End: 4}},
		// A mark uniseg attaches to a replaced invalid byte is not
		// standalone — it composes onto the U+FFFD base, so the match
		// lands on that cell's own cluster.
		{"mark joins a replaced byte's own cell", "a\xff́x\n", 2, 4, present.Span{Start: 1, End: 2}},
		// A match inside a two-cell cluster covers both cells — a
		// wide pair is never split by a highlight boundary.
		{"interior byte of a wide pair", "a世b\n", 2, 3, present.Span{Start: 1, End: 3}},
		// The emoji ZWJ sequence is one two-cell cluster under the
		// shared policy: a match on an inner emoji covers both cells.
		{"interior bytes of a ZWJ sequence", "a👨‍👩‍👧b\n", 8, 12, present.Span{Start: 1, End: 3}},
		// A standalone combining cluster at line start takes the same
		// fallback cell — one visible cell, never zero cells.
		{"standalone mark gets a fallback cell", "́x\n", 0, 2, present.Span{Start: 0, End: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			match := tc.data[tc.start:tc.end]
			b := load(t, writeFile(t, dir, "f", tc.data), searchindex.Stop{
				Path: []byte("f"), Line: 1,
				Submatches: []searchindex.Submatch{
					{Start: tc.start, End: tc.end, Bytes: []byte(match)},
				},
			})
			if got := b.Spans(0); len(got) != 1 || got[0] != tc.want {
				t.Fatalf("Spans(0) for %q[%d:%d] = %+v, want the cluster-expanded %+v",
					tc.data, tc.start, tc.end, got, tc.want)
			}
		})
	}
}

// clusterSpan is the cell-space expansion itself: over the cells' Lead
// marks it walks a nonempty span's start back to its cluster's first
// cell and its end forward to the next cluster boundary — covering
// start-inside, end-inside, and strictly interior partial coverage —
// while whole-cluster spans and zero-width marker positions stay
// unchanged.
func TestClusterSpanBoundaries(t *testing.T) {
	// A three-cell cluster between single-cell clusters: a lead at
	// cell 1, two continuation cells, then the next lead at 4.
	cells := []present.Cell{
		{Text: "a", Lead: true},
		{Text: "x", Lead: true},
		{Cont: true},
		{Cont: true},
		{Text: "z", Lead: true},
	}
	for _, tc := range []struct {
		name string
		in   present.Span
		want present.Span
	}{
		{"start inside expands left", present.Span{Start: 2, End: 4}, present.Span{Start: 1, End: 4}},
		{"end inside expands right", present.Span{Start: 1, End: 3}, present.Span{Start: 1, End: 4}},
		{"interior expands both ways", present.Span{Start: 2, End: 3}, present.Span{Start: 1, End: 4}},
		{"whole cluster unchanged", present.Span{Start: 1, End: 4}, present.Span{Start: 1, End: 4}},
		{"neighbour span unchanged", present.Span{Start: 0, End: 1}, present.Span{Start: 0, End: 1}},
		{"span to the line end keeps its end", present.Span{Start: 2, End: 5}, present.Span{Start: 1, End: 5}},
		{"marker stays a position", present.Span{Start: 2, End: 2}, present.Span{Start: 2, End: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := clusterSpan(cells, tc.in); got != tc.want {
				t.Fatalf("clusterSpan(%+v) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}
