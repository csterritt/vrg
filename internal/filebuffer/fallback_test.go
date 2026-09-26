package filebuffer

import (
	"testing"

	"vrg/internal/present"
	"vrg/internal/searchindex"
)

// A standalone combining cluster — a zero-width cluster with no base —
// paints as the recorded Issue #43 fallback: one real cell holding
// U+25CC DOTTED CIRCLE followed by the cluster's own bytes, marked Lead
// like any cluster boundary. Standalone clusters arise at line start
// and after every break-eligible predecessor — a caret escape, a tab
// expansion, another standalone cluster — while a mark uniseg attaches
// to a replaced invalid byte still composes onto that cell: the U+FFFD
// is already a real base.
func TestStandaloneFallbackCells(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		data string
		want []present.Cell
		text string
	}{
		{"mark after a caret escape", "a\x01́x\n",
			[]present.Cell{
				{Text: "a", Lead: true},
				{Text: "^A", Lead: true},
				{Cont: true},
				{Text: "◌́", Lead: true},
				{Text: "x", Lead: true},
			}, "a^A◌́x"},
		{"mark after a tab expansion", "\t́b\n",
			[]present.Cell{
				{Text: " ", Lead: true},
				{Text: " "}, {Text: " "}, {Text: " "},
				{Text: " "}, {Text: " "}, {Text: " "}, {Text: " "},
				{Text: "◌́", Lead: true},
				{Text: "b", Lead: true},
			}, "        ◌́b"},
		{"zero-width separator after a wide glyph", "x世\u2028y\n",
			[]present.Cell{
				{Text: "x", Lead: true},
				{Text: "世", Lead: true},
				{Cont: true},
				{Text: "◌\u2028", Lead: true},
				{Text: "y", Lead: true},
			}, "x世◌\u2028y"},
		{"two standalone marks share one fallback cell", "́̂x\n",
			[]present.Cell{
				{Text: "◌́̂", Lead: true},
				{Text: "x", Lead: true},
			}, "◌́̂x"},
		{"separate standalone clusters get separate cells", "x\u2028́y\n",
			[]present.Cell{
				{Text: "x", Lead: true},
				{Text: "◌\u2028", Lead: true},
				{Text: "◌́", Lead: true},
				{Text: "y", Lead: true},
			}, "x◌\u2028◌́y"},
		{"mark on an invalid byte keeps its U+FFFD base", "a\xff́x\n",
			[]present.Cell{
				{Text: "a", Lead: true},
				{Text: "\ufffd́", Lead: true},
				{Text: "x", Lead: true},
			}, "a\ufffd́x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := load(t, writeFile(t, dir, "f", tc.data))
			cells := b.Cells(0)
			if len(cells) != len(tc.want) {
				t.Fatalf("Cells(0) for %q = %+v, want %d cells", tc.data, cells, len(tc.want))
			}
			for i, w := range tc.want {
				if cells[i] != w {
					t.Fatalf("Cells(0)[%d] for %q = %+v, want %+v", i, tc.data, cells[i], w)
				}
			}
			if b.Text(0) != tc.text {
				t.Fatalf("Text(0) for %q = %q, want %q", tc.data, b.Text(0), tc.text)
			}
		})
	}
}

// The fallback's dotted circle is a display-only prefix: the byte→cell
// map still resolves the cell to the cluster's original source bytes.
// A match covering the mark highlights exactly the fallback cell —
// nothing adjacent — the following cluster's bytes map to the next
// cell, and a zero-width position inside the cluster lands on the
// fallback cell as the cluster's start.
func TestStandaloneFallbackByteMapping(t *testing.T) {
	dir := t.TempDir()
	data := "a\x01́x\n" // a | ^A | ◌́ fallback (bytes 2–3) | x (byte 4)
	b := load(t, writeFile(t, dir, "f", data), searchindex.Stop{
		Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{
			{Start: 2, End: 4, Bytes: []byte("́")},
			{Start: 4, End: 5, Bytes: []byte("x")},
		},
	})
	want := []present.Span{{Start: 3, End: 4}, {Start: 4, End: 5}}
	got := b.Spans(0)
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Spans(0) = %+v, want %+v — the fallback cell then x's own cell", got, want)
	}
}

// A match covering a standalone cluster highlights exactly the
// fallback cell it paints — one cell, no expansion into the escape's
// cells before it or the following cluster after it.
func TestStandaloneFallbackSpanIsOneCell(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "ab\x01́cd\n"), searchindex.Stop{
		Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{
			{Start: 3, End: 5, Bytes: []byte("́")},
		},
	})
	if got := b.Spans(0); len(got) != 1 || got[0] != (present.Span{Start: 4, End: 5}) {
		t.Fatalf("Spans(0) = %+v, want exactly the fallback cell [{4 5}]", got)
	}
}
