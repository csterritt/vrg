package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
)

// --- Issue #27: explicit reload (r) tests ---

// reloadSetup creates a browse model with a single file (src/a.go)
// loaded successfully. The failingLoader lets tests change the buffer
// between calls so the reload returns different content. Returns the
// model and the loader.
func reloadSetup(t *testing.T, lines []filebuffer.Line, lineCount, gutter int) (app.Model, *failingLoader) {
	t.Helper()
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newFailingLoader()
	loader.setBuffer("src/a.go", makeBuf(lines, lineCount, gutter))
	m, loadCh := setupBrowseFailing(t, idx, loader.load)
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)
	return m, loader
}

// TestReloadShowsLoadingAndRereads verifies that r shows "Loading…"
// and issues exactly one reread of the current file without rerunning
// rg or changing cursor stops. The loader call count should increase
// by exactly one.
func TestReloadShowsLoadingAndRereads(t *testing.T) {
	m, loader := reloadSetup(t, []filebuffer.Line{ml(1, "old-content")}, 1, 3)
	callsBefore := loader.callCount("src/a.go")
	cursorBefore := m.CursorPosition()

	// Change the buffer so the reload returns new content.
	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "new-content")}, 1, 3))

	// Press r — should show "Loading…" and start exactly one reload.
	m, cmd := update(t, m, keyPress('r'))
	if !m.IsLoading() {
		t.Fatalf("after r, IsLoading = false, want true (Loading… placeholder)")
	}
	view := viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("after r, view should show Loading: %q", view)
	}

	// The reload command should produce a FileLoadCompleteMsg.
	lc := execReloadCmd(t, cmd)
	m = deliverCompletion(t, m, lc)

	// Exactly one new load should have been issued.
	if got := loader.callCount("src/a.go"); got != callsBefore+1 {
		t.Fatalf("loader called %d times after r, want %d (exactly one reread)", got, callsBefore+1)
	}

	// Cursor stops should be unchanged.
	if m.CursorPosition() != cursorBefore {
		t.Fatalf("after r, CursorPosition = %d, want %d (unchanged)", m.CursorPosition(), cursorBefore)
	}

	// The panel should show the new content.
	view = viewContent(m)
	if !strings.Contains(view, "new-content") {
		t.Fatalf("after reload, view should show new-content: %q", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("after reload, view should not show Loading: %q", view)
	}
}

// execReloadCmd executes the command returned by pressing r. It may
// be a single FileLoadCompleteMsg or a batch. Returns the first
// FileLoadCompleteMsg found.
func execReloadCmd(t *testing.T, cmd tea.Cmd) app.FileLoadCompleteMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("r returned nil command")
	}
	msg := execCmd(t, cmd)
	if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
		return lc
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
				return lc
			}
		}
	}
	t.Fatalf("r command did not produce a FileLoadCompleteMsg, got %T", msg)
	return app.FileLoadCompleteMsg{}
}

// TestReloadDropsDuplicateWhileInFlight verifies that a duplicate r
// while the path's load is already in flight is dropped, not queued.
// The loader call count should not increase for the duplicate.
func TestReloadDropsDuplicateWhileInFlight(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
	)
	loader := newGatedFailingLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "old-content")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "b-content")}, 1, 3))
	m, loadA := setupBrowseGatedFailing(t, idx, loader)

	// Release A's gate so A loads successfully.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)
	callsBefore := loader.callCount("src/a.go")

	// Re-arm A's gate and change the buffer for the reload.
	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "new-content")}, 1, 3))
	loader.rearm("src/a.go")

	// Press r — reload starts (blocked at gate).
	m, cmd := update(t, m, keyPress('r'))
	_ = startLoadAsync(cmd)
	loader.waitStarted("src/a.go")
	callsAfterFirst := loader.callCount("src/a.go")
	if callsAfterFirst != callsBefore+1 {
		t.Fatalf("loader called %d times after first r, want %d", callsAfterFirst, callsBefore+1)
	}

	// Press r again while the load is in flight — should be dropped.
	m, cmd2 := update(t, m, keyPress('r'))
	_ = cmd2
	if got := loader.callCount("src/a.go"); got != callsAfterFirst {
		t.Fatalf("loader called %d times after duplicate r, want %d (dropped)", got, callsAfterFirst)
	}

	// Release the gate — the load completes.
	loader.release("src/a.go")
	lc2 := drainLoadFromCmd(t, cmd)
	m = deliverCompletion(t, m, lc2)

	// After completion, r should start a new load.
	callsAfterCompletion := loader.callCount("src/a.go")
	loader.rearm("src/a.go")
	m, cmd3 := update(t, m, keyPress('r'))
	_ = startLoadAsync(cmd3)
	loader.waitStarted("src/a.go")
	if got := loader.callCount("src/a.go"); got != callsAfterCompletion+1 {
		t.Fatalf("loader called %d times after post-completion r, want %d (new load)", got, callsAfterCompletion+1)
	}

	// Clean up.
	loader.release("src/a.go")
}

