package filebuffer_test

import (
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// --- Standalone combining cluster fallback cell (Issue #43) ---
//
// The recorded fallback representation (Notes/decisions/
// 043-combining-cluster-fallback-cell.md) is Candidate A: a standalone
// zero-width cluster displays as U+25CC ◌ followed by the cluster's
// original combining-mark bytes, occupying exactly one terminal cell.
// These tests assert the actual display bytes and one-cell geometry —
// a silently different fallback convention must fail.

// TestStandaloneClusterFallbackDisplayBytes verifies that a standalone
// combining mark with no base produces a real one-cell cluster whose
// display bytes are the recorded fallback: ◌ (U+25CC) followed by the
// original mark bytes. The following character forms the next cluster
// in the next cell with no overlap and no shared cell.
func TestStandaloneClusterFallbackDisplayBytes(t *testing.T) {
	dir := t.TempDir()
	content := "́x\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	// The recorded display bytes: ◌ + the combining mark + x.
	wantDisplay := "◌́x"
	if line.Display != wantDisplay {
		t.Fatalf("Display = %q, want %q (standalone cluster gains ◌ fallback base)", line.Display, wantDisplay)
	}
	if len(line.Clusters) != 2 {
		t.Fatalf("Clusters len = %d, want 2 (fallback unit + x)", len(line.Clusters))
	}
	fb := line.Clusters[0]
	if fb.StartByte != 0 || fb.EndByte != len("◌́") || fb.Width != 1 {
		t.Fatalf("fallback cluster = {%d, %d, %d}, want {0, %d, 1} covering ◌́",
			fb.StartByte, fb.EndByte, fb.Width, len("◌́"))
	}
	next := line.Clusters[1]
	if next.StartByte != len("◌́") || next.EndByte != len("◌́x") || next.Width != 1 {
		t.Fatalf("following cluster = {%d, %d, %d}, want {%d, %d, 1} covering x",
			next.StartByte, next.EndByte, next.Width, len("◌́"), len("◌́x"))
	}
	// The recorded expectation under the shared rivo/uniseg policy:
	// ◌ + combining marks segments as a single cluster of width 1.
	seg := safepresentation.GraphemeClusters(line.Display)
	if len(seg) != 2 || seg[0].Width != 1 || seg[0].StartByte != 0 || seg[0].EndByte != len("◌́") {
		t.Fatalf("re-segmented display = %+v, want ◌́ as one width-1 cluster", seg)
	}
}

// TestStandaloneClusterFallbackByteCells verifies that byte-to-cell
// mapping resolves the fallback cell to the cluster's original source
// bytes while the following cluster advances past the fallback cell:
// cellPos advances by the fallback width so no following byte shares
// the cell.
func TestStandaloneClusterFallbackByteCells(t *testing.T) {
	dir := t.TempDir()
	content := "́x\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	bc := buf.Lines[0].ByteCells
	// Raw bytes: 0xCC(0), 0x81(1), 'x'(2), '\n'(3).
	if len(bc) != 4 {
		t.Fatalf("ByteCells len = %d, want 4", len(bc))
	}
	if bc[0] != [2]int{0, 1} || bc[1] != [2]int{0, 1} {
		t.Fatalf("mark bytes = %v, %v, want [0, 1) each (fallback cell maps to source bytes)", bc[0], bc[1])
	}
	if bc[2] != [2]int{1, 2} {
		t.Fatalf("x byte = %v, want [1, 2) (cellPos advanced by the fallback width)", bc[2])
	}
	if bc[3] != [2]int{2, 2} {
		t.Fatalf("newline byte = %v, want [2, 2) (end-of-line after fallback cell)", bc[3])
	}
}

// TestStandaloneClusterFallbackContentWidth verifies that the fallback
// cell counts in the line's content extent like any other cell.
func TestStandaloneClusterFallbackContentWidth(t *testing.T) {
	dir := t.TempDir()
	content := "́x\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got := buf.Lines[0].ContentWidth; got != 2 {
		t.Fatalf("ContentWidth = %d, want 2 (fallback cell + x)", got)
	}
}

