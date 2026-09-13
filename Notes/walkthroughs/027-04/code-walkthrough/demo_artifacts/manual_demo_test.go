//go:build manual_demo

package app_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
)

// TestManualDemoExplicitReload demonstrates the Issue #27 manual
// case as a gated model test with an injected loader:
//   - Open a file (src/a.go) with 30 lines.
//   - Scroll down several rows.
//   - Append lines externally (the loader would return new content)
//     while the display remains unchanged (cache stable until r).
//   - Press r and show the new content at the same top position
//     (anchor preserved).
//   - Delete the file (make the loader fail) and press r to show
//     "(unreadable)".
//   - Restore the file (make the loader succeed again) and press r
//     to show content again.
//   - Reload a single-match search via r (one-stop index route).
//
// This mirrors the shell-based manual route but uses the injected
// loader so it runs without filesystem changes. The injected-loader
// model tests in reload_test.go remain the authoritative
// deterministic verification.
func TestManualDemoExplicitReload(t *testing.T) {
	// Build a search index with one file and one match (one-stop
	// index). The file has 30 lines.
	lines := make([]filebuffer.Line, 30)
	for i := 0; i < 30; i++ {
		lines[i] = ml(i+1, "line-"+padNum(i))
	}
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "line-00\n", 1, subSpec{"line-00", 0, 7}),
	)

	loader := newGatedFailingLoader()
	loader.setBuffer("src/a.go", makeBuf(lines, 30, 3))

	// Setup: A is the startup file.
	m, loadA := setupBrowseGatedFailing(t, idx, loader)

	// Step 1: A loads successfully. The panel shows A's content.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)
	view := viewContent(m)
	if !strings.Contains(view, "line-00") {
		t.Fatalf("Step 1: panel should show line-00: %q", view)
	}
	fmt.Printf("Step 1 (open file): panel shows line-00 (correct)\n")

	// Step 2: Scroll down several rows.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, keyPress(tea.KeyDown))
	}
	offsetBefore := m.ViewportOffset()
	if offsetBefore == 0 {
		t.Fatalf("Step 2: after 5 down, ViewportOffset = 0, want > 0")
	}
	fmt.Printf("Step 2 (scroll): ViewportOffset = %d (correct)\n", offsetBefore)

	// Step 3: Append lines externally (the loader would return new
	// content). The display remains unchanged (cache stable until r).
	callsBefore := loader.callCount("src/a.go")
	newLines := make([]filebuffer.Line, 35)
	for i := 0; i < 30; i++ {
		newLines[i] = lines[i]
	}
	for i := 30; i < 35; i++ {
		newLines[i] = ml(i+1, "appended-"+padNum(i))
	}
	loader.setBuffer("src/a.go", makeBuf(newLines, 35, 3))
	view = viewContent(m)
	if !strings.Contains(view, "line-05") {
		t.Fatalf("Step 3: panel should still show old content (cache stable): %q", view)
	}
	if strings.Contains(view, "appended") {
		t.Fatalf("Step 3: panel should not show appended content (no reload): %q", view)
	}
	if got := loader.callCount("src/a.go"); got != callsBefore {
		t.Fatalf("Step 3: loader called %d times without r, want %d (no reload)", got, callsBefore)
	}
	fmt.Printf("Step 3 (append externally): display unchanged (cache stable, correct)\n")

	// Step 4: Press r and show new content at the same top position
	// (anchor preserved).
	loader.rearm("src/a.go")
	m, cmd := update(t, m, keyPress('r'))
	if !m.IsLoading() {
		t.Fatalf("Step 4: after r, IsLoading should be true")
	}
	loader.release("src/a.go")
	lc = drainLoadFromCmd(t, cmd)
	m = deliverCompletion(t, m, lc)
	view = viewContent(m)
	if !strings.Contains(view, "line-") {
		t.Fatalf("Step 4: after r, panel should show content: %q", view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("Step 4: after r, panel should not show Loading: %q", view)
	}
	// The anchor should be preserved (same top position).
	if got := m.ViewportOffset(); got != offsetBefore {
		t.Fatalf("Step 4: after r, ViewportOffset = %d, want %d (anchor preserved)", got, offsetBefore)
	}
	fmt.Printf("Step 4 (press r): new content at same top position (anchor preserved, correct)\n")

	// Step 5: Delete the file (make the loader fail) and press r to
	// show "(unreadable)".
	loader.setFailing("src/a.go", "no such file or directory")
	loader.rearm("src/a.go")
	m, cmd = update(t, m, keyPress('r'))
	loader.release("src/a.go")
	lc = drainLoadFromCmd(t, cmd)
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() {
		t.Fatalf("Step 5: overlay should be open after failed reload")
	}
	// Dismiss the overlay to see the panel.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	view = viewContent(m)
	if !strings.Contains(view, "(unreadable)") {
		t.Fatalf("Step 5: panel should show (unreadable) after failed reload: %q", view)
	}
	fmt.Printf("Step 5 (delete + r): panel shows (unreadable) (correct)\n")

	// Step 6: Restore the file (make the loader succeed again) and
	// press r to show content again.
	loader.clearFailing("src/a.go")
	loader.setBuffer("src/a.go", makeBuf(newLines, 35, 3))
	loader.rearm("src/a.go")
	// Dismiss any overlay first if still open.
	if m.OverlayOpen() {
		m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	}
	m, cmd = update(t, m, keyPress('r'))
	loader.release("src/a.go")
	lc = drainLoadFromCmd(t, cmd)
	m = deliverCompletion(t, m, lc)
	// A successful reload clears readFailed but does not close a
	// prior read-failure overlay (Issue #26: the overlay remains
	// displayed until dismissed). Dismiss it to see the content.
	if m.OverlayOpen() {
		m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	}
	view = viewContent(m)
	if strings.Contains(view, "(unreadable)") {
		t.Fatalf("Step 6: panel should not show (unreadable) after restore: %q", view)
	}
	if !strings.Contains(view, "line-") {
		t.Fatalf("Step 6: panel should show content after restore: %q", view)
	}
	fmt.Printf("Step 6 (restore + r): panel shows content again (correct)\n")

	// Step 7: Reload a single-match search via r (one-stop index
	// route). The index has one stop; r works without navigation.
	loader.setBuffer("src/a.go", makeBuf([]filebuffer.Line{ml(1, "reloaded-one-stop")}, 1, 3))
	loader.rearm("src/a.go")
	m, cmd = update(t, m, keyPress('r'))
	loader.release("src/a.go")
	lc = drainLoadFromCmd(t, cmd)
	m = deliverCompletion(t, m, lc)
	view = viewContent(m)
	if !strings.Contains(view, "reloaded-one-stop") {
		t.Fatalf("Step 7: panel should show reloaded-one-stop: %q", view)
	}
	fmt.Printf("Step 7 (reload one-stop): panel shows reloaded content (correct)\n")

	fmt.Printf("\nManual demonstration passed: explicit reload behavior matches the Issue #27 contracts.\n")
}
