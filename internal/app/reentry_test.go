package app_test

import (
	"bytes"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
)

// --- Issue #26 Task 3: gated re-entry tests ---

// reentrySetup creates a browse model with two files (A and B) where
// both initially fail. A is the startup file. The gated failing
// loader holds all gates so tests can control when each load
// completes. Returns the model, the loader, and the async load
// channel for the startup file (A).
func reentrySetup(t *testing.T) (app.Model, *gatedFailingLoader, <-chan app.FileLoadCompleteMsg) {
	t.Helper()
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
	)
	loader := newGatedFailingLoader()
	loader.set("src/a.go", nil)
	loader.setFailing("src/a.go", "permission denied")
	loader.set("src/b.go", nil)
	loader.setFailing("src/b.go", "permission denied")
	m, loadA := setupBrowseGatedFailing(t, idx, loader)
	return m, loader, loadA
}

// TestReentryOverlayReopensImmediately verifies that entering a
// previously failed file from a different file reopens the prior
// failure overlay immediately (before the retry completes) and the
// panel switches from "(unreadable)" to "Loading…".
func TestReentryOverlayReopensImmediately(t *testing.T) {
	m, loader, loadA := reentrySetup(t)

	// A fails (startup file).
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open after A fails")
	}

	// Dismiss the overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("overlay should be closed after Esc")
	}

	// Navigate to B (cross-file entry). B also fails initially.
	loader.release("src/b.go")
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	lc = <-loadB
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open after B fails")
	}

	// Dismiss B's overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Now navigate back to A (re-entry from a different file). A
	// is in failedPaths. The overlay should reopen immediately and
	// the panel should show "Loading…" (not "(unreadable)").
	// Re-arm A's gate so the retry is held in flight.
	loader.rearm("src/a.go")
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	_ = startLoadAsync(cmd) // execute the load command (blocks at gate)
	loader.waitStarted("src/a.go")

	// The overlay should be open immediately (prior failure overlay
	// reopens).
	if !m.OverlayOpen() {
		t.Fatalf("overlay should reopen immediately on re-entry")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	if m.OverlayFatal() {
		t.Fatalf("re-entry overlay should be non-fatal")
	}

	// The panel should show "Loading…" (the retry is in flight),
	// not "(unreadable)".
	if m.ReadFailed() {
		t.Fatalf("panel should show 'Loading…' on re-entry, not '(unreadable)' (readFailed=true)")
	}
	if !m.IsLoading() {
		t.Fatalf("panel should show 'Loading…' on re-entry (loading=false)")
	}

	// Exactly one retry should start immediately (while the overlay
	// is open). It must not wait for dismissal.
	if got := loader.callCount("src/a.go"); got != 2 {
		t.Fatalf("loader called %d times for A after re-entry, want 2 (exactly one retry)", got)
	}

	// Clean up: release the gate and drain the load.
	loader.release("src/a.go")
	lc = <-loadA2(t, cmd)
	m = deliverCompletion(t, m, lc)
	_ = m
}

// loadA2 drains the second load for A from the command.
func loadA2(t *testing.T, cmd tea.Cmd) <-chan app.FileLoadCompleteMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	ch := make(chan app.FileLoadCompleteMsg, 1)
	go func() {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			ch <- lc
			return
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				if c == nil {
					continue
				}
				sub := c()
				if sub == nil {
					continue
				}
				if lc, ok := sub.(app.FileLoadCompleteMsg); ok {
					ch <- lc
					return
				}
			}
		}
	}()
	return ch
}

// TestReentrySuccessfulSettlement verifies that when the retry
// succeeds, the placeholder changes to content without waiting for
// dismissal, the overlay remains open and dismissible, and no new
// diagnostic is collected.
func TestReentrySuccessfulSettlement(t *testing.T) {
	m, loader, loadA := reentrySetup(t)

	// A fails (startup file).
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	// Dismiss the overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Navigate to B, let B fail, dismiss.
	loader.release("src/b.go")
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	lc = <-loadB
	m = deliverCompletion(t, m, lc)
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	diagCount := len(m.Diagnostics()) // after A's first failure + B's failure

	// Re-arm A with a successful buffer and re-enter A.
	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.clearFailing("src/a.go")
	loader.rearm("src/a.go")
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	loadA2Ch := startLoadAsync(cmd) // execute the load command (blocks at gate)
	loader.waitStarted("src/a.go")
	if !m.OverlayOpen() {
		t.Fatalf("overlay should reopen on re-entry")
	}

	// Release A's gate → retry succeeds.
	loader.release("src/a.go")
	lc = <-loadA2Ch
	m = deliverCompletion(t, m, lc)

	// The placeholder should change to content without waiting for
	// dismissal. Check the model state directly (the overlay covers
	// the panel in the rendered view).
	if m.ReadFailed() {
		t.Fatalf("panel should show content after successful retry, not '(unreadable)' (readFailed=true)")
	}
	if m.IsLoading() {
		t.Fatalf("panel should show content after successful retry, not 'Loading…' (loading=true)")
	}

	// The overlay should remain open and dismissible.
	if !m.OverlayOpen() {
		t.Fatalf("overlay should remain open after successful retry")
	}
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("overlay should be dismissible after successful retry")
	}

	// After dismissal, the panel should show A's content.
	view := viewContent(m)
	if strings.Contains(view, "(unreadable)") {
		t.Fatalf("panel should show content after successful retry and dismiss, not '(unreadable)':\n%s", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("panel should show content after successful retry and dismiss, not 'Loading':\n%s", view)
	}
	if !strings.Contains(view, "content-a") {
		t.Fatalf("panel should show A's content after successful retry and dismiss:\n%s", view)
	}

	// Successful retry should collect no new diagnostic.
	if got := len(m.Diagnostics()); got != diagCount {
		t.Fatalf("diagnostics after successful retry = %d, want %d (no new diagnostic)", got, diagCount)
	}
}

