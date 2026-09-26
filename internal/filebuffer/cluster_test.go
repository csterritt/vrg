package filebuffer

import (
	"testing"

	"vrg/internal/present"
	"vrg/internal/searchindex"
)

// FileBuffer exposes the shared grapheme segmentation and cell-width
// policy through the cells of each line: Lead marks a cluster's first
// cell — the only legal wrap boundary — and Cont marks the trailing
// cells of a multi-cell unit. Viewport consumes these marks without
// re-deriving segmentation.
func TestCellClusterBoundaries(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "a世éx\x01z\n"))
	cells := b.Cells(0)
	// a | 世(lead+cont) | é cluster | x | ^A escape (lead+cont) | z
	want := []struct {
		text string
		lead bool
		cont bool
	}{
		{"a", true, false},
		{"世", true, false},
		{"", false, true},
		{"é", true, false},
		{"x", true, false},
		{"^A", true, false},
		{"", false, true},
		{"z", true, false},
	}
	if len(cells) != len(want) {
		t.Fatalf("Cells(0) = %d cells, want %d", len(cells), len(want))
	}
	for i, w := range want {
		c := cells[i]
		if c.Text != w.text || c.Lead != w.lead || c.Cont != w.cont {
			t.Fatalf("Cells(0)[%d] = %+v, want text %q lead %v cont %v",
				i, c, w.text, w.lead, w.cont)
		}
	}
}

// A standalone combining mark at line start has no base cell to join:
// it takes a provisional cell of its own on a dotted-circle base — a
// visible cell, still a cluster boundary — so a highlight on the
// mark's bytes paints a real cell rather than a bare mark a terminal
// would merge into the previous cell.
func TestLeadingCombiningCluster(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "́x\n"))
	cells := b.Cells(0)
	if len(cells) != 2 || cells[0].Text != "◌́" || !cells[0].Lead ||
		cells[1].Text != "x" || !cells[1].Lead {
		t.Fatalf("Cells(0) = %+v, want the ◌́ fallback cell then x, both leads", cells)
	}
}

// Tabs expand to the next multiple of eight source-display columns:
// each expansion cell is a real space cell, the first a cluster
// boundary, and the tab byte maps to the whole expansion so a recorded
// submatch highlights all of it.
func TestTabStopCells(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "a\tb\n"), searchindex.Stop{
		Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{{Start: 1, End: 2, Bytes: []byte("\t")}},
	})
	cells := b.Cells(0)
	if len(cells) != 9 {
		t.Fatalf("Cells(0) = %d cells, want 9", len(cells))
	}
	for i := 1; i <= 7; i++ {
		c := cells[i]
		if c.Text != " " || c.Cont || (i > 1 && c.Lead) {
			t.Fatalf("tab expansion cell %d = %+v, want a mid-cluster space", i, c)
		}
	}
	if !cells[1].Lead {
		t.Fatal("the expansion's first cell is not a cluster boundary")
	}
	if cells[8].Text != "b" || !cells[8].Lead {
		t.Fatalf("cell 8 = %+v, want the b cluster lead", cells[8])
	}
	want := []present.Span{{Start: 1, End: 8}}
	if got := b.Spans(0); len(got) != 1 || got[0] != want[0] {
		t.Fatalf("Spans(0) = %+v, want %+v over the whole expansion", got, want)
	}
}