// drainLoadFromCmd drains a FileLoadCompleteMsg from the command
// returned by pressing r (which may block at the gated loader).
func drainLoadFromCmd(t *testing.T, cmd tea.Cmd) app.FileLoadCompleteMsg {
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
	return <-ch
}

// TestReloadDropsReentryWhileInFlight verifies that re-entry (n/p
// navigation away and back) while a reload is in flight is dropped
// under the one-load-per-path rule. The loader call count should not
// increase for the re-entry.
func TestReloadDropsReentryWhileInFlight(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
	)
	loader := newGatedFailingLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "old-content")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "b-content")}, 1, 3))
	m, loadA := setupBrowseGatedFailing(t, idx, loader)

	// Release A's gate so A loads successfully.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	// Navigate to B and let B load.
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	loader.release("src/b.go")
	lc = <-loadB
	m = deliverCompletion(t, m, lc)

	// Navigate back to A (cached).
	m, _ = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")

	// Re-arm A's gate and change the buffer for the reload.
	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "new-content")}, 1, 3))
	loader.rearm("src/a.go")

	// Press r — reload starts (blocked at gate).
	m, rCmd := update(t, m, keyPress('r'))
	_ = startLoadAsync(rCmd)
	loader.waitStarted("src/a.go")
	callsAfterR := loader.callCount("src/a.go")

	// Navigate to B and back to A while A's reload is in flight.
	// The re-entry should be dropped (one-load-per-path).
	m, _ = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	m, _ = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")

	if got := loader.callCount("src/a.go"); got != callsAfterR {
		t.Fatalf("loader called %d times after re-entry while reload in flight, want %d (dropped)", got, callsAfterR)
	}

	// Clean up.
	loader.release("src/a.go")
}

// TestReloadPreservesAnchorClamped verifies that a reload preserves
// the cursor and logical viewport anchor, clamped to new content. The
// assertion is made after the new revision's matching prepared layout
// installs, not against the old revision's layout.
func TestReloadPreservesAnchorClamped(t *testing.T) {
	// Create a file with 50 lines. At 80x24, the content height is
	// ~19 rows (24 minus filename row, list border, etc.), so
	// maxOffset is well above 10.
	lines := make([]filebuffer.Line, 50)
	for i := 0; i < 50; i++ {
		lines[i] = ml(i+1, "line-"+padNum(i))
	}
	m, loader := reloadSetup(t, lines, 50, 3)

	// Scroll down 10 rows.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, keyPress(tea.KeyDown))
	}
	offsetBefore := m.ViewportOffset()
	if offsetBefore == 0 {
		t.Fatal("after 10 down, ViewportOffset = 0, want > 0")
	}
	cursorBefore := m.CursorPosition()

	// Reload with the same 50-line content.
	loader.setBuffer("src/a.go", makeBuf(lines, 50, 3))
	m, cmd := update(t, m, keyPress('r'))
	lc := execReloadCmd(t, cmd)
	m = deliverCompletion(t, m, lc)

	// The anchor should be preserved after the new layout installs.
	if got := m.ViewportOffset(); got != offsetBefore {
		t.Fatalf("after reload, ViewportOffset = %d, want %d (anchor preserved)", got, offsetBefore)
	}
	if m.CursorPosition() != cursorBefore {
		t.Fatalf("after reload, CursorPosition = %d, want %d (preserved)", m.CursorPosition(), cursorBefore)
	}
}

