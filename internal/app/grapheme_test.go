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
// never zero cells. The next cluster renders in the following cell,
// unstyled and unshared: the styled run closes on the fallback and
// 'x' resumes in the base colours.
func TestStandaloneCombiningFallbackCellHighlighted(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: "\xcc\x81x\n",
		stops:   []navStop{{line: 1, start: 0, end: 2}},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	if v := viewText(m); !strings.Contains(v, "\x1b[30;47;4m◌́\x1b[37;40;24mx") {
		t.Fatalf("view = %q, want the ◌́ fallback cell highlighted with x unstyled in the next cell", v)
	}
}

// The fallback cell pans and clips like any other one-cell cluster
// (Issue #43): one step of horizontal offset hides it whole — never
// a partial cell — the next cluster's text shifts exactly one cell
// left, and the now entirely hidden match upgrades the gutter mark
// to '*'.
func TestStandaloneCombiningFallbackPansAsOneCell(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: "\xcc\x81" + strings.Repeat("x", 60) + "\n",
		stops:   []navStop{{line: 1, start: 0, end: 2}},
	}})
	key := flatIndicatorModel(t, m, idx, 3)
	listW := m.listWidth()

	// At offset 0 the fallback cell paints highlighted and the x run
	// begins in the next cell.
	row := frameRow(t, m, 1)[listW+1:]
	if !strings.Contains(row, "\x1b[30;47;4m◌́\x1b[37;40;24mx") {
		t.Fatalf("row = %q, want the ◌́ fallback highlighted with x in the next cell", row)
	}

	// One pan step hides the fallback's single cell entirely: no ◌
	// remains in the frame, the x run starts at the window's first
	// cell, and the hidden match stars the gutter.
	panTo(t, m, key, 1)
	row = frameRow(t, m, 1)[listW+1:]
	if strings.Contains(row, "◌") {
		t.Fatalf("row = %q still shows the fallback past its one cell", row)
	}
	if !strings.Contains(row, "\x1b[30;47m*\x1b[37;40;24m") {
		t.Fatalf("row = %q, want the gutter star for the hidden-left match", row)
	}
	if !strings.HasSuffix(row, strings.Repeat("x", 40)+" ") {
		t.Fatalf("row = %q, want the x run shifted to the window's first cell", row)
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

// Issue #39 — the composed view renders the shared grapheme/cell
// model end to end: a match overlapping only part of a two-cell CJK
// character highlights exactly that cluster's cells and never
// swallows the following character.
func TestCJKPartialMatchNeverSwallowsNextChar(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: "ab文cd\n",
		// The recorded submatch covers only the first two of 文's
		// three bytes — a partial-cluster overlap that expands
		// outward to the whole cluster and no further.
		stops: []navStop{{line: 1, start: 2, end: 4}},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	v := viewText(m)
	// The styled run is exactly the glyph: it closes immediately
	// after 文 and 'cd' follows unstyled — the highlight neither
	// splits the cluster nor consumes the character after it.
	if !strings.Contains(v, "\x1b[30;47;4m文\x1b[37;40;24mcd") {
		t.Fatalf("view = %q, want 文 highlighted alone with 'cd' unstyled after it", v)
	}
}

// A base-plus-combining sequence straddling the clip edge is clipped
// only as one cluster: its in-window cell renders as an unstyled
// blank — never a partial glyph, never a match cell — and the hidden
// match still earns the right-edge star (Issue #39).
func TestWideCombiningClusterClipsAsOne(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	// "文\u0301" is one two-cell cluster: a wide base plus its
	// combining mark. It spans cells 39–40 of the line, so its second
	// cell straddles the 40-cell text window's right edge.
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("x", 39) + "文́" + strings.Repeat("x", 30) + "\n",
		stops:   []navStop{{line: 1, start: 39, end: 44}},
	}})
	flatIndicatorModel(t, m, idx, 3)
	listW := m.listWidth()

	row := frameRow(t, m, 1)[listW+1:]
	if !strings.Contains(row, strings.Repeat("x", 39)+" ") {
		t.Fatalf("clip row = %q, want an unstyled blank for the straddling cluster", row)
	}
	if strings.Contains(row, "文") {
		t.Fatalf("clip row = %q paints a partial glyph — the cluster may only clip whole", row)
	}
	if !strings.Contains(row, "\x1b[30;47m*\x1b[37;40;24m") {
		t.Fatalf("clip row = %q, want the right star for the entirely hidden match", row)
	}
}

// An emoji ZWJ sequence occupies its measured cell width and is never
// split: a match overlapping part of its bytes highlights the whole
// glyph, and a clip boundary inside its cells blanks them rather
// than cutting the sequence in half (Issue #39).
func TestZWJClusterPaintsWholeAndClipsWhole(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	// The emoji is one two-cell cluster at cells 1–2 (bytes 1–18);
	// the recorded submatch covers only part of its bytes.
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: "x👨‍👩‍👧y" + strings.Repeat("x", 50) + "\n",
		stops:   []navStop{{line: 1, start: 1, end: 8}},
	}})
	key := flatIndicatorModel(t, m, idx, 3)
	listW := m.listWidth()

	row := frameRow(t, m, 1)[listW+1:]
	if !strings.Contains(row, "\x1b[30;47;4m👨‍👩‍👧\x1b[37;40;24my") {
		t.Fatalf("row = %q, want the whole ZWJ glyph highlighted with 'y' unstyled", row)
	}

	// Pan inside the cluster — off 2 starts the window at the
	// emoji's second cell: that cell is an unstyled clip blank, the
	// emoji paints nothing, and its entirely hidden match upgrades
	// the gutter mark to '*'.
	panTo(t, m, key, 2)
	row = frameRow(t, m, 1)[listW+1:]
	if strings.Contains(row, "👨") {
		t.Fatalf("row = %q paints part of the ZWJ sequence — it may only clip whole", row)
	}
	if !strings.Contains(row, "\x1b[30;47m*\x1b[37;40;24m") {
		t.Fatalf("row = %q, want the gutter star for the hidden-left match", row)
	}
}

// center pads by measured cells, not runes (Issue #39): a two-cell
// glyph counts two toward the frame width, so the pad leaves the
// text centred in cells.
func TestCenterMeasuresCells(t *testing.T) {
	// "文x" is three cells; centred in a 10-cell frame it pads
	// (10-3)/2 = 3 — rune counting would pad 4 and drift right.
	if got, want := center("文x", 10, 3), "\n   文x"; got != want {
		t.Fatalf("center = %q, want %q", got, want)
	}
}
