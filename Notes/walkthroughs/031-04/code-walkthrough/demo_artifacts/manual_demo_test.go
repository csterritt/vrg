//go:build manual_demo

package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// TestManualDemoHelpOverlay demonstrates the Issue #31 manual
// verification scenarios deterministically through the model: ?
// opens the bordered help, down scrolls, n does nothing to the file
// behind, Esc closes, shrinking to 25x8 shows clipped-but-present
// help, enlarging restores the normal layout, and opening help from
// the no-results screen then closing back to it with q exits 1.
func TestManualDemoHelpOverlay(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
	)
	buf := makeBuf(nil, 0, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}

	// 1. ? opens the bordered help overlay.
	m, _ = update(t, m, keyPress('?'))
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("step 1: help not open after ?")
	}
	view := viewContent(m)
	if !strings.Contains(view, "\u250c") {
		t.Fatalf("step 1: help overlay has no border: %q", view)
	}
	t.Logf("step 1 OK: ? opened bordered help (border present, %d chars)", len(view))

	// 2. down scrolls the help content.
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if !m.OverlayOpen() {
		t.Fatalf("step 2: down closed the overlay")
	}
	t.Logf("step 2 OK: down pressed (overlay still open)")

	// 3. n does nothing to the file behind.
	m, _ = update(t, m, keyPress('n'))
	if !m.OverlayOpen() {
		t.Fatalf("step 3: n closed the overlay, want still open")
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("step 3: n changed overlay kind")
	}
	t.Logf("step 3 OK: n ignored (overlay still open, kind=help)")

	// 4. Esc closes the help overlay, returning to browse.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("step 4: Esc did not close help")
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("step 4: state = %v, want browse", m.State())
	}
	t.Logf("step 4 OK: Esc closed help, returned to browse (state=%v)", m.State())

	// 5. Shrinking to 25x8 shows clipped-but-present help.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 25, Height: 8})
	m, _ = update(t, m, keyPress('?'))
	if !m.OverlayOpen() {
		t.Fatalf("step 5: help not open at 25x8")
	}
	view = viewContent(m)
	if view == "" {
		t.Fatalf("step 5: help view empty at 25x8")
	}
	for _, line := range strings.Split(view, "\n") {
		if w := helpVisibleWidth(line); w > 25 {
			t.Fatalf("step 5: line %d cells wide exceeds 25: %q", w, line)
		}
	}
	t.Logf("step 5 OK: 25x8 help clipped-but-present (%d lines, all <= 25 cells)", len(strings.Split(view, "\n")))

	// 6. Enlarging to 80x24 restores the normal layout.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if !m.OverlayOpen() {
		t.Fatalf("step 6: help closed after growth")
	}
	view = viewContent(m)
	if !strings.Contains(view, "\u250c") {
		t.Fatalf("step 6: no border after growth")
	}
	t.Logf("step 6 OK: 80x24 restored border (%d chars)", len(view))

	// Close help before the no-results demo.
	m, _ = update(t, m, keyPress('q'))

	// 7. Opening help from no-results and closing back to it with q exiting 1.
	noResultsIdx := buildIndex(t, "/work", summaryRecord())
	nr := app.New([]string{"--json", "--", "foo", "."}, "/work")
	nr, _ = update(t, nr, tea.WindowSizeMsg{Width: 80, Height: 24})
	nr, _ = update(t, nr, app.SearchCompleteMsg{Files: noResultsIdx.Files(), Lines: noResultsIdx.Len(), Index: noResultsIdx})
	if nr.State() != app.StateNoResults {
		t.Fatalf("step 7: state = %v, want no-results", nr.State())
	}
	nr, _ = update(t, nr, keyPress('?'))
	if !nr.OverlayOpen() || nr.OverlayKind() != app.OverlayHelp {
		t.Fatalf("step 7: help not open from no-results")
	}
	t.Logf("step 7a OK: ? opened help from no-results")
	nr, _ = update(t, nr, keyPress('q'))
	if nr.OverlayOpen() {
		t.Fatalf("step 7: help still open after q")
	}
	if nr.State() != app.StateNoResults {
		t.Fatalf("step 7: state = %v, want no-results", nr.State())
	}
	t.Logf("step 7b OK: q closed help, back to no-results")
	nr, cmd = update(t, nr, keyPress('q'))
	assertQuit(t, cmd)
	if nr.ExitCode() != 1 {
		t.Fatalf("step 7: exit code = %d, want 1", nr.ExitCode())
	}
	t.Logf("step 7c OK: q on no-results exits 1")
	t.Logf("Manual demonstration passed: help overlay behavior matches the Issue #31 contracts.")
}
