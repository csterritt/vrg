//go:build manual_demo

package app_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
)

// TestManualDemoReadFailure demonstrates the Issue #26 manual case
// as a gated model test with an injected failing loader:
//   - File 1 (src/a.go) loads successfully.
//   - File 2 (src/b.go) fails to load (unreadable).
//   - Startup shows file 1.
//   - n into file 2 shows the overlay and "(unreadable)".
//   - Esc dismisses the overlay.
//   - A same-file n step opens no new overlay.
//   - p p back into file 2 shows the overlay with "Loading…" (re-entry).
//   - Dismissal, then q exits 0 with the failures listed in diagnostics.
//
// This mirrors the shell-based manual route but uses the injected
// failing loader so it runs without filesystem permissions. The
// injected-loader model tests in read_failure_test.go and
// reentry_test.go remain the authoritative deterministic
// verification.
func TestManualDemoReadFailure(t *testing.T) {
	// Build a search index with two files. A has one match; B has
	// two matches (two stops) so a same-file n step is possible.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
		textMatch("src/b.go", "y\n", 2, subSpec{"y", 0, 1}),
	)

	bufA := makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3)

	loader := newGatedFailingLoader()
	loader.setBuffer("src/a.go", bufA)
	loader.setFailing("src/b.go", "permission denied")

	// Setup: A is the startup file (first in path order).
	m, loadA := setupBrowseGatedFailing(t, idx, loader)

	// Step 1: A loads successfully. The panel shows A's content.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)
	view := viewContent(m)
	if !strings.Contains(view, "content-a") {
		t.Fatalf("Step 1: panel should show A's content: %q", view)
	}
	fmt.Printf("Step 1 (startup): file 1 shows content (correct)\n")

	// Step 2: n into file 2. B fails. The overlay opens and the
	// panel shows "(unreadable)".
	loader.release("src/b.go")
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	loadB := startLoadAsync(cmd)
	lc = <-loadB
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() {
		t.Fatalf("Step 2: overlay should be open after B fails")
	}
	if m.OverlayFatal() {
		t.Fatalf("Step 2: overlay should be non-fatal")
	}
	// Dismiss the overlay to see the panel.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	view = viewContent(m)
	if !strings.Contains(view, "(unreadable)") {
		t.Fatalf("Step 2: panel should show (unreadable) after B fails: %q", view)
	}
	fmt.Printf("Step 2 (n into file 2): overlay opens, panel shows (unreadable) (correct)\n")

	// Step 3: Esc already dismissed the overlay. A same-file n
	// step opens no new overlay.
	callsBefore := loader.callCount("src/b.go")
	m, _ = update(t, m, keyPress('n'))
	if m.OverlayOpen() {
		t.Fatalf("Step 3: same-file n should not open a new overlay")
	}
	if got := loader.callCount("src/b.go"); got != callsBefore {
		t.Fatalf("Step 3: same-file n should not retry: calls %d -> %d", callsBefore, got)
	}
	fmt.Printf("Step 3 (same-file n): no new overlay, no retry (correct)\n")

	// Step 4: p p p back into file 2 (re-entry from a different
	// file). After the same-file n we're at B's second match. Three
	// p presses navigate: B2→B1 (same-file), B1→A (cross-file,
	// cached, no overlay), A→B2 (cross-file, re-entry). The overlay
	// reopens with "Loading…" (the retry is in flight).
	loader.rearm("src/b.go")
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/b.go") // B1, same-file
	if m.OverlayOpen() {
		t.Fatalf("Step 4a: same-file p should not open an overlay")
	}
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go") // A, cross-file, cached
	if m.OverlayOpen() {
		t.Fatalf("Step 4b: re-entering cached A should not open an overlay")
	}
	// p again into B (re-entry).
	m, cmd = update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/b.go")
	loadB2Ch := startLoadAsync(cmd)
	_ = loadB2Ch
	if !m.OverlayOpen() {
		t.Fatalf("Step 4: overlay should reopen on re-entry into B")
	}
	if m.ReadFailed() {
		t.Fatalf("Step 4: panel should show Loading on re-entry, not (unreadable)")
	}
	if !m.IsLoading() {
		t.Fatalf("Step 4: panel should show Loading on re-entry")
	}
	fmt.Printf("Step 4 (p p p back into file 2): overlay reopens with Loading (correct)\n")

	// Step 5: Dismiss the overlay, then q exits 0 with the
	// failures listed in diagnostics.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	diags := m.Diagnostics()
	if len(diags) == 0 {
		t.Fatalf("Step 5: diagnostics should contain the failure")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d, "permission denied") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Step 5: diagnostics should contain 'permission denied': %v", diags)
	}
	// The exit status is the fixed search-derived status (0 for a
	// successful search with results).
	if got := m.ExitCode(); got != 0 {
		t.Fatalf("Step 5: exit code should be 0 (fixed search status), got %d", got)
	}
	fmt.Printf("Step 5 (q): exit 0, failures listed in diagnostics (correct)\n")

	fmt.Printf("\nManual demonstration passed: read-failure behavior matches the Issue #26 contracts.\n")
}