// TestReloadPreservesAnchorClampedOnShrink verifies that a reload with
// fewer lines clamps the anchor to the new content's max offset.
func TestReloadPreservesAnchorClampedOnShrink(t *testing.T) {
	// Create a file with 50 lines. Scroll down to offset 10.
	lines := make([]filebuffer.Line, 50)
	for i := 0; i < 50; i++ {
		lines[i] = ml(i+1, "line-"+padNum(i))
	}
	m, loader := reloadSetup(t, lines, 50, 3)

	for i := 0; i < 10; i++ {
		m, _ = update(t, m, keyPress(tea.KeyDown))
	}
	offsetBefore := m.ViewportOffset()
	if offsetBefore == 0 {
		t.Fatal("after 10 down, ViewportOffset = 0, want > 0")
	}

	// Reload with only 5 lines. The old offset should be clamped
	// to the new content's maxOffset (which is 0 for 5 lines at
	// this content height).
	shortLines := make([]filebuffer.Line, 5)
	for i := 0; i < 5; i++ {
		shortLines[i] = ml(i+1, "short-"+padNum(i))
	}
	loader.setBuffer("src/a.go", makeBuf(shortLines, 5, 3))
	m, cmd := update(t, m, keyPress('r'))
	lc := execReloadCmd(t, cmd)
	m = deliverCompletion(t, m, lc)

	// The offset should be clamped to 0 (5 lines, content height > 5).
	if got := m.ViewportOffset(); got != 0 {
		t.Fatalf("after reload with 5 lines, ViewportOffset = %d, want 0 (clamped, was %d)", got, offsetBefore)
	}
}

// padNum formats a zero-padded 2-digit number for line content.
func padNum(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// TestReloadFailureReplacesWithUnreadable verifies that a failed reload
// replaces the old display with "(unreadable)" and shows the
// current-file failure overlay.
func TestReloadFailureReplacesWithUnreadable(t *testing.T) {
	m, loader := reloadSetup(t, []filebuffer.Line{ml(1, "old-content")}, 1, 3)

	// Make the reload fail.
	loader.setFailing("src/a.go", "permission denied")
	m, cmd := update(t, m, keyPress('r'))
	lc := execReloadCmd(t, cmd)
	m = deliverCompletion(t, m, lc)

	// The overlay should be open (read-failure overlay).
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open after failed reload")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	if m.OverlayFatal() {
		t.Fatalf("failed reload overlay should be non-fatal")
	}

	// Dismiss the overlay to see the panel.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	view := viewContent(m)
	if !strings.Contains(view, "(unreadable)") {
		t.Fatalf("after failed reload, view should contain '(unreadable)': %q", view)
	}
	if strings.Contains(view, "old-content") {
		t.Fatalf("after failed reload, view should not show old content: %q", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("after failed reload, view should not show Loading: %q", view)
	}
}

// TestReloadSecondFailureAppends verifies that a second consecutive
// reload failure appends exactly one new diagnostic occurrence to
// the open overlay with the reader's scroll position preserved.
func TestReloadSecondFailureAppends(t *testing.T) {
	m, loader := reloadSetup(t, []filebuffer.Line{ml(1, "old-content")}, 1, 3)

	// First reload fails.
	loader.setFailing("src/a.go", "permission denied")
	m, cmd := update(t, m, keyPress('r'))
	lc := execReloadCmd(t, cmd)
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open after first failed reload")
	}
	origOverlayText := m.OverlayText()

	// Scroll the overlay down.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyDown))
	m, _ = update(t, m, keyPressOrEscape(tea.KeyDown))
	scrollAfterScroll := m.OverlayScroll()

	// Press r again through the read-failure overlay — second
	// reload starts. The overlay stays open.
	m, cmd2 := update(t, m, keyPress('r'))
	lc2 := execReloadCmd(t, cmd2)
	m = deliverCompletion(t, m, lc2)

	// The overlay should still be open.
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open after second failed reload")
	}

	// Exactly one new diagnostic occurrence should be appended.
	overlayText := m.OverlayText()
	occurrences := strings.Count(overlayText, "permission denied")
	if occurrences != 2 {
		t.Fatalf("overlay should have 2 diagnostic occurrences after second failure, got %d: %s", occurrences, overlayText)
	}
	if overlayText == origOverlayText {
		t.Fatalf("overlay text should have changed after second failure")
	}

	// The overlay scroll position should be preserved.
	if m.OverlayScroll() != scrollAfterScroll {
		t.Fatalf("overlay scroll = %d after second failure, want %d (preserved)", m.OverlayScroll(), scrollAfterScroll)
	}
}