// TestStandaloneClusterFallbackHighlight verifies that a match covering
// the standalone cluster highlights exactly the fallback cell and
// nothing adjacent.
func TestStandaloneClusterFallbackHighlight(t *testing.T) {
	dir := t.TempDir()
	content := "́x\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Submatch on the combining mark only: bytes 0-2 (0xCC 0x81).
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("́", 0, 2)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1", len(line.Highlights))
	}
	if line.Highlights[0] != [2]int{0, 1} {
		t.Fatalf("Highlight = %v, want [0, 1) (exactly the fallback cell)", line.Highlights[0])
	}
}

// TestStandaloneClusterFallbackHighlightSpan verifies that a match
// spanning the standalone cluster and the following character covers
// both cells — the fallback cell counts like any other cell.
func TestStandaloneClusterFallbackHighlightSpan(t *testing.T) {
	dir := t.TempDir()
	content := "́x\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("́x", 0, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1", len(line.Highlights))
	}
	if line.Highlights[0] != [2]int{0, 2} {
		t.Fatalf("Highlight = %v, want [0, 2) (fallback cell plus x)", line.Highlights[0])
	}
}

// TestMultipleStandaloneClustersFallback verifies that consecutive
// zero-width clusters mid-line each receive their own one-cell
// fallback and that cell positions never overlap. A zero-width space
// forces a grapheme break, so the combining mark after it is a second
// standalone cluster.
func TestMultipleStandaloneClustersFallback(t *testing.T) {
	dir := t.TempDir()
	// "a" + ZWSP (width 0, breaks grapheme) + combining mark (width 0) + "x".
	content := "a​́x\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	wantDisplay := "a◌​◌́x"
	if line.Display != wantDisplay {
		t.Fatalf("Display = %q, want %q (each zero-width cluster gains ◌)", line.Display, wantDisplay)
	}
	if len(line.Clusters) != 4 {
		t.Fatalf("Clusters len = %d, want 4 (a + two fallback units + x)", len(line.Clusters))
	}
	for i, c := range line.Clusters {
		if c.Width != 1 {
			t.Fatalf("cluster %d Width = %d, want 1 (each fallback is one cell)", i, c.Width)
		}
	}
	if got := line.ContentWidth; got != 4 {
		t.Fatalf("ContentWidth = %d, want 4", got)
	}
	bc := line.ByteCells
	// Raw bytes: 'a'(0), ZWSP(1-3), mark(4-5), 'x'(6), '\n'(7).
	if bc[0] != [2]int{0, 1} {
		t.Fatalf("'a' byte = %v, want [0, 1)", bc[0])
	}
	for i := 1; i <= 3; i++ {
		if bc[i] != [2]int{1, 2} {
			t.Fatalf("ZWSP byte %d = %v, want [1, 2) (its fallback cell)", i, bc[i])
		}
	}
	for i := 4; i <= 5; i++ {
		if bc[i] != [2]int{2, 3} {
			t.Fatalf("mark byte %d = %v, want [2, 3) (its fallback cell)", i, bc[i])
		}
	}
	if bc[6] != [2]int{3, 4} {
		t.Fatalf("'x' byte = %v, want [3, 4) (no overlap with either fallback)", bc[6])
	}
}

// TestStandaloneClusterFallbackIsNormalizedToOneCell verifies the
// recorded normalization rule: the fallback unit is constructed as
// exactly one cell in the cluster table regardless of how the width
// library would measure the composed unit — no zero-width cluster may
// survive in the line's cluster table.
func TestStandaloneClusterFallbackIsNormalizedToOneCell(t *testing.T) {
	dir := t.TempDir()
	// Two standalone combining marks segment as one zero-width cluster;
	// the whole unit becomes a single ◌-based fallback cell.
	content := "́́y\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	for i, c := range line.Clusters {
		if c.Width <= 0 {
			t.Fatalf("cluster %d Width = %d: a zero-width cluster survived (fallback must normalize to one cell)", i, c.Width)
		}
	}
	wantDisplay := "◌́́y"
	if line.Display != wantDisplay {
		t.Fatalf("Display = %q, want %q", line.Display, wantDisplay)
	}
	if got := line.ContentWidth; got != 2 {
		t.Fatalf("ContentWidth = %d, want 2 (one fallback cell + y)", got)
	}
}
