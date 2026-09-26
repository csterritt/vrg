package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// twoFileModel builds the standard navigation fixture: a.txt with
// aLines lines and matches on lines 1 and 3, b.txt with bLines lines
// and a match on line 2 — records emitted b.txt-first so path ordering
// is proven rather than arrival order. a.txt's load completes before
// return, so the model is in browse on stop a.txt:1 with b.txt
// uncached.
func twoFileModel(t *testing.T, dir string, w, h, aLines, bLines int) Model {
	t.Helper()
	var a, b strings.Builder
	for i := 1; i <= aLines; i++ {
		if i == 1 || i == 3 {
			fmt.Fprintf(&a, "hit a%03d\n", i)
		} else {
			fmt.Fprintf(&a, "a%03d\n", i)
		}
	}
	for i := 1; i <= bLines; i++ {
		if i == 2 {
			fmt.Fprintf(&b, "hit b%03d\n", i)
		} else {
			fmt.Fprintf(&b, "b%03d\n", i)
		}
	}
	writeWorkFile(t, dir, "a.txt", a.String())
	writeWorkFile(t, dir, "b.txt", b.String())
	m, cmd := browseModel(t, dir, w, h,
		`{"type":"begin","data":{"path":{"text":"b.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"b.txt"},"lines":{"text":"hit b002\n"},"line_number":2,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"b.txt"},"binary_offset":null}}`,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit a001\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit a003\n"},"line_number":3,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m, _ = update(t, m, cmd())
	return m
}

// Startup selects the first stop in path-then-line order: although
// b.txt's records arrived first, the cursor sits on a.txt:1 — the
// panel identifies a.txt, the list underline is on its entry, the load
// request is for a.txt, and its line-1 match renders inverse +
// underlined while line 3's stays plain inverse.
func TestStartupSelectsFirstStopInPathOrder(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "hit a1\nplain\nhit a3\n")
	writeWorkFile(t, dir, "b.txt", "x\nhit b2\n")
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"b.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"b.txt"},"lines":{"text":"hit b2\n"},"line_number":2,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"b.txt"},"binary_offset":null}}`,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit a1\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit a3\n"},"line_number":3,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	v := m.View().Content
	if !strings.Contains(v, "\x1b[4ma.txt") {
		t.Fatalf("list underline not on a.txt: %q", v)
	}
	if !strings.Contains(v, "── a.txt ") {
		t.Fatalf("filename rule not showing a.txt: %q", v)
	}
	if cmd == nil {
		t.Fatal("search completion returned no load command")
	}
	msg := cmd()
	ld, ok := msg.(loadDoneMsg)
	if !ok || string(ld.path) != "a.txt" {
		t.Fatalf("startup load = %T %v, want loadDoneMsg for a.txt", msg, msg)
	}
	m, _ = update(t, m, msg)
	v = m.View().Content
	if !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m a1") {
		t.Fatalf("first stop's match not inverse+underlined: %q", v)
	}
	if !strings.Contains(v, "\x1b[30;47mhit\x1b[37;40m a3") {
		t.Fatalf("other stop's match not plain inverse: %q", v)
	}
}

// n advances the cursor within a file: the underline moves from the
// line-1 match to the line-3 match while the viewport stays put —
// the line-3 target is already on screen, so Issue #14's reveal is a
// no-scroll — and no load is requested.
func TestNextWithinFileMovesCurrentLine(t *testing.T) {
	m := twoFileModel(t, t.TempDir(), 80, 24, 8, 4)
	top := m.vp.Top()
	m, cmd := update(t, m, keyPress("n"))
	if cmd != nil {
		t.Fatalf("same-file n returned a command %T", cmd)
	}
	s, ok := m.currentStop()
	if !ok || s.Line != 3 || string(s.Path) != "a.txt" {
		t.Fatalf("stop after n = %+v ok=%v, want a.txt:3", s, ok)
	}
	if m.vp.Top() != top {
		t.Fatalf("same-file n scrolled: top=%d, want %d", m.vp.Top(), top)
	}
	v := m.View().Content
	if !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m a003") {
		t.Fatalf("line-3 match not underlined after n: %q", v)
	}
	if !strings.Contains(v, "\x1b[30;47mhit\x1b[37;40m a001") {
		t.Fatalf("line-1 match not plain inverse after n: %q", v)
	}
}

