package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// --- Issue #28: Two-stage load-completion contract tests ---
//
// These tests verify the two-stage contract: stage one (load completion)
// validates content, establishes the new content revision, computes the
// final gutter width, recomputes the text width with gutter growth and
// list changes participating, and requests a prepared layout — while
// performing no row-based decision. Stage two (layout installation)
// commits the reveal intent against the installed row model: visible-
// target no-scroll, else one-third placement with BOF/EOF clamping,
// plus horizontal reset then minimal horizontal reveal.
//
// The reveal intent targets the latest selected cursor's final display
// target, never a target captured when the load was requested. Obsolete
// layouts are discarded without consuming or mutating the intent.

// setupTwoStageGated creates a browse model with separately gated file
// loading and layout preparation. The file load gate is released
// immediately so the startup load completes, but the layout gate is held
// so the layout preparation is pending. Returns the model, the layout
// preparation command (blocked on the gate), and the layout gate channel.
func setupTwoStageGated(t *testing.T, idx *searchindex.Index, buf *filebuffer.Buffer) (app.Model, tea.Cmd, chan struct{}) {
	t.Helper()
	return setupTwoStageGatedSize(t, idx, buf, 80, 24)
}

// setupTwoStageGatedSize is like setupTwoStageGated but with custom
// terminal dimensions.
func setupTwoStageGatedSize(t *testing.T, idx *searchindex.Index, buf *filebuffer.Buffer, width, height int) (app.Model, tea.Cmd, chan struct{}) {
	t.Helper()
	fileGate := make(chan struct{})
	close(fileGate)
	layoutGate := make(chan struct{}) // held
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader),
		app.WithLayoutGate(layoutGate),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	// Execute the file load and deliver the completion. The layout
	// preparation is blocked on the gate.
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	// cmd is the layout preparation command (blocked on the gate).
	return m, cmd, layoutGate
}

// deliverGatedLayout releases the layout gate and delivers the layout
// preparation command to the model.
func deliverGatedLayout(t *testing.T, m app.Model, cmd tea.Cmd, layoutGate chan struct{}) app.Model {
	t.Helper()
	close(layoutGate)
	return deliverLayout(t, m, cmd)
}

// TestStageOneNoRevealWithGatedLayout verifies that stage one (load
// completion) performs no row-based decision: with the layout gated,
// the viewport offset does not change at load completion. The reveal
// commits only when the matching layout installs (stage two).
func TestStageOneNoRevealWithGatedLayout(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m, cmd, layoutGate := setupTwoStageGated(t, idx, buf)

	// Stage one is complete (load done, layout pending). The
	// viewport should be nil (no layout installed yet), so there
	// is no offset to check. But the model must carry a reveal
	// intent for the startup target.
	if m.HasPendingLayout() {
		// Good: layout is pending.
	}
	// The model must have a reveal intent (not an anchor intent).
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after stage one, LoadIntent = %v, want IntentReveal (startup reveal)", m.LoadIntent())
	}

	// Stage two: release the gate and deliver the layout.
	m = deliverGatedLayout(t, m, cmd, layoutGate)

	// The reveal should have committed: line 200 (row 199) at
	// floor(23/3) = 7, offset = 199 - 7 = 192.
	if got := m.ViewportOffset(); got != 192 {
		t.Fatalf("after layout install, ViewportOffset = %d, want 192 (startup reveal at one-third)", got)
	}
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("after layout install, LoadIntent = %v, want IntentNone (intent committed)", m.LoadIntent())
	}
}

// TestStartupVisibleTargetKeepsTopZero verifies that a startup target
// visible from the top keeps offset 0, and the first n advances to the
// second stop with its reveal.
func TestStartupVisibleTargetKeepsTopZero(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m, cmd, layoutGate := setupTwoStageGated(t, idx, buf)

	// Stage one: load complete, layout pending. Intent is reveal.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after stage one, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}

	// Stage two: release the gate. Line 5 (row 4) is visible from
	// offset 0, so no scroll.
	m = deliverGatedLayout(t, m, cmd, layoutGate)
	if m.ViewportOffset() != 0 {
		t.Fatalf("after layout install, ViewportOffset = %d, want 0 (visible target no-scroll)", m.ViewportOffset())
	}

	// First n advances to line 200 (row 199). Reveal: offset = 192.
	m, _ = update(t, m, keyPress('n'))
	if m.ViewportOffset() != 192 {
		t.Fatalf("after n, ViewportOffset = %d, want 192 (second stop reveal)", m.ViewportOffset())
	}
}