// TestReloadOneStopIndex verifies that r works with a one-stop index
// (a single matched line in a single file).
func TestReloadOneStopIndex(t *testing.T) {
	m, loader := reloadSetup(t, []filebuffer.Line{ml(1, "old-content")}, 1, 3)

	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "new-content")}, 1, 3))
	m, cmd := update(t, m, keyPress('r'))
	lc := execReloadCmd(t, cmd)
	m = deliverCompletion(t, m, lc)

	view := viewContent(m)
	if !strings.Contains(view, "new-content") {
		t.Fatalf("after reload with one stop, view should show new-content: %q", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("after reload with one stop, view should not show Loading: %q", view)
	}
}

// TestNoReloadOnDiskChange verifies that no reload is triggered by
// simulated disk changes without pressing r. The cached content
// remains stable until r is pressed.
func TestNoReloadOnDiskChange(t *testing.T) {
	m, loader := reloadSetup(t, []filebuffer.Line{ml(1, "old-content")}, 1, 3)
	callsBefore := loader.callCount("src/a.go")

	// Simulate a disk change: the loader would return new content.
	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "new-content")}, 1, 3))

	// Without pressing r, the panel should still show the old content.
	view := viewContent(m)
	if !strings.Contains(view, "old-content") {
		t.Fatalf("without r, view should show old-content (cache stable): %q", view)
	}
	if strings.Contains(view, "new-content") {
		t.Fatalf("without r, view should not show new-content (no reload): %q", view)
	}

	// The loader should not have been called again.
	if got := loader.callCount("src/a.go"); got != callsBefore {
		t.Fatalf("loader called %d times without r, want %d (no reload)", got, callsBefore)
	}

	// Now press r — the new content should appear.
	m, cmd := update(t, m, keyPress('r'))
	lc := execReloadCmd(t, cmd)
	m = deliverCompletion(t, m, lc)
	view = viewContent(m)
	if !strings.Contains(view, "new-content") {
		t.Fatalf("after r, view should show new-content: %q", view)
	}
}

// TestReloadFilenameRowIdentifiesPath verifies that the filename row
// keeps identifying the path after a reload.
func TestReloadFilenameRowIdentifiesPath(t *testing.T) {
	m, loader := reloadSetup(t, []filebuffer.Line{ml(1, "old-content")}, 1, 3)

	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "new-content")}, 1, 3))
	m, cmd := update(t, m, keyPress('r'))
	lc := execReloadCmd(t, cmd)
	m = deliverCompletion(t, m, lc)

	view := viewContent(m)
	if !strings.Contains(view, "a.go") {
		t.Fatalf("after reload, view should contain 'a.go' (filename row identifies path): %q", view)
	}
}

