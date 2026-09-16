package app_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// --- Issue #41: complete scrollable overlay rows ---
//
// Non-help overlays keep every wrapped row in the scrollable set:
// there is no head-plus-ellipsis-plus-tail compression, and
// overlayScroll clamps to [0, max(0, rows-maxVisible)] in both the
// key handler and the render path, so every row of an arbitrarily
// long diagnostic is reachable by up/down.

// overlayTestRows builds a diagnostic of n short lines (each well
// under the 60-cell interior width of an 80-column terminal, so each
// is exactly one wrapped row), with FIRST-MARKER and LAST-MARKER as
// the first and last lines.
func overlayTestRows(n int) []string {
	rows := make([]string, n)
	rows[0] = "FIRST-MARKER"
	for i := 1; i < n-1; i++ {
		rows[i] = fmt.Sprintf("diag-row-%03d", i)
	}
	rows[n-1] = "LAST-MARKER"
	return rows
}

// overlayDiagWithHead builds a diagnostic whose first line is head,
// followed by n-1 filler rows each exactly one wrapped row at 80
// columns. Existing scroll-preservation and routing tests use it so
// their scroll positions stay within the complete-set clamp.
func overlayDiagWithHead(head string, n int) string {
	rows := overlayTestRows(n)
	rows[0] = head
	return strings.Join(rows, "\n")
}

// setupOverlayFullScroll creates a browse model at the given terminal
// size with a non-fatal error overlay open on the given diagnostic,
// under the no-style theme so the rendered view is exactly the
// visible window of wrapped overlay rows (Issue #41).
func setupOverlayFullScroll(t *testing.T, diag string, width, height int) app.Model {
	t.Helper()
	idx := buildIndex(t, "/work",
		textBegin("a.go"),
		textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		endRecord("a.go", nil),
		summaryRecord(),
	)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return makeBuf(nil, 0, 3), nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	m, cmd := update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  diag,
	})
	if cmd != nil {
		execCmd(t, cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open, want open error overlay")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	return m
}

// TestOverlayCompleteRowsScrollable verifies that the scrollable row
// set of a non-help overlay is the complete wrapped diagnostic: the
// rendered window slides row by row over every row, no ellipsis row
// is ever injected, and overlayScroll clamps to [0, rows-maxVisible]
// at both ends. A bounded down traversal reaches the final row and an
// up traversal returns to the first.
func TestOverlayCompleteRowsScrollable(t *testing.T) {
	rows := overlayTestRows(30) // 30 rows; at 80x24 maxVisible = 20 → max scroll 10
	m := setupOverlayFullScroll(t, strings.Join(rows, "\n"), 80, 24)

	// The scrollable set starts with the complete head — not a
	// compressed head+ellipsis+tail frame.
	if got, want := viewContent(m), strings.Join(rows[:20], "\n"); got != want {
		t.Fatalf("initial overlay view = %q, want rows[0:20] %q", got, want)
	}
	// Scroll down through the complete set one row at a time.
	var cmd tea.Cmd
	for i := 1; i <= 10; i++ {
		m, cmd = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
		if cmd != nil {
			t.Fatalf("down %d produced a command: %v", i, cmd)
		}
		if m.OverlayScroll() != i {
			t.Fatalf("after %d downs, OverlayScroll = %d, want %d", i, m.OverlayScroll(), i)
		}
		want := strings.Join(rows[i:i+20], "\n")
		if got := viewContent(m); got != want {
			t.Fatalf("at scroll %d, overlay view = %q, want rows[%d:%d] %q", i, got, i, i+20, want)
		}
		if strings.Contains(viewContent(m), "…") {
			t.Fatalf("at scroll %d, overlay view contains an ellipsis row", i)
		}
	}
	// The key handler clamps at rows-maxVisible: extra downs are no-ops.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != 10 {
		t.Fatalf("after extra downs, OverlayScroll = %d, want clamped 10", m.OverlayScroll())
	}
	if got, want := viewContent(m), strings.Join(rows[10:], "\n"); got != want {
		t.Fatalf("bottom overlay view = %q, want rows[10:] %q", got, want)
	}
	// An up traversal returns to the first row; up clamps at 0.
	for i := 9; i >= 0; i-- {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
		if m.OverlayScroll() != i {
			t.Fatalf("during up traversal, OverlayScroll = %d, want %d", m.OverlayScroll(), i)
		}
	}
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	}
	if m.OverlayScroll() != 0 {
		t.Fatalf("after extra ups, OverlayScroll = %d, want clamped 0", m.OverlayScroll())
	}
	if got, want := viewContent(m), strings.Join(rows[:20], "\n"); got != want {
		t.Fatalf("top overlay view = %q, want rows[0:20] %q", got, want)
	}
}

