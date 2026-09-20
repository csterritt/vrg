package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// The startup file is a first visit: once its load completes the first
// stop's target row — the rendered row holding the first submatch's
// start cell — is revealed at zero-based row floor(content height / 3),
// not merely left wherever the top of the file put it.
func TestStartupRevealsTargetAfterLoad(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-50\n", 50, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24) // content height 23
	m = applyLoad(t, m, cmd())

	// floor(23/3) = 7, so target row 49 lands at content row 7 and the
	// window's top is rendered row 42.
	if row := contentRow(t, m); !strings.Contains(row, "line-43") {
		t.Fatalf("startup reveal: first content row = %q, want line-43 (top row 42)", row)
	}
	v := m.View().Content
	row := rowWith(t, v, "-50")
	if !strings.Contains(row, "\x1b[4m") {
		t.Fatalf("startup reveal: target line-50 is not the underlined current match:\n%s", v)
	}
}

// Every actual n/p transition reveals the target, including inside one
// file: n to a hidden row far below lands it one third down, a second n
// near EOF clamps the top to the last valid position, and p back to a
// near-BOF row clamps to the top of the file — content takes precedence
// over one-third placement at both ends.
func TestSameFileNavigationRevealsTarget(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-60\n", 60, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-99\n", 99, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24) // content height 23
	m = applyLoad(t, m, cmd())
	// Startup: target row 4 is already visible at top 0 — no scroll.
	if row := contentRow(t, m); !strings.Contains(row, "line-01") {
		t.Fatalf("startup: first content row = %q, want line-01", row)
	}

	m, cmd = update(t, m, keyMsg("n")) // to line-60: row 59 → top 59-7 = 52
	if cmd != nil {
		t.Fatalf("same-file n returned a command: %v", cmd)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-53") {
		t.Fatalf("n to hidden target: first content row = %q, want line-53 (top row 52)", row)
	}

	m, _ = update(t, m, keyMsg("n")) // to line-99: 98-7 = 91, clamped to 100-23 = 77
	if row := contentRow(t, m); !strings.Contains(row, "line-78") {
		t.Fatalf("n to near-EOF target: first content row = %q, want line-78 (top row 77)", row)
	}
	if v := m.View().Content; !strings.Contains(v, "-99") {
		t.Fatalf("near-EOF reveal lost the target:\n%s", v)
	}

	m, _ = update(t, m, keyMsg("p")) // back to line-60: row 59 → top 52
	if row := contentRow(t, m); !strings.Contains(row, "line-53") {
		t.Fatalf("p to hidden target: first content row = %q, want line-53 (top row 52)", row)
	}
	m, _ = update(t, m, keyMsg("p")) // back to line-05: 4-7 < 0 → clamped to 0
	if row := contentRow(t, m); !strings.Contains(row, "line-01") {
		t.Fatalf("p to near-BOF target: first content row = %q, want line-01", row)
	}
}

// The target is the first submatch's start cell, not just the line: a
// match starting mid-line still reveals that line's rendered row.
func TestRevealTargetsFirstSubmatchStart(t *testing.T) {
	dir := t.TempDir()
	var content strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&content, "plain-%03d text\n", i)
	}
	writeMatchFile(t, dir, "a.txt", content.String())
	idx := searchindex.New(dir)
	// Line 50's first submatch starts at byte 10, mid-line.
	addRec(t, idx, matchRec("a.txt", "plain-001 text\n", 1, 0, 4, "plai"))
	addRec(t, idx, matchRec("a.txt", "plain-050 text\n", 50, 10, 14, "text"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24) // content height 23
	m = applyLoad(t, m, cmd())
	m, _ = update(t, m, keyMsg("n")) // to line 50's row 49 → top 49-7 = 42
	if row := contentRow(t, m); !strings.Contains(row, "plain-043") {
		t.Fatalf("n to mid-line target: first content row = %q, want plain-043 (top row 42)", row)
	}
	if v := m.View().Content; !strings.Contains(rowWith(t, v, "plain-050"), "\x1b[4m") {
		t.Fatalf("mid-line target is not the underlined current match:\n%s", v)
	}
}

