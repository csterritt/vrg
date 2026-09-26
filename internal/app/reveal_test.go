package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/viewport"
)

// fileWithStops writes a lines-line file whose stopped lines read
// "hitNNNNN" and the rest "xNNNNNN", and returns its rg records — one
// match record per stop, the submatch covering the leading "hit" at
// bytes 0..3.
func fileWithStops(t *testing.T, dir, name string, lines int, stops ...int) []string {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		if slices.Contains(stops, i) {
			fmt.Fprintf(&b, "hit%05d\n", i)
		} else {
			fmt.Fprintf(&b, "x%06d\n", i)
		}
	}
	writeWorkFile(t, dir, name, b.String())
	recs := []string{fmt.Sprintf(`{"type":"begin","data":{"path":{"text":"%s"}}}`, name)}
	for _, ln := range stops {
		recs = append(recs, fmt.Sprintf(
			`{"type":"match","data":{"path":{"text":"%s"},"lines":{"text":"hit%05d\n"},"line_number":%d,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
			name, ln, ln))
	}
	return append(recs, fmt.Sprintf(`{"type":"end","data":{"path":{"text":"%s"},"binary_offset":null}}`, name))
}

// Startup reveal: the first visit starts at the top of the file, and
// once the load completes the hidden target lands at row
// floor(content height/3) — at 80x24 the content height is 23, so the
// line-200 match sits on row 7 with top 192. A reveal that moves the
// viewport replaces the file's saved vertical state.
func TestStartupRevealPlacesHiddenTargetOneThirdDown(t *testing.T) {
	dir := t.TempDir()
	recs := fileWithStops(t, dir, "a.txt", 300, 200)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	if cmd == nil {
		t.Fatal("search completion returned no load command")
	}
	m = settle(t, m, cmd)
	if m.vp.Top() != 192 {
		t.Fatalf("top after startup reveal = %d, want 192 (row 199 at row 7)", m.vp.Top())
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00200") {
		t.Fatalf("revealed view lacks the line-200 match: %q", v)
	}
	if m.saved["a.txt"] != (viewport.Target{Line: 192}) {
		t.Fatalf("moving reveal left saved = %v, want (192, 0)", m.saved["a.txt"])
	}
}

// A startup target already on screen is a no-scroll reveal: the top
// stays 0 and the file's saved vertical state is left alone — no entry
// is written.
func TestStartupRevealVisibleTargetDoesNotScroll(t *testing.T) {
	dir := t.TempDir()
	recs := fileWithStops(t, dir, "a.txt", 300, 3)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	if m.vp.Top() != 0 {
		t.Fatalf("top after visible-target startup = %d, want 0", m.vp.Top())
	}
	if _, ok := m.saved["a.txt"]; ok {
		t.Fatalf("no-scroll reveal wrote saved state: %v", m.saved)
	}
}

// The reveal waits for content: while the startup load is gate-held
// the placeholder shows and the top is still 0; the reveal lands when
// the loadDoneMsg arrives.
func TestStartupRevealAppliesOnLoadCompletion(t *testing.T) {
	dir := t.TempDir()
	recs := fileWithStops(t, dir, "a.txt", 300, 200)
	recs = append(recs, `{"type":"summary","data":{}}`)
	gate := make(chan struct{})
	m := newModel(nil, nil)
	m.loadGate = gate
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchDoneMsg{index: fixtureIndex(t, dir, recs...)})
	if cmd == nil {
		t.Fatal("search completion returned no load command")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case <-done:
		t.Fatal("load completed while the gate was held")
	case <-time.After(50 * time.Millisecond):
	}
	if v := m.View().Content; !strings.Contains(v, "Loading…") || m.vp.Top() != 0 {
		t.Fatalf("gate-held view: top=%d view=%q", m.vp.Top(), v)
	}
	close(gate)
	m = pump(t, m, <-done)
	if m.vp.Top() != 192 {
		t.Fatalf("top after load completion = %d, want 192", m.vp.Top())
	}
}

// n to a hidden same-file target moves the viewport so the destination
// row lands at floor(content height/3) and replaces the saved vertical
// state; p back to a target near BOF clamps to the top instead of
// placing it a third down.
func TestNPRevealHiddenTargets(t *testing.T) {
	dir := t.TempDir()
	recs := fileWithStops(t, dir, "a.txt", 300, 5, 200)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	if m.vp.Top() != 0 {
		t.Fatalf("startup top = %d, want 0 — line 5 is on screen", m.vp.Top())
	}

	m, cmd = update(t, m, keyPress("n"))
	if cmd != nil {
		t.Fatalf("same-file n returned a command %T", cmd)
	}
	if s, _ := m.currentStop(); s.Line != 200 {
		t.Fatalf("stop after n = %+v, want a.txt:200", s)
	}
	if m.vp.Top() != 192 {
		t.Fatalf("top after n = %d, want 192", m.vp.Top())
	}
	if m.saved["a.txt"] != (viewport.Target{Line: 192}) {
		t.Fatalf("moving reveal left saved = %v, want (192, 0)", m.saved["a.txt"])
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00200") {
		t.Fatalf("line-200 match not underlined after n: %q", v)
	}

	m, _ = update(t, m, keyPress("p"))
	if s, _ := m.currentStop(); s.Line != 5 {
		t.Fatalf("stop after p = %+v, want a.txt:5", s)
	}
	// Row 4 wants top 4-7 = -3: the BOF clamp leaves the top at 0 and
	// the match near the top.
	if m.vp.Top() != 0 {
		t.Fatalf("top after p = %d, want 0 — BOF clamp beats one-third", m.vp.Top())
	}
	if m.saved["a.txt"] != (viewport.Target{}) {
		t.Fatalf("moving reveal left saved = %v, want (0, 0)", m.saved["a.txt"])
	}
}

// n between two on-screen matches does not scroll: the target row is
// already visible, the top stays, and no saved state is written.
func TestNToVisibleTargetDoesNotScroll(t *testing.T) {
	dir := t.TempDir()
	recs := fileWithStops(t, dir, "a.txt", 50, 5, 10)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	before := m.vp.Top()
	m, cmd = update(t, m, keyPress("n"))
	if cmd != nil {
		t.Fatalf("same-file n returned a command %T", cmd)
	}
	if s, _ := m.currentStop(); s.Line != 10 {
		t.Fatalf("stop after n = %+v, want a.txt:10", s)
	}
	if m.vp.Top() != before {
		t.Fatalf("visible-target n scrolled: top=%d, want %d", m.vp.Top(), before)
	}
	if _, ok := m.saved["a.txt"]; ok {
		t.Fatalf("no-scroll reveal wrote saved state: %v", m.saved)
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00010") {
		t.Fatalf("line-10 match not underlined after n: %q", v)
	}
}

// A revisit starts from the saved per-file viewport and the reveal
// leaves it alone when the destination target is already on screen.
func TestRevisitStartsFromSavedViewport(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 300, 30, 200)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 10, 2)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd) // a.txt loads; line 30 reveals to top 22
	if m.vp.Top() != 22 {
		t.Fatalf("startup reveal top = %d, want 22", m.vp.Top())
	}
	m, _ = update(t, m, keyPress("n")) // a.txt:30 → a.txt:200
	if m.vp.Top() != 192 {
		t.Fatalf("top after n to line 200 = %d, want 192", m.vp.Top())
	}
	m, load := update(t, m, keyPress("n")) // → b.txt:2
	if load == nil {
		t.Fatal("crossing to uncached b.txt returned no load command")
	}
	m = pump(t, m, load())
	if m.saved["a.txt"] != (viewport.Target{Line: 192}) {
		t.Fatalf("departing a.txt saved = %v, want (192, 0)", m.saved["a.txt"])
	}
	m, cmd = update(t, m, keyPress("p")) // → a.txt:200, cached
	if cmd != nil {
		t.Fatalf("revisit to cached a.txt returned a command %T", cmd)
	}
	// Row 199 is inside the saved window 192..214: the revisit resumes
	// the saved position and the reveal does not move it.
	if m.vp.Top() != 192 {
		t.Fatalf("revisit top = %d, want the saved 192", m.vp.Top())
	}
}

// The reveal overrides the saved viewport when it hides the
// destination: the target lands a third down and the new top replaces
// the saved vertical state.
func TestRevisitRevealOverridesHiddenSavedViewport(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 300, 30, 200)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 10, 2)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m, _ = update(t, m, keyPress("n"))     // a.txt:200
	m, load := update(t, m, keyPress("n")) // b.txt:2
	m = pump(t, m, load())
	m.saved["a.txt"] = viewport.Target{Line: 50} // a saved position that hides line 200
	m, _ = update(t, m, keyPress("p"))           // → a.txt:200
	if m.vp.Top() != 192 {
		t.Fatalf("top = %d, want 192 — reveal overrides the hidden saved viewport", m.vp.Top())
	}
	if m.saved["a.txt"] != (viewport.Target{Line: 192}) {
		t.Fatalf("moving reveal left saved = %v, want (192, 0)", m.saved["a.txt"])
	}
}

// A first visit starts at the top of the file before the reveal: the
// destination shows "Loading…" with an empty viewport, and when its
// load completes the top-of-file start plus reveal lands the line-250
// match a third down.
func TestFirstVisitStartsAtTopThenReveals(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, fileWithStops(t, dir, "a.txt", 10, 1)...)
	recs = append(recs, fileWithStops(t, dir, "b.txt", 300, 250)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd) // a.txt loads; line 1 visible, top 0
	m, load := update(t, m, keyPress("n"))
	if load == nil {
		t.Fatal("crossing to uncached b.txt returned no load command")
	}
	if v := m.View().Content; !strings.Contains(v, "Loading…") {
		t.Fatalf("uncached b.txt lacks the placeholder: %q", v)
	}
	if m.vp.Top() != 0 {
		t.Fatalf("first visit top = %d, want 0 before the reveal", m.vp.Top())
	}
	m = pump(t, m, load())
	if m.vp.Top() != 242 {
		t.Fatalf("top after first-visit reveal = %d, want 242 (row 249 at row 7)", m.vp.Top())
	}
	if v := m.View().Content; !strings.Contains(v, "\x1b[30;47;4mhit\x1b[24;37;40m00250") {
		t.Fatalf("revealed view lacks the line-250 match: %q", v)
	}
	if m.saved["b.txt"] != (viewport.Target{Line: 242}) {
		t.Fatalf("moving reveal left saved = %v, want (242, 0)", m.saved["b.txt"])
	}
}
