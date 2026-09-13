package filebuffer_test

import (
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// --- Zero-width match markers (Issue #23) ---
//
// These tests cover the Issue #23 contracts: a zero-width submatch
// renders as one inverse-video cell at its mapped display location,
// marking an existing cell without shifting text; at end of line it
// extends the effective line width by one cell; a position inside a
// cluster maps to the cluster start; a terminator-only match on LF or
// CRLF produces a single marker cell at the display end-of-line
// column; an empty matched line has width one; and the marker
// participates in horizontal extent (via the Clusters slice).

// TestMarkerAtBOL verifies that a zero-width match at the beginning of
// the line (byte 0) produces a one-cell marker highlight at display
// cell 0. The marker marks the existing cell without shifting text, so
// the display text is unchanged.
func TestMarkerAtBOL(t *testing.T) {
	dir := t.TempDir()
	content := "hello\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("", 0, 0)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if !hasHighlight(line.Highlights, 0, 1) {
		t.Fatalf("Highlights = %v, want a [0, 1) marker at BOL", line.Highlights)
	}
	if line.Display != "hello" {
		t.Fatalf("Display = %q, want %q (marker at BOL does not shift text)", line.Display, "hello")
	}
}

// TestMarkerAtEOL verifies that a zero-width match at the end of the
// line produces a one-cell marker at the EOL display column. The
// marker extends the effective line width by one cell: the Display
// includes a trailing space and the Clusters include an extra 1-cell
// cluster.
func TestMarkerAtEOL(t *testing.T) {
	dir := t.TempDir()
	content := "hit\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Zero-width match at byte 3 (the \n terminator, maps to EOL column 3).
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("", 3, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if !hasHighlight(line.Highlights, 3, 4) {
		t.Fatalf("Highlights = %v, want a [3, 4) marker at EOL", line.Highlights)
	}
	if line.Display != "hit " {
		t.Fatalf("Display = %q, want %q (EOL marker extends display by one space)", line.Display, "hit ")
	}
	if clusterWidth(line.Clusters) != 4 {
		t.Fatalf("Cluster width = %d, want 4 (3 content + 1 marker cell)", clusterWidth(line.Clusters))
	}
}

// TestMarkerOnEmptyLine verifies that a zero-width match on an empty
// line produces a marker at cell 0 and the line has width one. The
// display is a single space and the Clusters have one 1-cell cluster.
func TestMarkerOnEmptyLine(t *testing.T) {
	dir := t.TempDir()
	content := "\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Zero-width match at byte 0 (the \n terminator of an empty line).
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("", 0, 0)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if !hasHighlight(line.Highlights, 0, 1) {
		t.Fatalf("Highlights = %v, want a [0, 1) marker on empty line", line.Highlights)
	}
	if line.Display != " " {
		t.Fatalf("Display = %q, want %q (empty line with marker has one space)", line.Display, " ")
	}
	if clusterWidth(line.Clusters) != 1 {
		t.Fatalf("Cluster width = %d, want 1 (marker-only empty line has width 1)", clusterWidth(line.Clusters))
	}
}

// TestMarkerOnCRLFTerminatorOnly verifies that a terminator-only $
// match on "hit\r\n" produces a single marker cell at display column 3.
// The \n byte (byte 4) maps to the EOL display column 3. The marker is
// an ordinary end-of-line marker with no special cases.
func TestMarkerOnCRLFTerminatorOnly(t *testing.T) {
	dir := t.TempDir()
	content := "hit\r\n"
	p := writeFile(t, dir, "crlf.txt", []byte(content))
	// Zero-width match at byte 4 (the \n of CRLF, maps to EOL column 3).
	stops := []searchindex.Stop{
		mkStop("crlf.txt", 1, content, sm("", 4, 4)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if !hasHighlight(line.Highlights, 3, 4) {
		t.Fatalf("Highlights = %v, want a [3, 4) marker at column 3 for CRLF terminator-only", line.Highlights)
	}
	if line.Display != "hit " {
		t.Fatalf("Display = %q, want %q (EOL marker extends display by one space)", line.Display, "hit ")
	}
	if clusterWidth(line.Clusters) != 4 {
		t.Fatalf("Cluster width = %d, want 4 (3 content + 1 marker)", clusterWidth(line.Clusters))
	}
}

// TestMarkerOnLFTerminatorOnly verifies that a terminator-only $ match
// on "hit\n" produces a single marker cell at display column 3. The \n
// byte (byte 3) maps to the EOL display column 3.
func TestMarkerOnLFTerminatorOnly(t *testing.T) {
	dir := t.TempDir()
	content := "hit\n"
	p := writeFile(t, dir, "lf.txt", []byte(content))
	// Zero-width match at byte 3 (the \n, maps to EOL column 3).
	stops := []searchindex.Stop{
		mkStop("lf.txt", 1, content, sm("", 3, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if !hasHighlight(line.Highlights, 3, 4) {
		t.Fatalf("Highlights = %v, want a [3, 4) marker at column 3 for LF terminator-only", line.Highlights)
	}
	if line.Display != "hit " {
		t.Fatalf("Display = %q, want %q (EOL marker extends display by one space)", line.Display, "hit ")
	}
}

// TestMarkerInsideWideCluster verifies that a zero-width position
// inside a wide cluster maps to the cluster start. The marker is
// placed at the cluster start cell and the wide glyph is not split.
func TestMarkerInsideWideCluster(t *testing.T) {
	dir := t.TempDir()
	// "a中b\n": 中 is a wide cluster (2 cells) at display cells [1, 3).
	// A zero-width match at byte 2 (inside the 中 cluster) maps to
	// the cluster start (cell 1).
	content := "a中b\n"
	p := writeFile(t, dir, "wide.txt", []byte(content))
	// Zero-width match at byte 2 (second byte of 中, inside the cluster).
	stops := []searchindex.Stop{
		mkStop("wide.txt", 1, content, sm("", 2, 2)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if !hasHighlight(line.Highlights, 1, 2) {
		t.Fatalf("Highlights = %v, want a [1, 2) marker at cluster start (wide glyph not split)", line.Highlights)
	}
	// The display should not be extended (marker is inside the line,
	// not at EOL).
	if line.Display != "a中b" {
		t.Fatalf("Display = %q, want %q (marker inside cluster does not extend display)", line.Display, "a中b")
	}
}

// TestMarkerInsideCombiningCluster verifies that a zero-width position
// inside a combining cluster (e + combining mark) maps to the cluster
// start. The marker is placed at the base character's cell.
func TestMarkerInsideCombiningCluster(t *testing.T) {
	dir := t.TempDir()
	// "e\u0301\n": é as base + combining, 1 cell at display cell [0, 1).
	// A zero-width match at byte 1 (the combining mark) maps to the
	// cluster start (cell 0).
	content := "e\u0301\n"
	p := writeFile(t, dir, "combining.txt", []byte(content))
	// Zero-width match at byte 1 (first byte of combining mark).
	stops := []searchindex.Stop{
		mkStop("combining.txt", 1, content, sm("", 1, 1)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if !hasHighlight(line.Highlights, 0, 1) {
		t.Fatalf("Highlights = %v, want a [0, 1) marker at cluster start (combining cluster)", line.Highlights)
	}
}

// TestMarkerAtEOLDoesNotShiftText verifies that a marker at the end of
// line extends the effective width by one cell without shifting the
// existing text. The display text's content portion is unchanged; only
// a trailing space is appended for the marker cell.
func TestMarkerAtEOLDoesNotShiftText(t *testing.T) {
	dir := t.TempDir()
	content := "hello world\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Zero-width match at byte 11 (the \n, maps to EOL column 11).
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("", 11, 11)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	// The content portion "hello world" must be unchanged.
	if !startsWith(line.Display, "hello world") {
		t.Fatalf("Display = %q, want it to start with %q (text not shifted)", line.Display, "hello world")
	}
	// The marker highlight is at [11, 12).
	if !hasHighlight(line.Highlights, 11, 12) {
		t.Fatalf("Highlights = %v, want a [11, 12) marker at EOL", line.Highlights)
	}
}

// TestMarkerMidLineDoesNotExtendWidth verifies that a zero-width match
// in the middle of the line (not at EOL) marks an existing cell without
// extending the line width. The Clusters width is unchanged.
func TestMarkerMidLineDoesNotExtendWidth(t *testing.T) {
	dir := t.TempDir()
	content := "hello\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Zero-width match at byte 2 (inside "hello", maps to cell 2).
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("", 2, 2)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if !hasHighlight(line.Highlights, 2, 3) {
		t.Fatalf("Highlights = %v, want a [2, 3) marker at cell 2", line.Highlights)
	}
	// The cluster width should be 5 (just "hello"), not 6.
	if clusterWidth(line.Clusters) != 5 {
		t.Fatalf("Cluster width = %d, want 5 (mid-line marker does not extend width)", clusterWidth(line.Clusters))
	}
	if line.Display != "hello" {
		t.Fatalf("Display = %q, want %q (mid-line marker does not change display)", line.Display, "hello")
	}
}

// TestMarkerAndNonZeroWidthOnSameLine verifies that a line with both a
// zero-width marker and a non-zero-width highlight produces both. The
// marker is a one-cell highlight and the non-zero-width highlight
// covers its full cell range.
func TestMarkerAndNonZeroWidthOnSameLine(t *testing.T) {
	dir := t.TempDir()
	content := "hello world\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content,
			sm("hello", 0, 5), // non-zero-width: "hello" at [0, 5)
			sm("", 11, 11),    // zero-width: EOL marker at [11, 12)
		),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if !hasHighlight(line.Highlights, 0, 5) {
		t.Fatalf("Highlights = %v, want a [0, 5) non-zero-width highlight", line.Highlights)
	}
	if !hasHighlight(line.Highlights, 11, 12) {
		t.Fatalf("Highlights = %v, want a [11, 12) zero-width marker at EOL", line.Highlights)
	}
}

// TestMarkerEOLClusterIsLast verifies that the EOL marker's virtual
// cluster is the last cluster in the Clusters slice and has width 1.
func TestMarkerEOLClusterIsLast(t *testing.T) {
	dir := t.TempDir()
	content := "ab\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("", 2, 2)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Clusters) != 3 {
		t.Fatalf("Clusters len = %d, want 3 (2 content + 1 marker)", len(line.Clusters))
	}
	last := line.Clusters[len(line.Clusters)-1]
	if last.Width != 1 {
		t.Fatalf("Last cluster width = %d, want 1 (marker cell)", last.Width)
	}
}

// --- helpers ---

// hasHighlight reports whether the highlights contain [start, end).
func hasHighlight(hls [][2]int, start, end int) bool {
	for _, hl := range hls {
		if hl[0] == start && hl[1] == end {
			return true
		}
	}
	return false
}

// clusterWidth returns the total terminal cell width of the clusters.
func clusterWidth(clusters []filebuffer.Cluster) int {
	w := 0
	for _, c := range clusters {
		w += c.Width
	}
	return w
}

// startsWith reports whether s starts with prefix.
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