// TestReloadRevisionSupersedesGatedLayout verifies that a reload
// produces a new content revision, and a gated pre-reload layout
// (from a resize) released after the reload completes is discarded
// without replacing the reloaded content or its anchor — the Issue
// #17 revision-supersession test.
func TestReloadRevisionSupersedesGatedLayout(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	// Set up with a layout gate (buffered so we can release one
	// preparation at a time).
	fileGate := make(chan struct{})
	close(fileGate)
	layoutGate := make(chan struct{}, 10)
	loader := newFailingLoader()
	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "old-content")}, 1, 3))
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader.load),
		app.WithLayoutGate(layoutGate),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}

	// Execute the initial file load.
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	// cmd is the initial layout preparation (blocked on the gate).
	// Release it and deliver.
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, cmd)
	view := viewContent(m)
	if !strings.Contains(view, "old-content") {
		t.Fatalf("initial load should show old-content: %q", view)
	}

	// Scroll down to set an anchor.
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, keyPress(tea.KeyDown))
	}
	offsetBefore := m.ViewportOffset()

	// Trigger a resize — starts a layout preparation with revision 1
	// (gated). Save the command.
	m, resizeCmd := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if resizeCmd == nil {
		t.Fatal("resize should produce a layout command")
	}

	// Press r — reload starts.
	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "new-content")}, 1, 3))
	m, rCmd := update(t, m, keyPress('r'))
	// Execute the reload load command.
	lc := execReloadCmd(t, rCmd)
	// Deliver the FileLoadCompleteMsg — this increments the
	// revision and starts a new layout preparation (revision 2,
	// gated).
	m, reloadLayoutCmd := update(t, m, lc)
	if reloadLayoutCmd == nil {
		t.Fatal("reload should produce a layout command")
	}

	// Release the gate for the resize layout (revision 1).
	// It should be discarded (key mismatch: revision 1 != 2).
	layoutGate <- struct{}{}
	resizeMsg := execCmd(t, resizeCmd)
	if lr, ok := resizeMsg.(app.LayoutReadyMsg); ok {
		m, _ = update(t, m, lr)
		// The resize layout should have been discarded.
		// The content should still be new-content (from the reload).
	}

	// Release the gate for the reload layout (revision 2).
	// It should be installed.
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, reloadLayoutCmd)

	// The reloaded content should be shown.
	view = viewContent(m)
	if !strings.Contains(view, "new-content") {
		t.Fatalf("after supersession, view should show new-content: %q", view)
	}
	if strings.Contains(view, "old-content") {
		t.Fatalf("after supersession, view should not show old-content: %q", view)
	}

	// The anchor should be preserved (clamped to new content).
	// The new content has only 1 line, so the offset should be 0.
	if got := m.ViewportOffset(); got != 0 {
		t.Fatalf("after supersession, ViewportOffset = %d, want 0 (clamped to 1-line content, was %d before)", got, offsetBefore)
	}
}

// TestReloadPreservesAnchorAfterLayoutInstall verifies that the anchor
// is asserted after the new revision's matching prepared layout
// installs, not against the old revision's layout. This uses a
// layout gate to separate the load completion from the layout
// installation.
func TestReloadPreservesAnchorAfterLayoutInstall(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	// Create a file with 30 lines.
	lines := make([]filebuffer.Line, 30)
	for i := 0; i < 30; i++ {
		lines[i] = ml(i+1, "line-"+padNum(i))
	}

	fileGate := make(chan struct{})
	close(fileGate)
	layoutGate := make(chan struct{}, 10)
	loader := newFailingLoader()
	loader.setBuffer("src/a.go", makeBuf(lines, 30, 3))
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader.load),
		app.WithLayoutGate(layoutGate),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}

	// Execute the initial file load.
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	// Release the initial layout.
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, cmd)

	// Scroll down 5 rows.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, keyPress(tea.KeyDown))
	}
	offsetBefore := m.ViewportOffset()
	if offsetBefore != 5 {
		t.Fatalf("after 5 down, ViewportOffset = %d, want 5", offsetBefore)
	}

	// Reload with the same content.
	loader.setBuffer("src/a.go", makeBuf(lines, 30, 3))
	m, rCmd := update(t, m, keyPress('r'))
	lc := execReloadCmd(t, rCmd)
	// Deliver the FileLoadCompleteMsg — this increments the revision
	// and starts a new layout preparation (gated).
	m, reloadLayoutCmd := update(t, m, lc)

	// Before the new layout installs, the old layout is still
	// installed. The offset might still be 5 (from the old layout).
	// But we should NOT assert the anchor here — only after the
	// new layout installs.

	// Release the gate and deliver the new layout.
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, reloadLayoutCmd)

	// Now assert the anchor is preserved after the new layout installs.
	if got := m.ViewportOffset(); got != offsetBefore {
		t.Fatalf("after new layout installs, ViewportOffset = %d, want %d (anchor preserved)", got, offsetBefore)
	}
}
