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

// TestManualDemoTooSmall demonstrates the Issue #33 manual verification
// scenario deterministically through the model:
//  1. Open help.
//  2. Scroll help.
//  3. Shrink terminal to 15x2.
//  4. Show "Terminal too small".
//  5. Enlarge terminal.
//  6. Show help reappearing at the same scroll position.
//  7. Shrink again.
//  8. Press q to exit.
func TestManualDemoTooSmall(t *testing.T) {
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
	if m.State() != app.StateBrowse {
		t.Fatalf("setup: state = %v, want browse", m.State())
	}

	// 1. Open help.
	m, _ = update(t, m, keyPress('h'))
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("step 1: help not open")
	}
	t.Logf("step 1 OK: help open")

	// 2. Scroll help to position 3.
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != 3 {
		t.Fatalf("step 2: help scroll = %d, want 3", m.OverlayScroll())
	}
	t.Logf("step 2 OK: help scrolled to position 3")

	// 3. Shrink terminal to 15x2 (below the 20x3 minimum).
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 15, Height: 2})
	if !m.TooSmall() {
		t.Fatalf("step 3: TooSmall = false, want true")
	}
	view := viewContent(m)
	if !strings.Contains(view, "Terminal too small") {
		t.Fatalf("step 3: view does not contain 'Terminal too small':\n%s", view)
	}
	// The help overlay is logically open but not displayed.
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("step 3: help overlay not logically open during too-small")
	}
	if m.OverlayScroll() != 3 {
		t.Fatalf("step 3: help scroll = %d, want 3 (preserved)", m.OverlayScroll())
	}
	t.Logf("step 3 OK: terminal shrunk to 15x2, 'Terminal too small' shown, help logically open at scroll 3")

	// 4. (Shown above) "Terminal too small" is displayed.
	t.Logf("step 4 OK: 'Terminal too small' displayed")

	// 5. Enlarge terminal back to 80x24.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.TooSmall() {
		t.Fatalf("step 5: TooSmall = true, want false")
	}
	t.Logf("step 5 OK: terminal enlarged to 80x24")

	// 6. Show help reappearing at the same scroll position (3).
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("step 6: help not open after recovery")
	}
	if m.OverlayScroll() != 3 {
		t.Fatalf("step 6: help scroll = %d, want 3 (preserved)", m.OverlayScroll())
	}
	view = viewContent(m)
	if strings.Contains(view, "Terminal too small") {
		t.Fatalf("step 6: view still contains 'Terminal too small' after recovery")
	}
	t.Logf("step 6 OK: help reappeared at scroll position 3")

	// 7. Shrink again to 15x2.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 15, Height: 2})
	if !m.TooSmall() {
		t.Fatalf("step 7: TooSmall = false, want true")
	}
	view = viewContent(m)
	if !strings.Contains(view, "Terminal too small") {
		t.Fatalf("step 7: view does not contain 'Terminal too small':\n%s", view)
	}
	t.Logf("step 7 OK: terminal shrunk to 15x2 again, 'Terminal too small' shown")

	// 8. Press q to exit.
	m, cmd = update(t, m, keyPress('q'))
	if cmd == nil {
		t.Fatalf("step 8: q produced no command, want tea.Quit")
	}
	if m.ExitCode() != 0 {
		t.Fatalf("step 8: ExitCode = %d, want 0 (fixed browse status)", m.ExitCode())
	}
	t.Logf("step 8 OK: q exited with fixed browse status 0")
	t.Logf("Manual demonstration passed: too-small screen behavior matches the Issue #33 contracts.")
}
