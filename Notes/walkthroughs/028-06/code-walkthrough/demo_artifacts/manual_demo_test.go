//go:build manual_demo

package app_test

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
)

// TestManualDemoTwoStageLoadCompletion demonstrates the Issue #28
// two-stage load-completion contract as a gated model test with an
// injected loader:
//   - Open a file (src/a.go) with 50 lines; the startup reveal targets
//     line 1 (offset 0).
//   - Navigate to line 25 while the layout is gated; the reveal
//     intent is carried (not committed against the old row model).
//   - Release the layout; the reveal commits against the matching
//     rows (offset 17, not 0).
//   - Reload (r) sets IntentReloadAnchor; the anchor is preserved
//     (no reveal) after the matching layout installs.
//   - Navigate during a reload replaces IntentReloadAnchor with
//     IntentReveal; the matching layout reveals the latest target.
//
// This mirrors the shell-based manual route but uses the injected
// loader so it runs without filesystem changes. The injected-loader
// model tests in two_stage_test.go and reload_intent_test.go remain
// the authoritative deterministic verification.
func TestManualDemoTwoStageLoadCompletion(t *testing.T) {
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
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	// Deliver the startup load (gated layout).
	if cmd != nil {
		msg := execCmd(t, cmd)
		if lc, ok := msg.(app.FileLoadCompleteMsg); ok {
			m, cmd = update(t, m, lc)
		}
	}
	// Stage one complete: the intent is IntentReveal, but the
	// layout is gated (not yet installed).
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("Step 1: after load, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}
	fmt.Printf("Step 1 (startup load): LoadIntent = IntentReveal (carried, not committed)\n")

	// Step 2: Navigate to line 25 while the layout is gated. The
	// intent is still IntentReveal (latest selection).
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() != 1 {
		t.Fatalf("Step 2: after n, CursorPosition = %d, want 1", m.CursorPosition())
	}
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("Step 2: after n, LoadIntent = %v, want IntentReveal", m.LoadIntent())
	}
	fmt.Printf("Step 2 (navigate during gated layout): LoadIntent = IntentReveal (latest selection)\n")

	// Step 3: Release the layout. The reveal commits against the
	// matching rows (line 25, row 24, offset 17).
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, cmd)
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("Step 3: after layout install, LoadIntent = %v, want IntentNone (committed)", m.LoadIntent())
	}
	if got := m.ViewportOffset(); got != 17 {
		t.Fatalf("Step 3: after layout install, ViewportOffset = %d, want 17 (reveal line 25)", got)
	}
	fmt.Printf("Step 3 (release layout): ViewportOffset = 17 (reveal committed against matching rows)\n")

	// Step 4: Reload (r) sets IntentReloadAnchor. The anchor is
	// preserved (no reveal) after the matching layout installs.
	loader.setBuffer("src/a.go", makeBuf(lines, 50, 3))
	m, rCmd := update(t, m, keyPress('r'))
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("Step 4: after r, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}
	fmt.Printf("Step 4 (press r): LoadIntent = IntentReloadAnchor (preserve anchor, no reveal)\n")

	// Deliver the reload's load completion (gated layout).
	lc := execReloadCmd(t, rCmd)
	m, layoutCmd := update(t, m, lc)
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("Step 4: after reload completion, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}
	// Release the matching layout. The anchor is preserved (offset 17).
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, layoutCmd)
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("Step 4: after layout install, LoadIntent = %v, want IntentNone (committed)", m.LoadIntent())
	}
	if got := m.ViewportOffset(); got != 17 {
		t.Fatalf("Step 4: after reload, ViewportOffset = %d, want 17 (anchor preserved)", got)
	}
	fmt.Printf("Step 4 (reload completes): ViewportOffset = 17 (anchor preserved, no reveal)\n")

	// Step 5: Reload again, then navigate during the reload. The
	// navigation replaces IntentReloadAnchor with IntentReveal.
	loader.setBuffer("src/a.go", makeBuf(lines, 50, 3))
	m, rCmd = update(t, m, keyPress('r'))
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("Step 5: after r, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}
	lc = execReloadCmd(t, rCmd)
	m, layoutCmd = update(t, m, lc)
	if m.LoadIntent() != app.IntentReloadAnchor {
		t.Fatalf("Step 5: after reload completion, LoadIntent = %v, want IntentReloadAnchor", m.LoadIntent())
	}
	// Navigate to line 1 (stop 0) while the layout is pending.
	m, _ = update(t, m, keyPress('p'))
	if m.CursorPosition() != 0 {
		t.Fatalf("Step 5: after p, CursorPosition = %d, want 0", m.CursorPosition())
	}
	if m.LoadIntent() != app.IntentReveal {
		t.Fatalf("Step 5: after p during reload, LoadIntent = %v, want IntentReveal (replaced)", m.LoadIntent())
	}
	fmt.Printf("Step 5 (navigate during reload): LoadIntent = IntentReveal (replaced reload intent)\n")

	// Release the matching layout. The reveal targets line 1 (row 0),
	// visible from offset 0 (no scroll). But the saved anchor was 17,
	// so the reveal moves to 0.
	layoutGate <- struct{}{}
	m = deliverLayout(t, m, layoutCmd)
	if m.LoadIntent() != app.IntentNone {
		t.Fatalf("Step 5: after layout install, LoadIntent = %v, want IntentNone (committed)", m.LoadIntent())
	}
	if got := m.ViewportOffset(); got != 0 {
		t.Fatalf("Step 5: after layout install, ViewportOffset = %d, want 0 (reveal line 1, not anchor 17)", got)
	}
	fmt.Printf("Step 5 (release layout): ViewportOffset = 0 (reveal line 1, not anchor 17)\n")

	fmt.Printf("\nManual demonstration passed: two-stage load-completion behavior matches the Issue #28 contracts.\n")
}