// n crossing into another file's stop switches the panel: the returned
// command is the uncached file's load, the filename rule and the list
// underline move immediately, the panel shows Loading… until the
// buffer arrives, the new file starts at the top of its content, and
// the departing file's viewport lands in its saved state.
func TestNextAcrossFileSwitchesPanelAndLoads(t *testing.T) {
	m := twoFileModel(t, t.TempDir(), 80, 24, 40, 30)
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, codePress(tea.KeyDown))
	}
	m, _ = update(t, m, keyPress("n")) // a.txt:1 → a.txt:3, same file
	// Line 3's row is above the scrolled window, so the reveal pulls
	// the top back to 0 — a moving reveal replaces the saved state.
	m, cmd := update(t, m, keyPress("n"))
	if cmd == nil {
		t.Fatal("crossing to uncached b.txt returned no load command")
	}
	if s, _ := m.currentStop(); string(s.Path) != "b.txt" || s.Line != 2 {
		t.Fatalf("stop after crossing n = %+v, want b.txt:2", s)
	}
	v := m.View().Content
	if !strings.Contains(v, "\x1b[4mb.txt") {
		t.Fatalf("list underline did not move to b.txt: %q", v)
	}
	if !strings.Contains(v, "── b.txt ") || !strings.Contains(v, "Loading…") {
		t.Fatalf("panel did not switch to loading b.txt: %q", v)
	}
	m, _ = update(t, m, cmd())
	v = m.View().Content
	if !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m b002") {
		t.Fatalf("b.txt's match not inverse+underlined: %q", v)
	}
	if m.vp.Top() != 0 {
		t.Fatalf("first visit to b.txt started at top %d, want 0", m.vp.Top())
	}
	if m.saved["a.txt"] != 0 {
		t.Fatalf("departing a.txt viewport saved as %d, want 0 — the reveal moved it", m.saved["a.txt"])
	}
}

