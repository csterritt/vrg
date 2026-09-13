package app_test

import (
	"os"
	"strings"
	"testing"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/viewport"
)

// --- Zero-width marker indicator tests (Issue #23) ---
//
// These tests verify that zero-width markers participate in the
// Issue #20 hidden-content indicators: a marker entirely hidden left
// produces a gutter `*`, a marker entirely hidden right produces a
// right `*` on the current matched line, and a visible marker
// produces no indicator. The terminator-only $ marker follows the
// same rules as any other marker.

// loadMarkerBufForApp loads a file with the given content and stops,
// returning a buffer with gutter width 3 for app-level tests.
func loadMarkerBufForApp(t *testing.T, dir, path, content string, stops []searchindex.Stop) *filebuffer.Buffer {
	t.Helper()
	full := dir + "/" + path
	if err := os.MkdirAll(dir+"/"+dirOf(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf, err := filebuffer.Load([]byte(full), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	buf.GutterWidth = 3
	return buf
}

// dirOf returns the directory portion of a slash-separated path,
// or "." if there is no slash.
func dirOf(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "."
	}
	return path[:idx]
}

// stopsForPath extracts the stops for the given raw path from the
// index.
func stopsForPath(idx *searchindex.Index, rawPath string) []searchindex.Stop {
	var stops []searchindex.Stop
	for _, s := range idx.Stops() {
		if string(s.RawPath) == rawPath {
			stops = append(stops, s)
		}
	}
	return stops
}

// panLeft pans the model left by the given number of columns using the
// '<' (10 columns) and ',' (1 column) pan keys.
func panLeft(t *testing.T, m app.Model, columns int) app.Model {
	t.Helper()
	for columns >= 10 {
		m, _ = update(t, m, keyPress('<'))
		columns -= 10
	}
	for columns > 0 {
		m, _ = update(t, m, keyPress(','))
		columns--
	}
	return m
}

// TestIndicatorMarkerHiddenLeftStar verifies that a zero-width EOL
// marker entirely hidden left produces a gutter `*` indicator. Line 1
// is "hit" with an EOL marker at cell 3 (extent 4); line 2 is 300 ASCII
// chars (extent 300). The max offset is 299 (from line 2), so panning
// right by 4 hides the marker on line 1 left.
func TestIndicatorMarkerHiddenLeftStar(t *testing.T) {
	line2 := makeLongASCII(300)
	content := "hit\n" + line2 + "\n"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hit\n", 1, subSpec{"", 3, 3}),
	)
	dir := t.TempDir()
	stops := stopsForPath(idx, "src/a.go")
	buf := loadMarkerBufForApp(t, dir, "src/a.go", content, stops)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 4)
	view := viewContent(m)
	if !strings.Contains(view, "1* ") {
		t.Fatalf("view does not contain left '*' (EOL marker hidden left): %q", view)
	}
}

// TestIndicatorMarkerVisibleNoStar verifies that a visible zero-width
// EOL marker produces no hidden-match indicator. The marker at cell 3
// is visible when the offset is 0.
func TestIndicatorMarkerVisibleNoStar(t *testing.T) {
	content := "hit\n"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", content, 1, subSpec{"", 3, 3}),
	)
	dir := t.TempDir()
	stops := stopsForPath(idx, "src/a.go")
	buf := loadMarkerBufForApp(t, dir, "src/a.go", content, stops)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	view := viewContent(m)
	if strings.Contains(view, "1* ") {
		t.Fatalf("view contains left '*' (EOL marker visible, no hidden match): %q", view)
	}
}

// TestIndicatorMarkerHiddenRightStar verifies that a zero-width EOL
// marker entirely hidden right produces a right `*` indicator on the
// current matched line. A 200-char line with the EOL marker at cell
// 200: the reveal pans to show the marker, then pan left to hide it
// right.
func TestIndicatorMarkerHiddenRightStar(t *testing.T) {
	content := makeLongASCII(200) + "\n"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", content, 1, subSpec{"", 200, 200}),
	)
	dir := t.TempDir()
	stops := stopsForPath(idx, "src/a.go")
	buf := loadMarkerBufForApp(t, dir, "src/a.go", content, stops)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	// The reveal pans right to show the marker. Pan left by 2 so the
	// marker (at cell 200) is beyond the right edge of the window.
	m = panLeft(t, m, 2)
	view := viewContent(m)
	if !anyContentRowEndsWith(view, "*") {
		t.Fatalf("view does not contain right '*' (EOL marker hidden right): %q", view)
	}
}

// TestIndicatorTerminatorOnlyMarkerHiddenLeftStar verifies that a
// terminator-only $ marker on "hit\r\n" follows the same indicator
// rules as any other marker. The marker at column 3 is hidden left
// when panned right past it (using a long second line to extend the
// max offset), producing a gutter `*`.
func TestIndicatorTerminatorOnlyMarkerHiddenLeftStar(t *testing.T) {
	line2 := makeLongASCII(300)
	content := "hit\r\n" + line2 + "\n"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hit\r\n", 1, subSpec{"", 4, 4}),
	)
	dir := t.TempDir()
	stops := stopsForPath(idx, "src/a.go")
	buf := loadMarkerBufForApp(t, dir, "src/a.go", content, stops)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 4)
	view := viewContent(m)
	if !strings.Contains(view, "1* ") {
		t.Fatalf("view does not contain left '*' (terminator-only marker hidden left): %q", view)
	}
}

// TestIndicatorMarkerOnlyLineAlwaysVisible verifies that a marker-only
// line (empty line with EOL marker) has max offset 0, so the marker
// is always visible and no `*` indicator appears.
func TestIndicatorMarkerOnlyLineAlwaysVisible(t *testing.T) {
	content := "\n"
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", content, 1, subSpec{"", 0, 0}),
	)
	dir := t.TempDir()
	stops := stopsForPath(idx, "src/a.go")
	buf := loadMarkerBufForApp(t, dir, "src/a.go", content, stops)
	m := setupBrowseIndicatorMode(t, idx, buf, viewport.WrapOff)
	m = panRight(t, m, 5)
	view := viewContent(m)
	if strings.Contains(view, "1* ") {
		t.Fatalf("view contains left '*' (marker-only line always visible, max offset 0): %q", view)
	}
}
