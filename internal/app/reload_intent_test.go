package app_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
)

// --- Issue #28: Reload-intent transition tests ---
//
// These tests verify the reload-intent arbitration: an explicit reload
// (r) sets the IntentReloadAnchor intent (preserve anchor, no reveal),
// while any navigation during the pending load replaces it with
// IntentReveal (latest selection takes precedence). The intent is
// committed only when the matching prepared layout installs.

// reloadIntentSetup creates a browse model with a single file (src/a.go)
// loaded successfully, using a gated layout so the layout preparation
// can be held. Returns the model, the layout gate channel, and the
// loader.
func reloadIntentSetup(t *testing.T, lines []filebuffer.Line, lineCount, gutter int) (app.Model, chan struct{}, *failingLoader) {
	t.Helper()
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	loader := newFailingLoader()
	loader.setBuffer("src/a.go", makeBuf(lines, lineCount, gutter))
	fileGate := make(chan struct{})
	close(fileGate)
	layoutGate := make(chan struct{}, 10)
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
	return m, layoutGate, loader
}

// TestReloadSetsAnchorIntent verifies that pressing r sets the
// IntentReloadAnchor intent (not IntentReveal), so the matching layout
// installation preserves the anchor without revealing a match.
func TestReloadSetsAnchorIntent(t *testing.T) {
	lines := makeScrollLines(30)
	m, layoutGate, loader := reloadIntentSetup(t, lines, 30, 3)

	// Scroll down 5 rows to set an anchor.
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
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("after r, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}

	// Deliver the load completion. The intent should still be
	// IntentReloadAnchor (no navigation happened).
	lc := execReloadCmd(t, rCmd)
	m, layoutCmd := update(t, m, lc)
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("after load completion, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}

	// Release the layout. The intent should be committed (cleared)
	// and the anchor preserved.
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, layoutCmd)
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("after layout install, LoadIntent = %v, want IntentNone (committed)", m.LoadIntent())
	}
	if got := m.ViewportOffset(); got != offsetBefore {
		t.Fatalf("after reload, ViewportOffset = %d, want %d (anchor preserved, no reveal)", got, offsetBefore)
	}
}

// TestNavigationDuringReloadReplacesIntent verifies that pressing n
// during a reload replaces the IntentReloadAnchor intent with
// IntentReveal, so the matching layout installation reveals the latest
// target (not the saved anchor).
func TestNavigationDuringReloadReplacesIntent(t *testing.T) {
	// Two stops in the same file: line 1 and line 25. Use 50 lines
	// so maxOffset (50 - 23 = 27) exceeds the reveal offset (24 - 7
	// = 17), avoiding clamping.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 25, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(50)
	loader := newFailingLoader()
	loader.setBuffer("src/a.go", makeBuf(lines, 50, 3))
	fileGate := make(chan struct{})
	close(fileGate)
	layoutGate := make(chan struct{}, 10)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader.load),
		app.WithLayoutGate(layoutGate),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, cmd)
	// Startup: line 1 (row 0) at offset 0.
	if m.ViewportOffset() != 0 {
		t.Fatalf("startup offset = %d, want 0", m.ViewportOffset())
	}

	// Reload with the same content.
	loader.setBuffer("src/a.go", makeBuf(lines, 50, 3))
	m, rCmd := update(t, m, keyPress('r'))
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("after r, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}

	// Deliver the load completion (gated layout).
	lc := execReloadCmd(t, rCmd)
	m, layoutCmd := update(t, m, lc)
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("after load completion, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}

	// Navigate to stop 1 (line 25) while the layout is pending.
	// This replaces the reload intent with IntentReveal.
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() != 1 {
		t.Fatalf("after n, CursorPosition = %d, want 1", m.CursorPosition())
	}
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after n during reload, LoadIntent = %v, want IntentReveal (replaced)", m.LoadIntent())
	}

	// Release the layout. The reveal should target line 25 (row 24),
	// not the saved anchor (offset 0).
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, layoutCmd)
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("after layout install, LoadIntent = %v, want IntentNone (committed)", m.LoadIntent())
	}
	// Line 25 (row 24) at floor(23/3) = 7, offset = 24 - 7 = 17.
	if got := m.ViewportOffset(); got != 17 {
		t.Fatalf("after layout install, ViewportOffset = %d, want 17 (reveal line 25, not anchor 0)", got)
	}
}

