package viewport

import (
	"slices"
	"strings"
	"testing"

	"vrg/internal/present"
)

// A marker is itself a paintable one-cell unit: it contributes to its
// line's content extent — an end-of-line marker extends the effective
// width by one cell — and joins the paintable-boundary candidates, so
// the Issue #18 maximum lets panning land the window's first column on
// the marker itself.
func TestMarkerExtentAndPanClamp(t *testing.T) {
	// "ab" with its end-of-line marker has extent 3 against "xy"'s 2:
	// the marker at cell 2 is the widest line's paintable boundary.
	src := source(t, "ab\nxy\n", map[int][]present.Span{
		0: {{Start: 2, End: 2}},
	})
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	for i := 0; i < 5; i++ {
		v.Right()
	}
	if v.Offset() != 2 {
		t.Fatalf("offset at maximum = %d, want 2 — the marker's own cell", v.Offset())
	}
	r := v.Visible()[0]
	if len(r.Cells) != 0 || !slices.Equal(r.Spans, []present.Span{{Start: 0, End: 0}}) {
		t.Fatalf("row at maximum = cells %+v spans %+v, want the marker alone at {0 0}",
			r.Cells, r.Spans)
	}

	// The marker stays a paintable boundary even past a final cluster
	// too wide for the text area: at the maximum it paints alone.
	src = source(t, "ab\xc2\x85\n", map[int][]present.Span{
		0: {{Start: 8, End: 8}},
	})
	var v2 Viewport
	v2.Resize(4, 4)
	v2.SetRows(Prepare(src, Key{Width: 4}))
	v2.SetOffset(99)
	if v2.Offset() != 8 {
		t.Fatalf("offset at maximum = %d, want 8 — the marker past the "+
			"unfittable cluster", v2.Offset())
	}
	r = v2.Visible()[0]
	if len(r.Cells) != 0 || !slices.Equal(r.Spans, []present.Span{{Start: 0, End: 0}}) {
		t.Fatalf("row at maximum = cells %+v spans %+v, want the marker alone at {0 0}",
			r.Cells, r.Spans)
	}
}

// A marker on an empty line gives the marker-only line extent 1 — one
// paintable cell at column 0 — so its paintable-boundary maximum is 0:
// panning cannot move past the marker, which already paints at offset
// zero.
func TestMarkerOnlyLineExtent(t *testing.T) {
	src := source(t, "\n\n", map[int][]present.Span{
		0: {{Start: 0, End: 0}},
		1: {{Start: 0, End: 0}},
	})
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	v.SetOffset(50)
	if v.Offset() != 0 {
		t.Fatalf("SetOffset on marker-only lines = %d, want the clamped 0", v.Offset())
	}
	for i := 0; i < 5; i++ {
		v.Right()
	}
	if v.Offset() != 0 {
		t.Fatalf("offset after pans = %d, want 0 — a marker-only line's "+
			"extent 1 gives maximum offset 0", v.Offset())
	}
	r := v.Visible()[0]
	if len(r.Cells) != 0 || !slices.Equal(r.Spans, []present.Span{{Start: 0, End: 0}}) {
		t.Fatalf("marker-only row = cells %+v spans %+v, want the marker at {0 0}",
			r.Cells, r.Spans)
	}
}

// A marker entirely hidden left or right drives the Issue #20 flags
// like any other match — including on a marker-only line, where the
// hidden marker alone upgrades the gutter signpost to "*" though no
// text exists to hide.
func TestMarkerHiddenDrivesIndicators(t *testing.T) {
	// A wide sibling permits the nonzero offsets these cases need.
	wide := strings.Repeat("x", 40) + "\n"
	src := source(t, "hit\r\n"+wide, map[int][]present.Span{
		0: {{Start: 3, End: 3}},
	})
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	v.SetOffset(10)
	if r := v.Visible()[0]; !r.MatchHiddenLeft {
		t.Fatalf("terminator marker at cell 3, offset 10: MatchHiddenLeft = %v, "+
			"want true — an entirely hidden marker", r.MatchHiddenLeft)
	}

	// The marker-only line has no text to hide — HiddenLeft stays
	// false — but its hidden marker still upgrades the signpost.
	src = source(t, "\n"+wide, map[int][]present.Span{
		0: {{Start: 0, End: 0}},
	})
	var v2 Viewport
	v2.Resize(10, 4)
	v2.SetRows(Prepare(src, Key{Width: 10}))
	v2.SetOffset(10)
	r := v2.Visible()[0]
	if r.HiddenLeft {
		t.Fatal("marker-only line: HiddenLeft = true, want false — no text to hide")
	}
	if !r.MatchHiddenLeft {
		t.Fatal("marker-only line: MatchHiddenLeft = false, want true — " +
			"the marker is entirely hidden left")
	}
}