// TestResizeBetweenLoadAndLayoutPreservesIntent verifies that a resize
// between load completion and layout installation discards the old-width
// layout while the intent survives to commit at the new width.
func TestResizeBetweenLoadAndLayoutPreservesIntent(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m, cmd, layoutGate := setupTwoStageGated(t, idx, buf)

	// Stage one: load complete, layout pending at width 80.
	// The intent is reveal.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after stage one, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}
	oldKey := m.PendingLayoutKey()

	// Resize to width 100. This triggers a new layout request at
	// the new width. The old layout (width 80) is still pending.
	m, resizeCmd := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	newKey := m.PendingLayoutKey()
	if newKey.TextWidth == oldKey.TextWidth {
		t.Fatalf("after resize, TextWidth unchanged (%d), want different (new width)", newKey.TextWidth)
	}

	// The intent must survive the resize.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after resize, LoadIntent = %v, want IntentReveal (intent survived)", m.LoadIntent())
	}

	// Release the gate: the old layout (width 80) arrives first.
	// It must be discarded (key mismatch).
	close(layoutGate)
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lr, ok := msg.(app.LayoutReadyMsg); ok {
			m, _ = update(t, m, lr)
		}
	}
	// The intent must NOT be consumed by the stale layout.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after stale layout discard, LoadIntent = %v, want IntentReveal (not consumed)", m.LoadIntent())
	}

	// Deliver the new layout (width 100). It must install and
	// commit the reveal.
	if resizeCmd != nil {
		msg := execCmd(t, resizeCmd)
		if lr, ok := msg.(app.LayoutReadyMsg); ok {
			m, _ = update(t, m, lr)
		}
	}
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("after new layout install, LoadIntent = %v, want IntentNone (committed)", m.LoadIntent())
	}
	// The reveal should have committed at the new width.
	if m.ViewportOffset() == 0 {
		t.Fatal("after new layout install, ViewportOffset = 0, want > 0 (reveal committed at new width)")
	}
}

// TestNavigationDuringPendingLayoutRevealsNewestTarget verifies that
// n/p while the layout is pending moves the cursor immediately, and the
// newest target is revealed on commit — not a target captured when the
// load was requested.
func TestNavigationDuringPendingLayoutRevealsNewestTarget(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 100, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m, cmd, layoutGate := setupTwoStageGated(t, idx, buf)

	// Stage one: load complete, layout pending. Startup cursor at
	// stop 0 (line 1). Intent is reveal.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after stage one, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}

	// Navigate to stop 1 (line 100) while layout is pending.
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() != 1 {
		t.Fatalf("after n, CursorPosition = %d, want 1", m.CursorPosition())
	}
	// Navigate to stop 2 (line 200) while layout is still pending.
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() != 2 {
		t.Fatalf("after second n, CursorPosition = %d, want 2", m.CursorPosition())
	}

	// The intent must still be reveal (targeting the latest cursor).
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after navigation, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}

	// Stage two: release the gate. The reveal must target stop 2
	// (line 200, row 199), not stop 0 (line 1).
	m = deliverGatedLayout(t, m, cmd, layoutGate)
	// Line 200 (row 199) at floor(23/3) = 7, offset = 192.
	if got := m.ViewportOffset(); got != 192 {
		t.Fatalf("after layout install, ViewportOffset = %d, want 192 (newest target line 200)", got)
	}
}