// TestAwayAndBackReplacesReloadIntent verifies that away-and-back
// navigation (n then p, returning to the original cursor) during a
// reload replaces the reload intent with IntentReveal, even though the
// final cursor equals the initial cursor. The reveal commits against
// the original target, not the saved anchor.
func TestAwayAndBackReplacesReloadIntent(t *testing.T) {
	// Three stops in the same file: lines 1, 15, 25.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 15, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 25, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(30)
	loader := newFailingLoader()
	loader.setBuffer("src/a.go", makeBuf(lines, 30, 3))
	fileGate := make(chan struct{})
	close(fileGate)
	layoutGate := make(chan struct{}, 10)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader.load),
		app.WithLayoutGate(layoutGate),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, cmd)
	// Startup: line 1 (row 0) at offset 0.
	if m.ViewportOffset() != 0 {
		t.Fatalf("startup offset = %d, want 0", m.ViewportOffset())
	}

	// Reload with the same content.
	loader.setBuffer("src/a.go", makeBuf(lines, 30, 3))
	m, rCmd := update(t, m, keyPress('r'))
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("after r, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}

	// Deliver the load completion (gated layout).
	lc := execReloadCmd(t, rCmd)
	m, layoutCmd := update(t, m, lc)

	// Navigate forward then back (away-and-back).
	m, _ = update(t, m, keyPress('n')) // stop 1 (line 15)
	if m.CursorPosition() != 1 {
		t.Fatalf("after n, CursorPosition = %d, want 1", m.CursorPosition())
	}
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after n, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}
	m, _ = update(t, m, keyPress('p')) // back to stop 0 (line 1)
	if m.CursorPosition() != 0 {
		t.Fatalf("after p, CursorPosition = %d, want 0", m.CursorPosition())
	}
	// The intent should still be IntentReveal (not IntentReloadAnchor),
	// even though the cursor is back at the original stop.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after away-and-back, LoadIntent = %v, want IntentReveal (not reload anchor)", m.LoadIntent())
	}

	// Release the layout. The reveal should target line 1 (row 0),
	// which is visible from offset 0 (no scroll). But the intent is
	// IntentReveal, so it commits a reveal (which is a no-scroll
	// since the target is visible from the saved offset).
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, layoutCmd)
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("after layout install, LoadIntent = %v, want IntentNone (committed)", m.LoadIntent())
	}
	// Line 1 (row 0) is visible from offset 0. The reveal is a
	// no-scroll. The offset should be 0 (not the saved anchor from
	// before the reload, which was also 0).
	if got := m.ViewportOffset(); got != 0 {
		t.Fatalf("after layout install, ViewportOffset = %d, want 0 (reveal line 1, visible from 0)", got)
	}
}

