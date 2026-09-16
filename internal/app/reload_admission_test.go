package app_test

import (
	"strings"
	"testing"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
)

// --- Issue #42: atomic reload-admission tests ---
//
// handleReload must treat the one-load-per-path admission check and
// the reload-state mutation as a single decision point. A dropped r
// leaves revision, intent, and presentation exactly as they were so
// the in-flight startup or navigation load completes under its
// original classification — never misclassified as a reload with an
// extra revision bump or anchor preservation. An accepted r applies
// the full reload contract: flags, "Loading…" presentation,
// IntentReloadAnchor, and exactly one revision increment.

// TestDroppedReloadDuringStartupLoadPreservesReveal verifies that r
// pressed while the startup load is in flight is dropped without
// touching revision, intent, or presentation, so the load's
// completion performs the required first-match reveal (story 50)
// rather than reload-style anchor preservation.
func TestDroppedReloadDuringStartupLoadPreservesReveal(t *testing.T) {
	// Match at line 40 of a 50-line file: the startup reveal places
	// row 39 at floor(23/3) = 7, offset 32, clamped to maxOffset
	// 50 - 23 = 27. Anchor preservation would leave offset 0.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 40, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf(makeScrollLines(50), 50, 3))
	m, loadA := setupBrowseGated(t, idx, loader)

	// The startup load is in flight with the reveal intent pending.
	loader.waitStarted("src/a.go")
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("before r, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}
	callsBefore := loader.callCount("src/a.go")

	// Press r while the startup load is in flight. The request is
	// dropped, so the pending load's classification must not change.
	m, _ = update(t, m, keyPress('r'))
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after dropped r, LoadIntent = %v, want IntentReveal (unchanged)", m.LoadIntent())
	}
	if !m.IsLoading() {
		t.Fatal("after dropped r, IsLoading = false, want true (Loading… placeholder unchanged)")
	}
	if got := loader.callCount("src/a.go"); got != callsBefore {
		t.Fatalf("loader called %d times after dropped r, want %d (dropped, not queued)", got, callsBefore)
	}

	// Release the gate: the in-flight load completes under its
	// original classification — a first-load reveal, not a reload.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	// Revision must still be 1: the dropped r recorded no reload.
	if got := m.LayoutKey().Revision; got != 1 {
		t.Fatalf("after startup completion, LayoutKey().Revision = %d, want 1 (no reload bump)", got)
	}
	// The completion performs the first-match reveal (story 50):
	// line 40's target row lands at offset 27, not the top-of-file
	// anchor a misclassified reload would preserve.
	if got := m.ViewportOffset(); got != 27 {
		t.Fatalf("after startup completion, ViewportOffset = %d, want 27 (first-match reveal, not anchor preservation)", got)
	}
}

// TestDroppedReloadDuringNavigationLoadPreservesReveal verifies that
// r pressed while a navigation load is in flight is dropped without
// replacing the pending destination reveal: the completion reveals
// the latest selected target under the latest-target rules and bumps
// no revision.
func TestDroppedReloadDuringNavigationLoadPreservesReveal(t *testing.T) {
	// a.go has one stop; b.go has two (lines 10 and 40 of 50).
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 10, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 40, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf(makeScrollLines(50), 50, 3))
	loader.set("src/b.go", makeBuf(makeScrollLines(50), 50, 3))
	m, loadA := setupBrowseGated(t, idx, loader)
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)
	assertCurrentPath(t, m, "src/a.go")

	// Navigate to b.go — its load starts and the reveal intent is
	// pending for the b.go line-10 stop.
	m, navCmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(navCmd)
	loader.waitStarted("src/b.go")
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after n to b.go, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}
	callsBefore := loader.callCount("src/b.go")

	// Press r while b.go's navigation load is in flight — dropped.
	m, _ = update(t, m, keyPress('r'))
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after dropped r, LoadIntent = %v, want IntentReveal (unchanged)", m.LoadIntent())
	}
	if !m.IsLoading() {
		t.Fatal("after dropped r, IsLoading = false, want true (Loading… placeholder unchanged)")
	}
	if got := loader.callCount("src/b.go"); got != callsBefore {
		t.Fatalf("loader called %d times for b.go after dropped r, want %d (dropped)", got, callsBefore)
	}

	// Navigate to b.go's second stop while the load is still in
	// flight: the latest-target rule must reveal line 40, not the
	// line-10 stop that was current when the load was requested.
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() != 2 {
		t.Fatalf("after second n, CursorPosition = %d, want 2", m.CursorPosition())
	}

	loader.release("src/b.go")
	lc = <-loadB
	m = deliverCompletion(t, m, lc)

	// Revision must still be 1: the dropped r recorded no reload.
	if got := m.LayoutKey().Revision; got != 1 {
		t.Fatalf("after navigation completion, LayoutKey().Revision = %d, want 1 (no reload bump)", got)
	}
	// Latest-target reveal: line 40's row lands at offset 27.
	if got := m.ViewportOffset(); got != 27 {
		t.Fatalf("after navigation completion, ViewportOffset = %d, want 27 (latest-target reveal of line 40)", got)
	}
}

