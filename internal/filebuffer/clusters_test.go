package filebuffer_test

import (
	"path/filepath"
	"testing"
	"unicode/utf8"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// The buffer exposes the grapheme clusters of each source line — cell
// range and terminal cell width — so the viewport wraps at cluster
// boundaries without re-deriving segmentation. A combining sequence is
// one cluster; a tab is one cluster expanded to the next multiple of
// eight source-display columns.
func TestClustersExposed(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "áb\tc\n")
	buf, err := filebuffer.Load([]byte(path), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := buf.Clusters(0)
	want := []safepresentation.Cluster{
		{Start: 0, End: 2, Width: 1},  // a + combining acute: 2 cells, 1 column
		{Start: 2, End: 3, Width: 1},  // b
		{Start: 3, End: 9, Width: 6},  // tab at column 2: 6 spaces to column 8
		{Start: 9, End: 10, Width: 1}, // c
	}
	if len(got) != len(want) {
		t.Fatalf("Clusters(0) = %+v, want %+v", got, want)
	}
	for i, c := range got {
		if c != want[i] {
			t.Fatalf("cluster %d = %+v, want %+v", i, c, want[i])
		}
	}
	if got := buf.Clusters(9); got != nil {
		t.Fatalf("Clusters out of range = %+v, want nil", got)
	}
}

// A wide CJK cluster reports its two-cell width so a wrap boundary can
// move it whole to the next row.
func TestClusterWidthWide(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "a世b\n")
	buf, err := filebuffer.Load([]byte(path), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := buf.Clusters(0)
	want := []safepresentation.Cluster{
		{Start: 0, End: 1, Width: 1},
		{Start: 1, End: 2, Width: 2},
		{Start: 2, End: 3, Width: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("Clusters(0) = %+v, want %+v", got, want)
	}
	for i, c := range got {
		if c != want[i] {
			t.Fatalf("cluster %d = %+v, want %+v", i, c, want[i])
		}
	}
}

// Tabs expand to the next multiple of eight source-display columns: the
// tab byte becomes that many space cells, all mapping back to the tab
// byte, and the following text lands on the stop. The positions are
// source-display columns of the line itself — no gutter or horizontal
// offset participates.
func TestTabStops(t *testing.T) {
	for _, tc := range []struct {
		name     string
		content  string
		wantText string
		// Index of the cell after the tab's spaces — the cell the
		// following text occupies — and its source-display column.
		afterIdx, afterCol int
	}{
		{"tab at line start", "\tx\n", "        x", 8, 8},
		{"tab at column one", "a\tb\n", "a       b", 8, 8},
		{"tab at column two", "ab\tc\n", "ab      c", 8, 8},
		{"tab after wide rune", "世\tx\n", "世      x", 7, 8},
		{"tab at a stop expands full", "abcdefgh\tx\n", "abcdefgh        x", 16, 16},
		{"consecutive tabs", "a\t\tb\n", "a               b", 16, 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), tc.content)
			buf, err := filebuffer.Load([]byte(path), nil)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			cells := buf.Cells(0)
			if got := text(cells); got != tc.wantText {
				t.Fatalf("line = %q, want %q", got, tc.wantText)
			}
			if got, want := len(cells), utf8.RuneCountInString(tc.wantText); got != want {
				t.Fatalf("cell count = %d, want %d", got, want)
			}
			if c := cells[tc.afterIdx-1]; c.Text != " " {
				t.Fatalf("cell %d = %q, want a tab-expansion space", tc.afterIdx-1, c.Text)
			}
		})
	}

	// Every space cell of a tab expansion maps back to the tab byte, so
	// a match covering the tab highlights the whole expansion.
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "a\tb\n")
	buf, err := filebuffer.Load([]byte(path), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for i := 1; i <= 7; i++ {
		c := buf.Cells(0)[i]
		if c.Start != 1 || c.End != 2 {
			t.Fatalf("tab space cell %d maps to bytes [%d,%d), want [1,2)", i, c.Start, c.End)
		}
	}
}

// TargetCell identifies the display target of a stop: its clamped
// 0-based source line and the start cell of its first coverage range —
// the cell the row model maps onto a rendered row.
func TestTargetCell(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "alpha\nbeta gam\n")
	buf, err := filebuffer.Load([]byte(path), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, tc := range []struct {
		name               string
		stop               searchindex.Stop
		wantLine, wantCell int
	}{
		{"first coverage start", stop(2, searchindex.Range{Start: 5, End: 8}), 1, 5},
		{"line start", stop(1, searchindex.Range{Start: 0, End: 3}), 0, 0},
		{"out-of-range line clamps", stop(99, searchindex.Range{Start: 0, End: 1}), 1, 0},
		{"no coverage targets the line start", stop(2), 1, 0},
		{"coverage past the line end clamps to cells", stop(2, searchindex.Range{Start: 50, End: 60}), 1, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line, cell := buf.TargetCell(tc.stop)
			if line != tc.wantLine || cell != tc.wantCell {
				t.Fatalf("TargetCell = (%d, %d), want (%d, %d)",
					line, cell, tc.wantLine, tc.wantCell)
			}
		})
	}
}

// A coverage range over escaped bytes targets the first cell of the
// escaped form: the caret pair of an ESC byte starts at its first cell.
func TestTargetCellOnEscapedBytes(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.bin"), "a\x1bb\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 1, End: 2}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	line, cell := buf.TargetCell(stop(1, searchindex.Range{Start: 1, End: 2}))
	if line != 0 || cell != 1 {
		t.Fatalf("TargetCell = (%d, %d), want (0, 1)", line, cell)
	}
}