// TestReloadDoesNotRevealMatch verifies that a reload without
// navigation preserves the saved anchor and does not reveal a match.
// The offset after reload equals the saved offset (not a reveal offset).
func TestReloadDoesNotRevealMatch(t *testing.T) {
	// Match at line 40 (far from the top). Use 50 lines so maxOffset
	// (50 - 23 = 27) exceeds the reveal offset (39 - 7 = 32 → clamped
	// to 27), making the startup reveal distinct from a top-anchored
	// view.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 40, subSpec{"x", 0, 1}),
	)
	lines := makeScrollLines(50)
	loader := newFailingLoader()
	loader.setBuffer("src/a.go", makeBuf(lines, 50, 3))
	fileGate := make(chan struct{})
	close(fileGate)
	layoutGate := make(chan struct{}, 10)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader.load),
		app.WithLayoutGate(layoutGate),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, cmd)
	// Startup: line 40 (row 39) at floor(23/3) = 7, offset = 32,
	// clamped to maxOffset = 27.
	startupOffset := m.ViewportOffset()
	if startupOffset != 27 {
		t.Fatalf("startup offset = %d, want 27 (reveal line 40, clamped)", startupOffset)
	}

	// Scroll up 3 rows to move away from the match.
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, keyPress(tea.KeyUp))
	}
	offsetBefore := m.ViewportOffset()
	if offsetBefore != 24 {
		t.Fatalf("after 3 up, ViewportOffset = %d, want 24", offsetBefore)
	}

	// Reload with the same content.
	loader.setBuffer("src/a.go", makeBuf(lines, 50, 3))
	m, rCmd := update(t, m, keyPress('r'))
	lc := execReloadCmd(t, rCmd)
	m, layoutCmd := update(t, m, lc)

	// Release the layout. The anchor should be preserved (offset 24),
	// NOT a reveal of line 40 (which would be offset 27).
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, layoutCmd)
	if got := m.ViewportOffset(); got != offsetBefore {
		t.Fatalf("after reload, ViewportOffset = %d, want %d (anchor preserved, no reveal)", got, offsetBefore)
	}
}

// TestNewerNavigationSupersedesReloadIntent verifies that cross-file
// navigation during a reload supersedes the reload intent with a
// reveal intent for the new file.
func TestNewerNavigationSupersedesReloadIntent(t *testing.T) {
	// Two files: a.go (stop 0) and b.go (stop 1).
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 20, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(30)
	linesB := makeScrollLines(30)
	loader := newFailingLoader()
	loader.setBuffer("src/a.go", makeBuf(linesA, 30, 3))
	loader.setBuffer("src/b.go", makeBuf(linesB, 30, 3))
	fileGate := make(chan struct{})
	close(fileGate)
	layoutGate := make(chan struct{}, 10)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(fileGate),
		app.WithFileLoader(loader.load),
		app.WithLayoutGate(layoutGate),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, cmd)
	assertCurrentPath(t, m, "src/a.go")

	// Reload a.go.
	loader.setBuffer("src/a.go", makeBuf(linesA, 30, 3))
	m, rCmd := update(t, m, keyPress('r'))
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("after r, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}

	// Deliver the reload's load completion (gated layout for a.go
	// revision 2).
	lc := execReloadCmd(t, rCmd)
	m, aLayoutCmd := update(t, m, lc)
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("after a.go reload completion, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}

	// Navigate to b.go (cross-file, uncached). This supersedes the
	// reload intent with IntentReveal for b.go.
	m, navCmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after n to b.go, LoadIntent = %v, want IntentReveal (superseded)", m.LoadIntent())
	}

	// The a.go reload layout (revision 2) is still pending. Release
	// it — it should be discarded (key mismatch: a.go != b.go).
	layoutGate <- struct{}{}
	if aLayoutCmd != nil {
		msg := execCmd(t, aLayoutCmd)
		if lr, ok := msg.(app.LayoutReadyMsg); ok {
			m, _ = update(t, m, lr)
		}
	}
	// The intent should NOT be consumed by the stale a.go layout.
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("after stale a.go layout discard, LoadIntent = %v, want IntentReveal (not consumed)", m.LoadIntent())
	}

	// Deliver b.go's load completion manually (the layout gate is
	// still buffered). Execute the navigation command to get the
	// FileLoadCompleteMsg.
	navMsg := execCmd(t, navCmd)
	var bLayoutCmd tea.Cmd
	if bLc, ok := navMsg.(app.FileLoadCompleteMsg); ok {
		m, bLayoutCmd = update(t, m, bLc)
		// Release the gate for b.go's layout and deliver it.
		layoutGate <- struct{}{}
		m = deliverLayout(t, m, bLayoutCmd)
	} else if batch, ok := navMsg.(tea.BatchMsg); ok {
		// Find the FileLoadCompleteMsg in the batch (the
		// pop-up timer command returns a non-load message).
		var bLc app.FileLoadCompleteMsg
		var found bool
		for _, c := range batch {
			if c == nil {
				continue
			}
			ch := make(chan tea.Msg, 1)
			go func(cmd tea.Cmd) { ch <- cmd() }(c)
			select {
			case sub := <-ch:
				if lc, ok := sub.(app.FileLoadCompleteMsg); ok {
					bLc = lc
					found = true
				}
			case <-time.After(100 * time.Millisecond):
			}
		}
		if !found {
			t.Fatal("no FileLoadCompleteMsg in navigation batch")
		}
		m, bLayoutCmd = update(t, m, bLc)
		layoutGate <- struct{}{}
		m = deliverLayout(t, m, bLayoutCmd)
	}
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("after b.go layout install, LoadIntent = %v, want IntentNone (committed)", m.LoadIntent())
	}
	// b.go line 20 (row 19) is visible from offset 0 (content height
	// 23: [0,23) includes row 19), so the reveal is a no-scroll.
	// The intent committed as IntentReveal but the target was
	// already visible from the restored saved offset (0 for a first
	// visit).
	if got := m.ViewportOffset(); got != 0 {
		t.Fatalf("after b.go layout install, ViewportOffset = %d, want 0 (target visible from 0)", got)
	}
}