// TestAcceptedReloadBumpsRevisionExactlyOnce verifies that an r
// accepted after the previous load finished applies the reload
// contract in full: a new load is issued, "Loading…" presentation and
// IntentReloadAnchor are set, and the content revision increments by
// exactly one.
func TestAcceptedReloadBumpsRevisionExactlyOnce(t *testing.T) {
	m, loader := reloadSetup(t, makeScrollLines(50), 50, 3)

	// The initial load's revision is 1.
	if got := m.LayoutKey().Revision; got != 1 {
		t.Fatalf("after initial load, LayoutKey().Revision = %d, want 1", got)
	}
	callsBefore := loader.callCount("src/a.go")

	// Press r after the previous load finished — accepted.
	m, cmd := update(t, m, keyPress('r'))
	if !m.IsLoading() {
		t.Fatal("after accepted r, IsLoading = false, want true (Loading… presentation)")
	}
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("after accepted r, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}
	lc := execReloadCmd(t, cmd)
	m = deliverCompletion(t, m, lc)

	// Exactly one new load was issued.
	if got := loader.callCount("src/a.go"); got != callsBefore+1 {
		t.Fatalf("loader called %d times after accepted r, want %d (exactly one reread)", got, callsBefore+1)
	}
	// Exactly one revision increment: 1 → 2.
	if got := m.LayoutKey().Revision; got != 2 {
		t.Fatalf("after accepted reload, LayoutKey().Revision = %d, want 2 (exactly one increment)", got)
	}
}

// TestRapidReloadPressesKeepOneReloadInFlight verifies that rapid
// repeated r presses admit at most one reload per path: the extra
// presses are dropped without mutating state, the single accepted
// reload bumps the revision exactly once, and the placeholder→content
// transition remains the visible completion signal.
func TestRapidReloadPressesKeepOneReloadInFlight(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedFailingLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "old-content")}, 1, 3))
	m, loadA := setupBrowseGatedFailing(t, idx, loader)
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	// Re-arm the gate and press r three times in quick succession;
	// only the first press admits a load.
	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "new-content")}, 1, 3))
	loader.rearm("src/a.go")
	m, rCmd := update(t, m, keyPress('r'))
	_ = startLoadAsync(rCmd)
	loader.waitStarted("src/a.go")
	callsAfterFirst := loader.callCount("src/a.go")

	for i := 0; i < 2; i++ {
		m, _ = update(t, m, keyPress('r'))
		if !m.IsLoading() {
			t.Fatal("during in-flight reload, IsLoading = false, want true (Loading… placeholder held)")
		}
	}
	if got := loader.callCount("src/a.go"); got != callsAfterFirst {
		t.Fatalf("loader called %d times after rapid r presses, want %d (at most one reload in flight)", got, callsAfterFirst)
	}

	// Release the gate: the placeholder→content transition is the
	// completion signal, and exactly one revision bump was recorded.
	loader.release("src/a.go")
	lc2 := drainLoadFromCmd(t, rCmd)
	m = deliverCompletion(t, m, lc2)

	if got := m.LayoutKey().Revision; got != 2 {
		t.Fatalf("after rapid r presses, LayoutKey().Revision = %d, want 2 (exactly one increment)", got)
	}
	view := viewContent(m)
	if !strings.Contains(view, "new-content") {
		t.Fatalf("after accepted reload completes, view should show new-content: %q", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("after accepted reload completes, view should not show Loading: %q", view)
	}
}

// TestNavigationReentryDuringInFlightLoadStillReveals verifies that
// the atomic-admission restriction applies only to handleReload:
// navigation re-entry into a path whose load is already in flight
// still updates the selection, the placeholder presentation, and the
// IntentReveal intent even though startLoad drops the duplicate load.
func TestNavigationReentryDuringInFlightLoadStillReveals(t *testing.T) {
	// a.go's single stop is at line 40 of 50; b.go's at line 1.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 40, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf(makeScrollLines(50), 50, 3))
	loader.set("src/b.go", makeBuf(makeScrollLines(50), 50, 3))
	m, loadA := setupBrowseGated(t, idx, loader)

	// A's startup load is in flight. Navigate to B (B starts
	// loading), then back to A — re-entry while A's load is still
	// in flight drops the duplicate load but must still update the
	// selection, the placeholder, and the intent.
	loader.waitStarted("src/a.go")
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	loader.waitStarted("src/b.go")

	m, _ = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	if !m.IsLoading() {
		t.Fatal("after re-entry into loading a.go, IsLoading = false, want true (Loading… placeholder)")
	}
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after re-entry into loading a.go, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}
	view := viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("after re-entry into loading a.go, view should show Loading: %q", view)
	}

	// The in-flight startup load completes under IntentReveal: the
	// first-match reveal of a.go's line-40 stop lands at offset 27.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)
	if got := m.ViewportOffset(); got != 27 {
		t.Fatalf("after re-entry completion, ViewportOffset = %d, want 27 (first-match reveal)", got)
	}

	// Clean up B's in-flight load.
	loader.release("src/b.go")
	<-loadB
}