// A destination already on screen does not scroll: the viewport —
// including a manually scrolled one — is the reveal's starting point
// and stays put when the target row is inside it.
func TestNavigationToVisibleTargetDoesNotScroll(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-10\n", 10, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-15\n", 15, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-06") {
		t.Fatalf("scrolled to %q, want line-06", row)
	}

	// Target row 14 is inside the visible window [5, 28): no scroll.
	m, cmd = update(t, m, keyMsg("n"))
	if cmd != nil {
		t.Fatalf("same-file n returned a command: %v", cmd)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-06") {
		t.Fatalf("n to a visible target scrolled: first content row = %q, want line-06", row)
	}
	v := m.View().Content
	if !strings.Contains(rowWith(t, v, "-15"), "\x1b[4m") {
		t.Fatalf("n did not move the current-match underline to line-15:\n%s", v)
	}
}

// On a file change the saved per-file viewport is the reveal's starting
// point: a target already visible there leaves it, a hidden target
// overrides it, and a moved reveal becomes the file's new saved state.
// A first visit — including the startup file — starts at the top.
func TestRevisitStartsFromSavedViewportThenReveals(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(50))
	var b strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&b, "row-%02d\n", i)
	}
	writeMatchFile(t, dir, "b.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-06\n", 6, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-40\n", 40, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "row-01\n", 1, 0, 3, "row"))
	idx.Finish()
	// Stop order: a.txt:6, a.txt:40, b.txt:1. Content height 23, third = 7.

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd()) // a.txt loaded; row 5 visible at top 0
	// The no-style theme keeps the fixture's matched text contiguous.
	m.theme = theme.Plain()

	// Scroll a.txt to top 20; target row 39 is inside [20, 43).
	for i := 0; i < 20; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m, _ = update(t, m, keyMsg("n")) // → a.txt:40, visible → stays
	if row := contentRow(t, m); !strings.Contains(row, "line-21") {
		t.Fatalf("n to visible target scrolled: %q, want line-21", row)
	}

	// n to b.txt: a first visit starts at the top, and its target row 0
	// is visible there — no scroll.
	m, cmd = update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, cmd))
	if row := contentRow(t, m); !strings.Contains(row, "row-01") {
		t.Fatalf("first visit to b.txt: %q, want row-01 at the top", row)
	}

	// p back to a.txt:40: the saved top 20 still shows row 39, so the
	// revisit resumes there — the no-scroll reveal leaves it.
	m, _ = update(t, m, keyMsg("p"))
	if row := contentRow(t, m); !strings.Contains(row, "line-21") {
		t.Fatalf("revisit to a.txt: %q, want the saved top line-21", row)
	}

	// p to a.txt:6: row 5 is hidden above the saved window → reveal
	// clamps to the top of the file, replacing the saved state.
	m, _ = update(t, m, keyMsg("p"))
	if row := contentRow(t, m); !strings.Contains(row, "line-01") {
		t.Fatalf("p to hidden BOF target: %q, want line-01", row)
	}

	// n to a.txt:40: row 39 hidden below [0, 23) → 39-7 = 32, clamped
	// to 50-23 = 27 — the moved reveal is the new saved top.
	m, _ = update(t, m, keyMsg("n"))
	if row := contentRow(t, m); !strings.Contains(row, "line-28") {
		t.Fatalf("n to hidden target: %q, want line-28 (top row 27)", row)
	}
	m, _ = update(t, m, keyMsg("n")) // → b.txt:1, cached
	m, _ = update(t, m, keyMsg("p")) // → a.txt:40, saved top 27 shows row 39
	if row := contentRow(t, m); !strings.Contains(row, "line-28") {
		t.Fatalf("second revisit: %q, want the revealed top line-28", row)
	}
}