// TestOverlayHugeDiagnosticCompleteRows verifies the complete-row
// contract on a ≥ 1 MiB diagnostic shaped like the large-stderr
// fixture: a distinctive head marker, a large body, and a tail
// marker. At 80x24 the interior width is 60 cells, so each 10240-byte
// body line wraps to 171 rows (ceil(10240/60)); with the two marker
// lines the complete set is 1+103*171+1 = 17615 rows and the maximum
// scroll is rows-maxVisible = 17595. The head renders first, the tail
// only after scrolling to the bottom, and no ellipsis row ever
// substitutes for content.
func TestOverlayHugeDiagnosticCompleteRows(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("HEADMARKER\n")
	for i := 0; i < 103; i++ {
		sb.WriteString(strings.Repeat("E", 10240))
		sb.WriteString("\n")
	}
	sb.WriteString("TAILMARKER")
	diag := sb.String()
	if len(diag) < 1<<20 {
		t.Fatalf("fixture is %d bytes, want ≥ 1 MiB", len(diag))
	}
	m := setupOverlayFullScroll(t, diag, 80, 24)

	const maxScroll = 17615 - 20
	view := viewContent(m)
	if !strings.Contains(view, "HEADMARKER") {
		t.Fatalf("initial overlay view lacks HEADMARKER")
	}
	if strings.Contains(view, "TAILMARKER") {
		t.Fatalf("initial overlay view shows TAILMARKER (head/tail compression)")
	}
	if strings.Contains(view, "…") {
		t.Fatalf("initial overlay view contains an ellipsis row")
	}
	// A bounded down traversal reaches the final row: the tail is a
	// real row at the end of the complete set, not a render-time
	// shortcut.
	for i := 0; i < maxScroll; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != maxScroll {
		t.Fatalf("after %d downs, OverlayScroll = %d, want %d (rows-maxVisible)", maxScroll, m.OverlayScroll(), maxScroll)
	}
	// Extra downs are clamped by the key handler at rows-maxVisible.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != maxScroll {
		t.Fatalf("past the bottom, OverlayScroll = %d, want clamped %d", m.OverlayScroll(), maxScroll)
	}
	view = viewContent(m)
	if !strings.Contains(view, "TAILMARKER") {
		t.Fatalf("bottom overlay view lacks TAILMARKER")
	}
	if strings.Contains(view, "…") {
		t.Fatalf("bottom overlay view contains an ellipsis row")
	}
	// An up traversal returns to the first row.
	for i := 0; i < maxScroll; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	}
	if m.OverlayScroll() != 0 {
		t.Fatalf("after full up traversal, OverlayScroll = %d, want 0", m.OverlayScroll())
	}
	view = viewContent(m)
	if !strings.Contains(view, "HEADMARKER") {
		t.Fatalf("top overlay view lacks HEADMARKER after return traversal")
	}
}

// TestOverlayRenderClampsStaleScroll verifies the render-path clamp:
// when a resize shrinks the bound below the current scroll position,
// rendering clips to the last page rather than running off the end of
// the row set, and the next scroll key re-clamps the position into
// [0, rows-maxVisible].
func TestOverlayRenderClampsStaleScroll(t *testing.T) {
	rows := overlayTestRows(30)
	m := setupOverlayFullScroll(t, strings.Join(rows, "\n"), 80, 24)
	// Scroll to the bottom at 80x24: max scroll = 30-20 = 10.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != 10 {
		t.Fatalf("OverlayScroll = %d, want 10", m.OverlayScroll())
	}
	// Grow the terminal: at 80x40 maxVisible is 36, covering all 30
	// rows. The render clamps the stale scroll so every row shows.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 40})
	if got, want := viewContent(m), strings.Join(rows, "\n"); got != want {
		t.Fatalf("after resize, overlay view = %q, want all rows %q", got, want)
	}
	// The next scroll key re-clamps the position into [0, 0].
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.OverlayScroll() != 0 {
		t.Fatalf("after up, OverlayScroll = %d, want clamped 0", m.OverlayScroll())
	}
}

// TestOverlayAppendExtendsScrollableSet verifies that an error
// appended while an overlay is open extends the complete scrollable
// row set without moving the reader's position: the appended row is
// reachable by scrolling and no ellipsis row is injected (Issue #41,
// story 81).
func TestOverlayAppendExtendsScrollableSet(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "z\n", 1, subSpec{"z", 0, 1}),
	)
	loader := newGatedFailingLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	loader.setFailing("src/a.go", strings.Join(overlayTestRows(30), "\n"))
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader.load),
		app.WithPopupDuration(0),
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}

	// Startup file A fails to load → read-failure overlay opens.
	loader.release("src/a.go")
	lc := <-startLoadAsync(cmd)
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayError {
		t.Fatalf("error overlay not open after first failure")
	}
	// A middle row — one that head/tail compression would elide — is
	// part of the scrollable set from the start.
	if view := viewContent(m); !strings.Contains(view, "diag-row-015") {
		t.Fatalf("overlay view lacks middle row 'diag-row-015': %q", view)
	}

	// Scroll the overlay to position P.
	const scrollP = 2
	for i := 0; i < scrollP; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != scrollP {
		t.Fatalf("overlay scroll = %d, want %d", m.OverlayScroll(), scrollP)
	}

	// Press r to retry through the open read-failure overlay (Issue
	// #27). The retry fails with a new diagnostic that is appended.
	loader.setFailing("src/a.go", "APPENDED-MARKER")
	m, reloadCmd := update(t, m, keyPress('r'))
	loader.rearm("src/a.go")
	loadRetry := startLoadAsync(reloadCmd)
	loader.release("src/a.go")
	lc2 := <-loadRetry
	m = deliverCompletion(t, m, lc2)

	// The overlay stays open and the reader's position is preserved.
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayError {
		t.Fatalf("error overlay not open after appended failure")
	}
	if m.OverlayScroll() != scrollP {
		t.Fatalf("overlay scroll = %d after append, want %d (preserved)", m.OverlayScroll(), scrollP)
	}
	// The appended row extends the scrollable set: 31 rows → max
	// scroll 11, and the appended marker is the final row.
	for i := scrollP; i < 11; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != 11 {
		t.Fatalf("overlay scroll = %d at bottom, want 11 (rows-maxVisible)", m.OverlayScroll())
	}
	view := viewContent(m)
	if lastNL := strings.LastIndex(view, "\n"); lastNL < 0 || view[lastNL+1:] != "src/a.go: APPENDED-MARKER" {
		t.Fatalf("bottom overlay view's last row is not the appended marker: %q", view)
	}
	if strings.Contains(view, "…") {
		t.Fatalf("overlay view contains an ellipsis row")
	}
}