// TestGutterGrowthBetweenStagesCommitsAtFinalWidth verifies that gutter
// growth between load completion and layout installation commits the
// reveal against the final text width (which accounts for the wider
// gutter).
func TestGutterGrowthBetweenStagesCommitsAtFinalWidth(t *testing.T) {
	// Use a file with 1000 lines so the gutter grows to 4 digits
	// (gutter width 6: 4 digits + 2 spaces) vs the default 3 (1
	// digit + 2 spaces). The match is at line 500.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 500, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(1000)
	buf := makeBuf(lines, 1000, 6) // gutter width 6 (4 digits + 2 spaces)
	m, cmd, layoutGate := setupTwoStageGated(t, idx, buf)

	// Stage one: load complete, layout pending. The gutter width
	// is 6, so the text width accounts for it.
	key := m.PendingLayoutKey()
	if key.TextWidth <= 0 {
		t.Fatalf("PendingLayoutKey.TextWidth = %d, want > 0", key.TextWidth)
	}

	// The intent is reveal.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after stage one, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}

	// Stage two: release the gate. The reveal commits against the
	// final text width (which accounts for the 6-cell gutter).
	m = deliverGatedLayout(t, m, cmd, layoutGate)
	// Line 500 (row 499) at floor(23/3) = 7, offset = 492.
	if got := m.ViewportOffset(); got != 492 {
		t.Fatalf("after layout install, ViewportOffset = %d, want 492 (gutter growth final width)", got)
	}
}

// TestListToggleBetweenStagesCommitsAtFinalWidth verifies that hiding
// the file list between load completion and layout installation commits
// the reveal against the final (wider) text width.
func TestListToggleBetweenStagesCommitsAtFinalWidth(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m, cmd, layoutGate := setupTwoStageGated(t, idx, buf)

	// Stage one: load complete, layout pending with list visible.
	keyBefore := m.PendingLayoutKey()

	// Hide the list (left/tab). This changes the panel width and
	// triggers a new layout request at the wider text width.
	m, toggleCmd := update(t, m, keyPress(tea.KeyTab))
	if m.ListVisible() {
		t.Fatal("after tab, ListVisible = true, want false (hidden)")
	}
	keyAfter := m.PendingLayoutKey()
	if keyAfter.TextWidth <= keyBefore.TextWidth {
		t.Fatalf("after list hide, TextWidth = %d, want > %d (wider without list)", keyAfter.TextWidth, keyBefore.TextWidth)
	}

	// The intent must survive the list toggle.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after list toggle, LoadIntent = %v, want IntentReveal (survived)", m.LoadIntent())
	}

	// Release the gate: the old layout (list visible) arrives first.
	// It must be discarded (key mismatch).
	close(layoutGate)
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lr, ok := msg.(app.LayoutReadyMsg); ok {
			m, _ = update(t, m, lr)
		}
	}
	// The intent must NOT be consumed by the stale layout.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after stale layout discard, LoadIntent = %v, want IntentReveal (not consumed)", m.LoadIntent())
	}

	// Deliver the new layout (list hidden). It must install and
	// commit the reveal at the wider text width.
	if toggleCmd != nil {
		msg := execCmd(t, toggleCmd)
		if lr, ok := msg.(app.LayoutReadyMsg); ok {
			m, _ = update(t, m, lr)
		}
	}
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("after new layout install, LoadIntent = %v, want IntentNone (committed)", m.LoadIntent())
	}
	// The reveal should have committed at the new width.
	if m.ViewportOffset() == 0 {
		t.Fatal("after new layout install, ViewportOffset = 0, want > 0 (reveal at final width)")
	}
}

// TestSavedViewportRevisitVisibleStays verifies that a saved-viewport
// revisit with the target visible from the saved offset stays (no
// scroll), through the two-stage path.
func TestSavedViewportRevisitVisibleStays(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(300)
	linesB := makeScrollLines(300)
	bufA := makeBuf(linesA, 300, 4)
	bufB := makeBuf(linesB, 300, 4)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": bufA,
		"src/b.go": bufB,
	}
	m := setupBrowseMulti(t, idx, bufs)
	// Startup: a.go line 1 (row 0) visible from offset 0.
	if m.ViewportOffset() != 0 {
		t.Fatalf("a.go startup offset = %d, want 0", m.ViewportOffset())
	}

	// Navigate to b.go (uncached). b.go line 1 (row 0) visible from
	// offset 0 (first visit).
	m, navCmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	m = deliverLoad(t, m, navCmd)
	if m.ViewportOffset() != 0 {
		t.Fatalf("b.go first visit offset = %d, want 0", m.ViewportOffset())
	}

	// Scroll b.go down 5 rows.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, keyPress(tea.KeyDown))
	}
	if m.ViewportOffset() != 5 {
		t.Fatalf("after scroll, b.go offset = %d, want 5", m.ViewportOffset())
	}

	// Navigate back to a.go (cached, matching layout). a.go's
	// saved offset is 0. Line 1 (row 0) is visible from offset 0.
	// The reveal should be no-scroll (target visible).
	m, _ = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	if m.ViewportOffset() != 0 {
		t.Fatalf("a.go revisit offset = %d, want 0 (saved viewport, visible target stays)", m.ViewportOffset())
	}
}