// TestReentrySecondFailureAppend verifies that when the retry fails
// again, exactly one new diagnostic occurrence is appended to the
// open overlay, the overlay scroll position is preserved, and
// exactly one new replay occurrence is collected.
func TestReentrySecondFailureAppend(t *testing.T) {
	m, loader, loadA := reentrySetup(t)

	// A fails (startup file).
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)
	origOverlayText := m.OverlayText()

	// Dismiss the overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Navigate to B, let B fail, dismiss.
	loader.release("src/b.go")
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	lc = <-loadB
	m = deliverCompletion(t, m, lc)
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	diagCount := len(m.Diagnostics()) // after A's first failure + B's failure

	// Re-enter A (still failing).
	loader.rearm("src/a.go")
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	loadA2Ch := startLoadAsync(cmd) // execute the load command (blocks at gate)
	loader.waitStarted("src/a.go")
	if !m.OverlayOpen() {
		t.Fatalf("overlay should reopen on re-entry")
	}

	// Scroll the overlay down a bit.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyDown))
	m, _ = update(t, m, keyPressOrEscape(tea.KeyDown))
	scrollAfterScroll := m.OverlayScroll()

	// Release A's gate → retry fails again.
	loader.release("src/a.go")
	lc = <-loadA2Ch
	m = deliverCompletion(t, m, lc)

	// The overlay should still be open.
	if !m.OverlayOpen() {
		t.Fatalf("overlay should remain open after second failure")
	}

	// Exactly one new diagnostic occurrence should be appended to
	// the overlay text.
	overlayText := m.OverlayText()
	if overlayText == origOverlayText {
		t.Fatalf("overlay text should have appended diagnostic after second failure")
	}
	if !strings.Contains(overlayText, "permission denied") {
		t.Fatalf("overlay text should contain 'permission denied':\n%s", overlayText)
	}
	// Count occurrences of the diagnostic.
	occurrences := strings.Count(overlayText, "permission denied")
	if occurrences != 2 {
		t.Fatalf("overlay text should have 2 diagnostic occurrences, got %d:\n%s", occurrences, overlayText)
	}

	// The overlay scroll position should be preserved.
	if m.OverlayScroll() != scrollAfterScroll {
		t.Fatalf("overlay scroll = %d after second failure, want %d (preserved)", m.OverlayScroll(), scrollAfterScroll)
	}

	// Exactly one new replay occurrence should be collected.
	if got := len(m.Diagnostics()); got != diagCount+1 {
		t.Fatalf("diagnostics after second failure = %d, want %d (exactly one new)", got, diagCount+1)
	}
}

// TestReentryEscDoesNotDisturbInFlightLoad verifies that Esc dismisses
// the overlay without disturbing the in-flight retry load. The load
// completes and updates the file's state/cache.
func TestReentryEscDoesNotDisturbInFlightLoad(t *testing.T) {
	m, loader, loadA := reentrySetup(t)

	// A fails (startup file).
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	// Dismiss the overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Navigate to B, let B fail, dismiss.
	loader.release("src/b.go")
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	lc = <-loadB
	m = deliverCompletion(t, m, lc)
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Re-enter A with a successful buffer, but hold the gate.
	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.clearFailing("src/a.go")
	loader.rearm("src/a.go")
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	loadA2Ch := startLoadAsync(cmd) // execute the load command (blocks at gate)
	loader.waitStarted("src/a.go")
	if !m.OverlayOpen() {
		t.Fatalf("overlay should reopen on re-entry")
	}

	// Esc dismisses the overlay while the retry is in flight.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("overlay should be closed after Esc")
	}

	// The in-flight load should still be running. Release the gate
	// → the load completes successfully.
	loader.release("src/a.go")
	lc = <-loadA2Ch
	m = deliverCompletion(t, m, lc)

	// The file's state should be updated (content shown, not
	// "(unreadable)").
	view := viewContent(m)
	if strings.Contains(view, "(unreadable)") {
		t.Fatalf("panel should show content after in-flight load completes, not '(unreadable)':\n%s", view)
	}
	if !strings.Contains(view, "content-a") {
		t.Fatalf("panel should show A's content after in-flight load completes:\n%s", view)
	}
}