// The cursor wraps circularly at both ends: n on the last stop selects
// the first and p on the first selects the last — both are file
// crossings to already-cached files, so neither requests a load.
func TestNavigationWrapsBothEnds(t *testing.T) {
	m := twoFileModel(t, t.TempDir(), 80, 24, 8, 4)
	m, _ = update(t, m, keyPress("n"))    // a.txt:1 → a.txt:3
	m, cmd := update(t, m, keyPress("n")) // a.txt:3 → b.txt:2
	if cmd == nil {
		t.Fatal("crossing to uncached b.txt returned no load command")
	}
	m, _ = update(t, m, cmd())           // b.txt loads
	m, cmd = update(t, m, keyPress("n")) // last stop → first
	if cmd != nil {
		t.Fatalf("wrap to cached a.txt returned a command %T", cmd)
	}
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 1 {
		t.Fatalf("stop after wrap-forward = %+v, want a.txt:1", s)
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[4ma.txt") ||
		!strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m a001") {
		t.Fatalf("wrap-forward view not back on a.txt:1: %q", v)
	}
	m, cmd = update(t, m, keyPress("p")) // first stop → last
	if cmd != nil {
		t.Fatalf("wrap to cached b.txt returned a command %T", cmd)
	}
	if s, _ := m.currentStop(); string(s.Path) != "b.txt" || s.Line != 2 {
		t.Fatalf("stop after wrap-back = %+v, want b.txt:2", s)
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[4mb.txt") ||
		!strings.Contains(v, "── b.txt ") {
		t.Fatalf("wrap-back view not on b.txt: %q", v)
	}
}

// A one-stop index makes n and p strict no-ops: no command — so no
// reload — and a byte-identical frame.
func TestOneStopIndexIgnoresNP(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "x\n")
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	m, _ = update(t, m, cmd())
	before := m.View().Content
	for _, k := range []string{"n", "p"} {
		var c tea.Cmd
		m, c = update(t, m, keyPress(k))
		if c != nil {
			t.Fatalf("%s on a one-stop index produced a command %T", k, c)
		}
	}
	if got := m.View().Content; got != before {
		t.Fatalf("one-stop n/p changed the frame: %q", got)
	}
}

// Manual scrolling leaves the cursor on its last selected stop: after
// scrolling deep into the file, n advances from that stop to a.txt:3 —
// not from the scrolled position — and the destination reveal then
// scrolls back to show the now-hidden target, BOF-clamped to the top.
func TestManualScrollThenNContinuesFromStop(t *testing.T) {
	m := twoFileModel(t, t.TempDir(), 80, 24, 60, 4)
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, codePress(tea.KeyDown))
	}
	if m.vp.Top() != 10 {
		t.Fatalf("top after ten downs = %d, want 10", m.vp.Top())
	}
	m, cmd := update(t, m, keyPress("n"))
	if cmd != nil {
		t.Fatalf("n after manual scroll returned a command %T", cmd)
	}
	if s, _ := m.currentStop(); s.Line != 3 || string(s.Path) != "a.txt" {
		t.Fatalf("n after manual scroll selected %+v, want a.txt:3", s)
	}
	// Line 3's row is hidden above the window; the reveal lands it
	// near the top (row 2 − 7 clamps to 0).
	if m.vp.Top() != 0 {
		t.Fatalf("n after manual scroll left top=%d, want the revealed 0", m.vp.Top())
	}
}

// A departing file's viewport is saved on the way out and restored on
// the way back as the reveal's starting point: the n to a.txt:3 reveals
// the hidden target and replaces a.txt's saved top with 0, so the
// revisit resumes at 0 and the already-visible target does not scroll.
func TestDepartingViewportSavedAndRestoredOnRevisit(t *testing.T) {
	m := twoFileModel(t, t.TempDir(), 80, 24, 60, 30)
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, codePress(tea.KeyDown))
	}
	m, _ = update(t, m, keyPress("n"))    // a.txt:1 → a.txt:3, reveal → top 0
	m, cmd := update(t, m, keyPress("n")) // a.txt:3 → b.txt:2
	if cmd == nil {
		t.Fatal("crossing to uncached b.txt returned no load command")
	}
	m, _ = update(t, m, cmd())           // b.txt loads at top 0
	m, cmd = update(t, m, keyPress("p")) // b.txt:2 → a.txt:3, cached
	if cmd != nil {
		t.Fatalf("revisit to cached a.txt returned a command %T", cmd)
	}
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 3 {
		t.Fatalf("stop after p = %+v, want a.txt:3", s)
	}
	// The earlier moving reveal replaced the saved 5 with 0; row 2 is
	// on screen from there, so the revisit's reveal does not scroll.
	if m.vp.Top() != 0 {
		t.Fatalf("revisit top = %d, want the saved 0", m.vp.Top())
	}
}

// The file list is passive: it has no direct selection route — keys
// outside the navigation set move neither the cursor nor the view.
func TestFileListHasNoDirectSelection(t *testing.T) {
	m := twoFileModel(t, t.TempDir(), 80, 24, 8, 4)
	before := m.View().Content
	for _, k := range []string{"l", "0", " ", "e", "N", "P"} {
		var c tea.Cmd
		m, c = update(t, m, keyPress(k))
		if c != nil {
			t.Fatalf("key %q produced a command %T", k, c)
		}
	}
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 1 {
		t.Fatalf("passive-list keys moved the cursor to %+v", s)
	}
	if got := m.View().Content; got != before {
		t.Fatal("passive-list keys changed the frame")
	}
}