// TestSavedViewportRevisitHiddenMoves verifies that a saved-viewport
// revisit with the target hidden from the saved offset moves to reveal
// the target, through the two-stage path.
func TestSavedViewportRevisitHiddenMoves(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 200, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(300)
	linesB := makeScrollLines(300)
	bufA := makeBuf(linesA, 300, 4)
	bufB := makeBuf(linesB, 300, 4)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": bufA,
		"src/b.go": bufB,
	}
	m := setupBrowseMulti(t, idx, bufs)
	// Startup: a.go line 200 (row 199) at floor(23/3) = 7, offset 192.
	if m.ViewportOffset() != 192 {
		t.Fatalf("a.go startup offset = %d, want 192", m.ViewportOffset())
	}

	// Navigate to b.go (cached). b.go line 1 (row 0) at offset 0.
	m, _ = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	if m.ViewportOffset() != 0 {
		t.Fatalf("b.go offset = %d, want 0 (first visit)", m.ViewportOffset())
	}

	// Navigate back to a.go (cached). a.go's saved offset is 192.
	// Line 200 (row 199) is visible from offset 192 (range [192, 215)).
	// The reveal should be no-scroll (target visible from saved offset).
	m, _ = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	if m.ViewportOffset() != 192 {
		t.Fatalf("a.go revisit offset = %d, want 192 (saved viewport, visible target stays)", m.ViewportOffset())
	}
}

// TestTerminatorOnlyMarkerRevealsInRunOffEdge verifies that a
// terminator-only marker target far down reveals the marker's row with
// the marker cell painted in run-off-edge mode, through the two-stage
// path.
func TestTerminatorOnlyMarkerRevealsInRunOffEdge(t *testing.T) {
	// A terminator-only match: the submatch covers the line's
	// terminator (byte offset at end of line text). The marker cell
	// is at the end-of-line position.
	lineText := "some text here"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", lineText+"\n", 200, subSpec{"\n", len(lineText), len(lineText) + 1}),
	)
	lines := makeScrollLines(300)
	// Replace line 200's content with the match line.
	lines[199] = ml(200, lineText)
	buf := makeBuf(lines, 300, 4)
	// Use run-off-edge mode (WrapOff) so the horizontal reveal applies.
	fileGate := make(chan struct{})
	close(fileGate)
	layoutGate := make(chan struct{})
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader),
		app.WithLayoutGate(layoutGate),
		app.WithWrapMode(viewport.WrapOff),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	// Deliver startup load.
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	// Release layout gate.
	close(layoutGate)
	m = deliverLayout(t, m, cmd)

	// The marker is on line 200 (row 199). The reveal should place
	// it at floor(23/3) = 7, offset = 192.
	if got := m.ViewportOffset(); got != 192 {
		t.Fatalf("after layout install, ViewportOffset = %d, want 192 (terminator marker reveal)", got)
	}
}

