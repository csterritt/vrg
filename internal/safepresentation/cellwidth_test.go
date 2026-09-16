package safepresentation_test

import (
	"testing"

	"vrg/internal/safepresentation"
)

// --- Issue #39: shared ANSI-aware grapheme/cell helper ---
//
// The shared helper is the single display-geometry policy for the
// final render path: grapheme clusters under the rivo/uniseg policy,
// measured in terminal cells, with ANSI escape sequences contributing
// zero cells and never splitting segmentation.

// TestCellWidth measures the terminal cell width of display strings
// under the shared policy.
func TestCellWidth(t *testing.T) {
	cases := []struct {
		name string
		s    string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "abc", 3},
		{"wide cjk", "中", 2},
		{"wide mixed", "a中b", 4},
		{"combining cluster", "é", 1},
		{"combining mid-string", "xéy", 3},
		{"emoji zwj", "👩‍💻", 2},
		{"ellipsis", "…", 1},
		{"box drawing", "──", 2},
		{"ansi around text", "\x1b[31mabc\x1b[0m", 3},
		{"ansi around wide", "\x1b[30;47m中\x1b[0m", 2},
		{"ansi only", "\x1b[0m", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresentation.CellWidth(tc.s); got != tc.want {
				t.Fatalf("CellWidth(%q) = %d, want %d", tc.s, got, tc.want)
			}
		})
	}
}

// TestGraphemeClustersANSI verifies that ANSI escape sequences become
// zero-width clusters at their byte positions and the text between
// them is segmented by the shared grapheme policy.
func TestGraphemeClustersANSI(t *testing.T) {
	s := "a\x1b[31m中b"
	clusters := safepresentation.GraphemeClustersANSI(s)
	want := []safepresentation.Cluster{
		{StartByte: 0, EndByte: 1, Width: 1}, // a
		{StartByte: 1, EndByte: 6, Width: 0}, // \x1b[31m
		{StartByte: 6, EndByte: 9, Width: 2}, // 中
		{StartByte: 9, EndByte: 10, Width: 1},
	}
	if len(clusters) != len(want) {
		t.Fatalf("GraphemeClustersANSI(%q) = %v, want %v", s, clusters, want)
	}
	for i, c := range clusters {
		if c != want[i] {
			t.Fatalf("cluster %d = %+v, want %+v", i, c, want[i])
		}
	}
}

// TestGraphemeClustersANSIPlain verifies that a string without ANSI
// sequences segments exactly like GraphemeClusters.
func TestGraphemeClustersANSIPlain(t *testing.T) {
	s := "xé👩‍💻中"
	got := safepresentation.GraphemeClustersANSI(s)
	want := safepresentation.GraphemeClusters(s)
	if len(got) != len(want) {
		t.Fatalf("GraphemeClustersANSI(%q) = %v, want %v", s, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cluster %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestTruncateLeftCells verifies that left truncation keeps whole
// grapheme clusters and never splits them, measured in cells.
func TestTruncateLeftCells(t *testing.T) {
	cases := []struct {
		name string
		s    string
		keep int
		want string
	}{
		{"ascii fits", "hello", 5, "hello"},
		{"ascii keep", "hello", 3, "llo"},
		{"keep zero", "hello", 0, ""},
		{"keep negative", "hello", -1, ""},
		{"wide never split", "a中中b", 3, "中b"},
		{"wide boundary", "a中中b", 5, "中中b"},
		{"combining kept whole", "xéy", 2, "éy"},
		{"combining dropped whole", "éab", 2, "ab"},
		{"emoji zwj kept whole", "a👩‍💻b", 3, "👩‍💻b"},
		{"emoji zwj dropped whole", "👩‍💻ab", 2, "ab"},
		{"ansi kept tail", "\x1b[31mabc\x1b[0m", 2, "bc\x1b[0m"},
		{"ansi before kept text", "x\x1b[31mab", 2, "\x1b[31mab"},
		{"ansi not counted", "x\x1b[31mab", 5, "x\x1b[31mab"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresentation.TruncateLeftCells(tc.s, tc.keep); got != tc.want {
				t.Fatalf("TruncateLeftCells(%q, %d) = %q, want %q", tc.s, tc.keep, got, tc.want)
			}
		})
	}
}