// The terminator-only marker on "hit\r\n" — `$`'s match solely on
// removed bytes, mapped to display column 3 — is an ordinary marker in
// every shared rule: reveal, wrap, clip, extent, and indicators all
// treat it exactly like any other marker cell.
func TestTerminatorMarkerIsOrdinary(t *testing.T) {
	spans := map[int][]present.Span{0: {{Start: 3, End: 3}}}

	// Extent: the marker extends "hit" to width 4, so the pan maximum
	// is the marker's own column 3, not the last text cluster's 2.
	src := source(t, "hit\r\n", spans)
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	v.SetOffset(99)
	if v.Offset() != 3 {
		t.Fatalf("offset at maximum = %d, want 3 — the marker's cell", v.Offset())
	}

	// Reveal: the marker is a navigable target — one painted cell at
	// column 3 — so a width-3 window pans one column to show it.
	v2 := hline(t, 3, "hit\r\n", []present.Span{{Start: 3, End: 3}})
	v2.Reveal(Target{Line: 0, Cell: 3})
	if v2.Offset() != 1 {
		t.Fatalf("offset after revealing the marker = %d, want 1 = 3 + 1 − 3",
			v2.Offset())
	}
	if got := v2.Visible()[0].Spans; !slices.Equal(got, []present.Span{{Start: 2, End: 2}}) {
		t.Fatalf("clipped spans = %+v, want the marker at [{2 2}]", got)
	}

	// Clip: inside the window the marker follows its cell.
	v3 := hline(t, 3, "hit\r\n", []present.Span{{Start: 3, End: 3}})
	v3.SetOffset(1)
	r := v3.Visible()[0]
	if got := rowText(r); got != "it" {
		t.Fatalf("row at offset 1 = %q, want %q", got, "it")
	}
	if !slices.Equal(r.Spans, []present.Span{{Start: 2, End: 2}}) {
		t.Fatalf("spans at offset 1 = %+v, want [{2 2}]", r.Spans)
	}
	// Just past the window's right edge the marker is entirely hidden
	// right — the reserved column's "*" on the current line.
	v4 := hline(t, 3, "hit\r\n", []present.Span{{Start: 3, End: 3}})
	if r := v4.Visible()[0]; !r.MatchHiddenRight {
		t.Fatal("marker at cell 3 in a width-3 window: MatchHiddenRight = false, " +
			"want true — the marker is entirely hidden right")
	}

	// Wrap: "hit" fills a width-3 row completely, so the marker
	// occupies another row — a continuation row with no cells — and
	// the marker target resolves to it.
	m := Prepare(source(t, "hit\r\n", spans), Key{Width: 3, Wrap: true})
	if m.Len() != 2 {
		t.Fatalf("wrap Len = %d, want 2 — the marker overflows the full row", m.Len())
	}
	mr := m.Row(1)
	if len(mr.Cells) != 0 || !mr.Cont ||
		!slices.Equal(mr.Spans, []present.Span{{Start: 0, End: 0}}) {
		t.Fatalf("marker row = %+v, want an empty continuation carrying {0 0}", mr)
	}
	if got := m.RowOf(Target{Line: 0, Cell: 3}); got != 1 {
		t.Fatalf("RowOf(marker cell) = %d, want 1 — the marker's own row", got)
	}
}

// A marker inside a line paints its own cell on the row owning it and
// never shifts text: mid-line and beginning-of-line markers ride the
// same clip, wrap, and reveal rules as end-of-line ones.
func TestInteriorMarkers(t *testing.T) {
	// The BOL marker at cell 0 is painted at offset 0 while the
	// interior marker's cell 4 sits past the width-4 window: clipped
	// away, it counts as entirely hidden right.
	src := source(t, "abcdefgh\n", map[int][]present.Span{
		0: {{Start: 0, End: 0}, {Start: 4, End: 4}},
	})
	var v Viewport
	v.Resize(4, 4)
	v.SetRows(Prepare(src, Key{Width: 4}))
	r := v.Visible()[0]
	if !slices.Equal(r.Spans, []present.Span{{Start: 0, End: 0}}) {
		t.Fatalf("clipped spans = %+v, want only the BOL marker [{0 0}]", r.Spans)
	}
	if !r.MatchHiddenRight {
		t.Fatal("the interior marker hidden right did not set the flag")
	}
	v.SetOffset(4)
	r = v.Visible()[0]
	if got := rowText(r); got != "efgh" {
		t.Fatalf("row at offset 4 = %q, want %q", got, "efgh")
	}
	if !slices.Equal(r.Spans, []present.Span{{Start: 0, End: 0}}) {
		t.Fatalf("clipped spans = %+v, want the interior marker at [{0 0}]", r.Spans)
	}
	if !r.MatchHiddenLeft {
		t.Fatal("the BOL marker hidden left did not upgrade the gutter signpost")
	}
	if r.MatchHiddenRight {
		t.Fatal("the painted interior marker still counted as hidden right")
	}
}