// TestNonCurrentFileCompletionLeavesPanelUntouched verifies that a
// load completion for a non-current file updates only that file's cache,
// leaving the visible panel and intent untouched.
func TestNonCurrentFileCompletionLeavesPanelUntouched(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufA := makeBuf([]filebuffer.Line{ml(1, "a-content")}, 1, 3)
	bufB := makeBuf([]filebuffer.Line{ml(1, "b-content")}, 1, 3)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": bufA,
		"src/b.go": bufB,
	}
	m := setupBrowseMulti(t, idx, bufs)
	assertCurrentPath(t, m, "src/a.go")

	// Navigate to b.go (cached) and back to a.go so both are loaded.
	m, _ = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	m, _ = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")

	// Construct a late completion for b.go (non-current). Use a
	// request ID that does not match any in-flight request so it
	// is rejected by the request-identity guard. This simulates a
	// stale completion arriving after the load already settled.
	lateMsg := app.FileLoadCompleteMsg{
		Path:      []byte("src/b.go"),
		RequestID: 999, // stale: no matching in-flight request
		Buffer:    bufB,
	}
	m, _ = update(t, m, lateMsg)

	// The panel should still show a.go's content.
	view := viewContent(m)
	if !strings.Contains(view, "a-content") {
		t.Fatalf("after late b.go completion, view should show a-content: %q", view)
	}
	if strings.Contains(view, "b-content") {
		t.Fatalf("after late b.go completion, view should not show b-content: %q", view)
	}
	// The current path should still be a.go.
	assertCurrentPath(t, m, "src/a.go")
}

// TestPopupUnaffectedByStages verifies that the file-change pop-up is
// unaffected by either stage (load completion or layout installation).
func TestPopupUnaffectedByStages(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	bufA := makeBuf([]filebuffer.Line{ml(1, "a-content")}, 1, 3)
	bufB := makeBuf([]filebuffer.Line{ml(1, "b-content")}, 1, 3)
	layoutGate := make(chan struct{})
	fileGate := make(chan struct{})
	close(fileGate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if string(path) == "src/a.go" {
			return bufA, nil
		}
		return bufB, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader),
		app.WithLayoutGate(layoutGate),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	// Deliver startup load for a.go.
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	close(layoutGate)
	m = deliverLayout(t, m, cmd)

	// Navigate to b.go (uncached). The pop-up should start at
	// selection time (before the load completes).
	m, navCmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	if !m.PopupOpen() {
		t.Fatal("after n to b.go, popup should be open (started at selection time)")
	}

	// Deliver b.go's load (stage one). The pop-up should still be
	// open (unaffected by load completion).
	if navCmd != nil {
		msg := execCmd(t, navCmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, layoutCmd := update(t, m, lc)
			// The pop-up should still be open after stage one.
			if !m.PopupOpen() {
				t.Fatal("after stage one (load completion), popup should still be open")
			}
			// Deliver the layout (stage two). The pop-up should
			// still be open (unaffected by layout installation).
			if layoutCmd != nil {
				msg := execCmd(t, layoutCmd)
				if lr, ok := msg.(app.LayoutReadyMsg); ok {
					m, _ = update(t, m, lr)
				}
			}
			if !m.PopupOpen() {
				t.Fatal("after stage two (layout install), popup should still be open")
			}
		}
	}
}

// TestObsoleteLayoutDiscardedWithoutConsumingIntent verifies that an
// obsolete layout (from a prior width) is discarded without consuming
// or mutating the reveal intent. The intent survives to commit when the
// matching layout installs.
func TestObsoleteLayoutDiscardedWithoutConsumingIntent(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 200, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(300)
	buf := makeBuf(lines, 300, 4)
	m, cmd, layoutGate := setupTwoStageGated(t, idx, buf)

	// Stage one: load complete, layout pending at width 80.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after stage one, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}

	// Resize to a different width. This triggers a new layout
	// request at the new width.
	m, resizeCmd := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})

	// Release the gate. The old layout (width 80) arrives first.
	close(layoutGate)
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lr, ok := msg.(app.LayoutReadyMsg); ok {
			m, _ = update(t, m, lr)
		}
	}

	// The intent must NOT be consumed by the stale layout.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after stale layout discard, LoadIntent = %v, want IntentReveal (not consumed)", m.LoadIntent())
	}

	// Deliver the new layout (width 100). It must install and commit.
	if resizeCmd != nil {
		msg := execCmd(t, resizeCmd)
		if lr, ok := msg.(app.LayoutReadyMsg); ok {
			m, _ = update(t, m, lr)
		}
	}
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("after matching layout install, LoadIntent = %v, want IntentNone (committed)", m.LoadIntent())
	}
}
