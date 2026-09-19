package app

import (
	"strings"
	"testing"
)

// Issue #21: grapheme-cluster highlight expansion and wide-glyph
// safety — the rendering half. The expanded spans FileBuffer records
// are the only highlight source: a whole cluster styles together, and
// the blank cells Issues #16 (wrap) and #18 (clip) introduce are
// filler, never match cells.

// A cluster split by a clip edge renders its in-window cells as clip
// blanks — and those blanks are never painted as match cells. A match
// on the clipped cluster styles nothing in the window: the inverse
// style belongs to painted glyph cells alone.
func TestClipBlanksNeverPaintMatchCells(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{{
		name: "a.txt",
		// Line 1's 文 spans cells 39–40: at offset 0 it clips at the
		// right edge; at offset 3 it paints whole.
		// Line 2's 文 spans cells 2–3: at offset 3 it clips at the
		// left edge — its one in-window cell is a clip blank.
		// Line 3's tab expands cells 1–7 as one cluster: at offset 3
		// cells 3–7 are in-window clip blanks.
		content: strings.Repeat("x", 39) + "文" + strings.Repeat("x", 30) + "\n" +
			"ab文" + strings.Repeat("x", 60) + "\n" +
			"a\t" + strings.Repeat("x", 60) + "\n",
		stops: []navStop{
			{line: 1, start: 39, end: 42},
			{line: 2, start: 2, end: 5},
			{line: 3, start: 1, end: 2},
		},
	}})
	key := flatIndicatorModel(t, m, idx, 3)
	listW := m.listWidth()

	// At offset 0 the right-edge clip of line 1's 文 already renders
	// its one in-window cell as an unstyled blank — then the current
	// line's right star (the match is entirely hidden right).
	row1 := frameRow(t, m, 1)[listW+1:]
	if !strings.Contains(row1, strings.Repeat("x", 39)+" ") ||
		!strings.Contains(row1, "\x1b[30;47m*\x1b[37;40;24m") {
		t.Fatalf("right-edge clip row = %q, want an unstyled blank cell then the star", row1)
	}

	// At offset 3 both clipped clusters contribute in-window clip
	// blanks — none may carry the match style.
	panTo(t, m, key, 3)
	v := viewText(m)
	for _, styledBlank := range []string{"\x1b[30;47m ", "\x1b[30;47;4m "} {
		if strings.Contains(v, styledBlank) {
			t.Fatalf("view = %q paints a clip blank as a match cell (%q)", v, styledBlank)
		}
	}
	// Line 2's clip blank sits unstyled inside the x run: the match
	// is entirely hidden left, so the gutter stars but no cell is
	// inverse.
	if row := frameRow(t, m, 2)[listW+1:]; !strings.Contains(row, " "+strings.Repeat("x", 39)) {
		t.Fatalf("left-edge clip row = %q, want an unstyled clip blank then the x run", row)
	}
	// Line 3's tab expansion contributes five clip blanks, all
	// unstyled — a split cluster's clipped portion never styles.
	if row := frameRow(t, m, 3)[listW+1:]; !strings.Contains(row, "     "+strings.Repeat("x", 35)) {
		t.Fatalf("tab-expansion clip row = %q, want five unstyled clip blanks then the x run", row)
	}
}

// A highlight at a wrap boundary does not paint the blank filler
// cell: the cluster that cannot fit the row's remaining cells moves
// to the next row, where its whole glyph is highlighted, while the
// blank the previous row leaves is filler — never a match cell.
func TestWrapBoundaryBlankIsNotAMatchCell(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	// 40 x's, then 文 (two cells, cannot fit the one remaining cell
	// of the 41-wide row and wraps), then the tail.
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("x", 40) + "文tail\n",
		stops:   []navStop{{line: 1, start: 40, end: 43}},
	}})
	flatIndicatorModel(t, m, idx, 3)
	// flatIndicatorModel leaves run-off-edge installed; wrap is the
	// startup mode, so toggle back to reach the wrap-boundary case.
	_, wc := m.Update(keyW)
	deliverLayout(t, m, wc)
	listW := m.listWidth()

	// Row 1 ends at the wrap boundary: the x run plus one filler
	// blank — no inverse style at all on the row.
	row1 := frameRow(t, m, 1)[listW+1:]
	if !strings.Contains(row1, strings.Repeat("x", 40)+" ") {
		t.Fatalf("wrap row 1 = %q, want the x run then an unstyled filler blank", row1)
	}
	if strings.Contains(row1, "\x1b[30;47") {
		t.Fatalf("wrap row 1 = %q paints the boundary filler as a match cell", row1)
	}
	// Row 2 is the continuation row: the wrapped cluster's whole
	// glyph is highlighted there — the match style is on the glyph,
	// not on the previous row's blank.
	if row := frameRow(t, m, 2); !strings.Contains(row, "\x1b[30;47;4m文") {
		t.Fatalf("wrap row 2 = %q, want the wrapped 文 highlighted whole", row)
	}
}

// A match on the combining-mark bytes of a decomposed é highlights
// the whole glyph in the view — the cluster-expanded span is what the
// renderer consumes.
func TestCombiningOnlyMatchPaintsWholeGlyph(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: "cafe\xcc\x81x\n",
		stops:   []navStop{{line: 1, start: 4, end: 6}},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	// The cluster's display bytes are decomposed "e\u0301": the
	// styled run holds the whole cluster's form.
	if v := viewText(m); !strings.Contains(v, "\x1b[30;47;4me\u0301") {
		t.Fatalf("view = %q, want the whole é glyph highlighted for a combining-only match", v)
	}
}

// A standalone combining mark displays as the ◌ fallback cell, and a
// match on it highlights that one visible cell — the highlight is
// never zero cells.
func TestStandaloneCombiningFallbackCellHighlighted(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: "\xcc\x81x\n",
		stops:   []navStop{{line: 1, start: 0, end: 2}},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	if v := viewText(m); !strings.Contains(v, "\x1b[30;47;4m◌́") {
		t.Fatalf("view = %q, want the ◌ fallback cell highlighted", v)
	}
}

// A CJK match paints both of the glyph's cells together — the
// expanded span covers the whole cluster, never a split.
func TestCJKMatchPaintsBothCells(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: "ab文cd\n",
		stops:   []navStop{{line: 1, start: 2, end: 5}},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	if v := viewText(m); !strings.Contains(v, "\x1b[30;47;4m文\x1b[37;40;24m") {
		t.Fatalf("view = %q, want the whole two-cell glyph highlighted", v)
	}
}