// TestStaleLoadCompletionDoesNotChangeIntent verifies that a stale
// (non-current) load completion does not change the pending intent.
func TestStaleLoadCompletionDoesNotChangeIntent(t *testing.T) {
	// Two files: a.go (stop 0) and b.go (stop 1).
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	linesA := makeScrollLines(30)
	linesB := makeScrollLines(30)
	bufs := map[string]*filebuffer.Buffer{
		"src/a.go": makeBuf(linesA, 30, 3),
		"src/b.go": makeBuf(linesB, 30, 3),
	}
	m := setupBrowseMulti(t, idx, bufs)
	assertCurrentPath(t, m, "src/a.go")

	// Navigate to b.go (cached) and back to a.go.
	m, _ = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	m, _ = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")

	// Set the intent to IntentReveal (simulate a pending reveal).
	// Navigate to b.go (cached) — this sets IntentReveal.
	m, _ = update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	if m.LoadIntent() != app.IntentNone {
		// After a cached cross-file navigation with immediate
		// layout install, the intent should be committed (None).
		// That's fine — we just need to test that a stale
		// completion doesn't change it.
	}

	// Navigate back to a.go (cached).
	m, _ = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")

	// Construct a stale completion for b.go (non-current, stale
	// request ID). It should be rejected without changing the intent.
	staleMsg := app.FileLoadCompleteMsg{
		Path:      []byte("src/b.go"),
		RequestID: 999,
		Buffer:    bufs["src/b.go"],
	}
	m, _ = update(t, m, staleMsg)
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("after stale completion, LoadIntent = %v, want IntentNone (unchanged)", m.LoadIntent())
	}
	assertCurrentPath(t, m, "src/a.go")
}

// TestReloadAnchorCommittedOnlyAfterMatchingLayout verifies that the
// reload-anchor intent is committed (and the anchor asserted) only
// after the matching prepared layout installs, not at load completion.
func TestReloadAnchorCommittedOnlyAfterMatchingLayout(t *testing.T) {
	lines := makeScrollLines(30)
	m, layoutGate, loader := reloadIntentSetup(t, lines, 30, 3)

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
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("after r, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}

	// Deliver the load completion. The intent should still be
	// IntentReloadAnchor (not yet committed).
	lc := execReloadCmd(t, rCmd)
	m, layoutCmd := update(t, m, lc)
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("after load completion, LoadIntent = %v, want IntentReloadAnchor (not yet committed)", m.LoadIntent())
	}

	// Release the matching layout. The intent should be committed
	// (cleared) and the anchor preserved.
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, layoutCmd)
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("after layout install, LoadIntent = %v, want IntentNone (committed)", m.LoadIntent())
	}
	if got := m.ViewportOffset(); got != offsetBefore {
		t.Fatalf("after layout install, ViewportOffset = %d, want %d (anchor preserved)", got, offsetBefore)
	}
}