// TestReentryNavigateAwayDuringRetry verifies that navigating away
// during a retry causes the completion to update only that file's
// state/cache. Later re-entry follows the same sequence against the
// new prior state.
func TestReentryNavigateAwayDuringRetry(t *testing.T) {
	m, loader, loadA := reentrySetup(t)

	// A fails (startup file).
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	// Dismiss the overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Navigate to B, let B fail, dismiss.
	loader.release("src/b.go")
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	lc = <-loadB
	m = deliverCompletion(t, m, lc)
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Re-enter A with a successful buffer, but hold the gate.
	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.clearFailing("src/a.go")
	loader.rearm("src/a.go")
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	loadA2Ch := startLoadAsync(cmd) // execute the load command (blocks at gate)
	loader.waitStarted("src/a.go")
	if !m.OverlayOpen() {
		t.Fatalf("overlay should reopen on re-entry")
	}

	// Navigate away to B during the retry (A's gate is still held).
	// Dismiss the overlay first (Esc), then navigate. Set B to
	// succeed so B's re-entry doesn't open an overlay.
	loader.setBuffer("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	loader.clearFailing("src/b.go")
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	m, cmdB := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	// Drain B's load (succeeds immediately since B's gate was
	// already released).
	loadB2 := startLoadAsync(cmdB)
	lc = <-loadB2
	m = deliverCompletion(t, m, lc)
	// B's re-entry overlay remains open after successful retry
	// (expected). Dismiss it before continuing.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Release A's gate → A's retry completes. The completion should
	// update only A's state/cache, not the visible panel (B is
	// current).
	loader.release("src/a.go")
	lc = <-loadA2Ch
	m = deliverCompletion(t, m, lc)

	// The visible panel should still show B (not A's content).
	if !bytes.Equal(m.CurrentPath(), []byte("src/b.go")) {
		t.Fatalf("current path should still be B after A's retry completes, got %q", m.CurrentPath())
	}

	// A should now be cached (successful load). Re-enter A: no
	// overlay should appear (A is no longer in failedPaths).
	m, navCmd := update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	// Deliver the layout command to build the viewport for the
	// cached buffer.
	m = deliverLayout(t, m, navCmd)
	if m.OverlayOpen() {
		t.Fatalf("overlay should not open on re-entry after successful retry")
	}
	view := viewContent(m)
	if strings.Contains(view, "(unreadable)") {
		t.Fatalf("panel should show A's content after successful retry:\n%s", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("panel should show A's content after successful retry, not 'Loading':\n%s", view)
	}
	if !strings.Contains(view, "content-a") {
		t.Fatalf("panel should show A's content after successful retry:\n%s", view)
	}
}

// TestReentryOneRetryWhileInFlight verifies that if a retry is already
// in flight, a second re-entry request is dropped under Issue #25's
// one-load-per-path rule. The loader call count should not increase.
func TestReentryOneRetryWhileInFlight(t *testing.T) {
	m, loader, loadA := reentrySetup(t)

	// A fails (startup file).
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	// Dismiss the overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Navigate to B, let B fail, dismiss.
	loader.release("src/b.go")
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	lc = <-loadB
	m = deliverCompletion(t, m, lc)
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// Re-enter A (still failing). Hold the gate so the retry is
	// in flight.
	loader.rearm("src/a.go")
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	loadA2Ch := startLoadAsync(cmd) // execute the load command (blocks at gate)
	loader.waitStarted("src/a.go")
	callsAfterReentry := loader.callCount("src/a.go")

	// Navigate to B and back to A while A's retry is in flight.
	// The second re-entry should not start a second load (one load
	// per path). Dismiss the overlay first (Esc), then navigate.
	// Set B to succeed so B's re-entry doesn't open an overlay.
	loader.setBuffer("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	loader.clearFailing("src/b.go")
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	m, cmdB := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB2 := startLoadAsync(cmdB)
	lc = <-loadB2
	m = deliverCompletion(t, m, lc)
	// B's re-entry overlay remains open after successful retry
	// (expected). Dismiss it before continuing.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	m, _ = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")

	if got := loader.callCount("src/a.go"); got != callsAfterReentry {
		t.Fatalf("loader called %d times for A after second re-entry while in flight, want %d (dropped)", got, callsAfterReentry)
	}

	// Clean up.
	loader.release("src/a.go")
	lc = <-loadA2Ch
	m = deliverCompletion(t, m, lc)
	_ = m
}
