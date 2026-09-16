package app_test

import (
	"os"
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// --- Standalone combining cluster fallback cell (Issue #43) ---
//
// The recorded fallback representation (Notes/decisions/
// 043-combining-cluster-fallback-cell.md) displays a standalone
// zero-width cluster as U+25CC ◌ followed by the cluster's original
// combining-mark bytes in exactly one terminal cell. These composed
// tests render through the app View() and assert the emitted cell
// layout: the recorded display bytes paint in one cell, the following
// cluster renders in the next cell with no overlap, and wrapping,
// clipping, and panning count the fallback like any other cell.

// loadBufWithStops loads a real buffer through the production
// filebuffer.Load path for the given file content and search stops, so
// the composed tests exercise the Issue #43 materialization itself
// rather than a hand-built line.
func loadBufWithStops(t *testing.T, idx *searchindex.Index, path, content string) *filebuffer.Buffer {
	t.Helper()
	dir := t.TempDir()
	p := dir + "/" + path
	if err := os.MkdirAll(dir+"/src", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var stops []searchindex.Stop
	for _, s := range idx.Stops() {
		if string(s.RawPath) == path {
			stops = append(stops, s)
		}
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	buf.GutterWidth = 3
	return buf
}

// TestRenderStandaloneClusterPaintsFallbackBytes verifies that a line
// beginning with a standalone combining mark paints exactly one cell
// showing the recorded fallback bytes ◌́, with the following cluster
// rendered in the next cell — no overlap and no shared cell (Issue
// #43). A match covering the mark highlights exactly the fallback cell
// and nothing adjacent, so the styled span is exactly "◌́" and "x" is
// emitted unstyled.
func TestRenderStandaloneClusterPaintsFallbackBytes(t *testing.T) {
	raw := "́x\n"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"́", 0, 2}),
	)
	buf := loadBufWithStops(t, idx, "src/a.go", raw)
	m := setupBrowse(t, idx, buf)
	view := viewContent(m)
	// Line 1 is the current line: the match covering the standalone
	// cluster styles exactly the fallback cell "◌́"; the following "x"
	// cluster must be emitted unstyled in the next cell.
	want := "\x1b[30;47;4m◌́\x1b[37;40mx"
	if !strings.Contains(view, want) {
		t.Fatalf("view does not contain %q (fallback cell paints ◌́ for one cell, x follows unstyled): %q", want, view)
	}
	// The emitted text places the fallback bytes immediately before
	// the following cluster — one cell each, no shared cell.
	if !strings.Contains(stripANSI(view), "◌́x") {
		t.Fatalf("view does not place ◌́ and x in consecutive cells: %q", view)
	}
}

// TestStandaloneFallbackCountsForWrap verifies that wrapping counts the
// fallback cell like any other cell: a line of a standalone mark plus
// 100 x's wraps with the ◌́ cell occupying one cell of the first row,
// so the first row holds ◌́ plus 65 x's (wrap-mode text width 66), not
// 66 x's (Issue #43).
func TestStandaloneFallbackCountsForWrap(t *testing.T) {
	raw := "́" + strings.Repeat("x", 100) + "\n"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"́", 0, 2}),
	)
	buf := loadBufWithStops(t, idx, "src/a.go", raw)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOn)
	view := viewContent(m)
	rows := contentRows(view)
	if len(rows) < 2 {
		t.Fatalf("view has %d content rows, want the line wrapped to at least 2: %q", len(rows), view)
	}
	if !strings.Contains(rows[0], "◌́"+strings.Repeat("x", 65)) {
		t.Fatalf("first row does not contain ◌́ plus 65 x's (fallback must count as one wrap cell): %q", rows[0])
	}
	if !strings.Contains(rows[1], strings.Repeat("x", 35)) {
		t.Fatalf("second row does not contain the remaining 35 x's: %q", rows[1])
	}
}

// TestStandaloneFallbackCountsForPanAndClip verifies that horizontal
// panning and clipping count the fallback cell: panning right by one
// cell hides the fallback cluster entirely, the following "x" moves to
// the first text cell, and the match is reported entirely hidden left
// by the gutter `*` indicator (Issue #43, run-off-edge mode).
func TestStandaloneFallbackCountsForPanAndClip(t *testing.T) {
	raw := "́x\n"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", raw, 1, subSpec{"́", 0, 2}),
	)
	buf := loadBufWithStops(t, idx, "src/a.go", raw)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 1)
	view := viewContent(m)
	// The fallback cluster occupied cell [0, 1); panning to offset 1
	// hides it entirely and clips "x" into the first text cell.
	if !strings.Contains(view, "1* x") {
		t.Fatalf("view does not show '1* x' (x must clip into cell 0 after the hidden fallback cell): %q", view)
	}
	if strings.Contains(stripANSI(view), "◌") {
		t.Fatalf("view still shows the fallback cell after it is hidden left: %q", view)
	}
}
