package filebuffer_test

import (
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// --- Grapheme cluster highlight expansion (Issue #21) ---

// TestHighlightExpandStartInsideCluster verifies that a nonempty span
// starting inside a grapheme cluster (not at the cluster start) expands
// outward to the entire cluster start. The submatch covers only the
// combining mark bytes of an "e\u0301" cluster; the highlight must
// expand left to cover the base "e" as well, so the whole é glyph is
// highlighted.
func TestHighlightExpandStartInsideCluster(t *testing.T) {
	dir := t.TempDir()
	content := "abcde\u0301f\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Submatch on the combining mark only: bytes 5-7 (0xCC 0x81).
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("\u0301", 5, 7)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1 (combining-only match expanded to cluster)", len(line.Highlights))
	}
	hl := line.Highlights[0]
	// The é cluster (e + combining) occupies display cells [4, 5).
	// The highlight must expand to [4, 5) — the whole cluster.
	if hl[0] != 4 || hl[1] != 5 {
		t.Fatalf("Highlight = [%d, %d), want [4, 5) (expanded to whole é cluster)", hl[0], hl[1])
	}
}

// TestHighlightExpandEndInsideCluster verifies that a nonempty span
// ending inside a grapheme cluster expands outward to the entire cluster
// end. The submatch covers the base "e" but not the combining mark; the
// highlight must expand right to include the combining mark, covering
// the whole é cluster.
func TestHighlightExpandEndInsideCluster(t *testing.T) {
	dir := t.TempDir()
	content := "abcde\u0301f\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Submatch on "e" only: byte 4 (the base of the é cluster).
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("e", 4, 5)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1", len(line.Highlights))
	}
	hl := line.Highlights[0]
	// The é cluster occupies display cells [4, 5). The highlight must
	// expand to [4, 5) — the whole cluster including the combining mark.
	if hl[0] != 4 || hl[1] != 5 {
		t.Fatalf("Highlight = [%d, %d), want [4, 5) (expanded to whole é cluster)", hl[0], hl[1])
	}
}

// TestHighlightExpandBothInsideCluster verifies that a nonempty span
// whose start and end are both inside the same cluster expands to cover
// the entire cluster.
func TestHighlightExpandBothInsideCluster(t *testing.T) {
	dir := t.TempDir()
	// "中" is a wide character (3 UTF-8 bytes, 2 cells).
	content := "a中b\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Submatch on the first 2 bytes of "中" (bytes 1-3): partially
	// covers the wide cluster.
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("\xe4\xb8", 1, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1", len(line.Highlights))
	}
	hl := line.Highlights[0]
	// "中" occupies display cells [1, 3) (2 cells). The highlight must
	// expand to [1, 3) — both cells of the wide glyph.
	if hl[0] != 1 || hl[1] != 3 {
		t.Fatalf("Highlight = [%d, %d), want [1, 3) (expanded to whole wide cluster)", hl[0], hl[1])
	}
}

// TestHighlightCombiningOnlyMatchHighlightsBaseCluster verifies that a
// combining-only match within a base cluster highlights that whole
// cluster. The submatch covers only the combining mark bytes; the
// highlight must cover the base character's cells.
func TestHighlightCombiningOnlyMatchHighlightsBaseCluster(t *testing.T) {
	dir := t.TempDir()
	// "e\u0301" = é as two code points (base + combining).
	content := "e\u0301\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Submatch on the combining mark only: bytes 1-3 (0xCC 0x81).
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("\u0301", 1, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1 (combining-only match highlights base cluster)", len(line.Highlights))
	}
	hl := line.Highlights[0]
	// The é cluster occupies display cells [0, 1). The highlight must
	// expand to [0, 1) — the whole cluster.
	if hl[0] != 0 || hl[1] != 1 {
		t.Fatalf("Highlight = [%d, %d), want [0, 1) (combining-only match expanded to base cluster)", hl[0], hl[1])
	}
}

// TestHighlightStandaloneClusterFallbackCell verifies that a cluster
// without a base or independent visible cell (a standalone combining
// mark with width 0) receives a visible fallback cell so the highlight
// is never zero cells.
func TestHighlightStandaloneClusterFallbackCell(t *testing.T) {
	dir := t.TempDir()
	// Standalone combining mark at line start (no base character).
	content := "\u0301\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Submatch on the combining mark: bytes 0-2 (0xCC 0x81).
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("\u0301", 0, 2)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1 (standalone cluster with fallback cell)", len(line.Highlights))
	}
	hl := line.Highlights[0]
	// The standalone combining mark has width 0. The highlight must
	// receive a visible fallback cell: [0, 1) so it is never zero cells.
	if hl[0] != 0 || hl[1] != 1 {
		t.Fatalf("Highlight = [%d, %d), want [0, 1) (standalone cluster fallback cell)", hl[0], hl[1])
	}
}

// TestHighlightWidePairNeverSplit verifies that a wide glyph (2 cells)
// is never split by highlight boundaries. A submatch partially covering
// the wide glyph expands to cover both cells.
func TestHighlightWidePairNeverSplit(t *testing.T) {
	dir := t.TempDir()
	// "中" is a CJK ideograph, 2 cells wide.
	content := "x中y\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Submatch on the last byte of "中" (byte 3): partially covers the
	// wide cluster.
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("\xad", 3, 4)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1", len(line.Highlights))
	}
	hl := line.Highlights[0]
	// "中" occupies display cells [1, 3) (2 cells). The highlight must
	// expand to [1, 3) — both cells, never split.
	if hl[0] != 1 || hl[1] != 3 {
		t.Fatalf("Highlight = [%d, %d), want [1, 3) (wide pair never split)", hl[0], hl[1])
	}
}

// TestHighlightZWJSequenceHandledBySharedPolicy verifies that an emoji
// ZWJ sequence is handled by the shared grapheme policy as one cluster.
// A submatch partially covering the ZWJ sequence expands to the entire
// sequence.
func TestHighlightZWJSequenceHandledBySharedPolicy(t *testing.T) {
	dir := t.TempDir()
	// 👨‍👩‍👧 = man + ZWJ + woman + ZWJ + girl (one grapheme cluster).
	// UTF-8: 👨(4) + \u200D(3) + 👩(4) + \u200D(3) + 👧(4) = 18 bytes.
	content := "a👨‍👩‍👧b\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Submatch on the ZWJ character between man and woman.
	// Find the ZWJ byte offset: 'a'(1) + 👨(4) = byte 5 is the ZWJ.
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("\u200d", 5, 8)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1 (ZWJ sequence expanded to one cluster)", len(line.Highlights))
	}
	hl := line.Highlights[0]
	// The ZWJ sequence is one cluster. 'a' is cell [0, 1), the emoji
	// cluster starts at cell 1. The emoji has width 2 (uniseg counts
	// emoji as 2 cells). So the highlight must expand to [1, 3).
	if hl[0] != 1 || hl[1] != 3 {
		t.Fatalf("Highlight = [%d, %d), want [1, 3) (ZWJ sequence as one cluster)", hl[0], hl[1])
	}
}

// TestHighlightExpandedSpanIsSoleSource verifies that the expanded span
// is what FileBuffer hands to Viewport and App: Line.Highlights contains
// the expanded spans, and Line.ByteCells maps submatch bytes to the
// expanded cell coordinates so the Issue #19 reveal targets the cluster
// start.
func TestHighlightExpandedSpanIsSoleSource(t *testing.T) {
	dir := t.TempDir()
	content := "e\u0301\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("\u0301", 1, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	// ByteCells: e(0), CC(1), 81(2), \n(3) = 4 entries.
	if len(line.ByteCells) != 4 {
		t.Fatalf("ByteCells len = %d, want 4 (e + 2 combining bytes + newline)", len(line.ByteCells))
	}
	// Byte 0 (e): [0, 1) — the cluster's full range (grapheme width).
	if line.ByteCells[0] != [2]int{0, 1} {
		t.Fatalf("ByteCells[0] = %v, want [0, 1) (cluster range for base)", line.ByteCells[0])
	}
	// Byte 1 (combining): [0, 1) — same cluster range, not [1, 2)
	// (1-cell-per-rune) which would misplace the reveal target.
	if line.ByteCells[1] != [2]int{0, 1} {
		t.Fatalf("ByteCells[1] = %v, want [0, 1) (cluster range for combining mark)", line.ByteCells[1])
	}
	// Byte 2 (combining): [0, 1) — same cluster range.
	if line.ByteCells[2] != [2]int{0, 1} {
		t.Fatalf("ByteCells[2] = %v, want [0, 1) (cluster range for combining mark)", line.ByteCells[2])
	}
	// Byte 3 (newline): [2, 2) — zero-width, maps to end-of-line.
	// The original EscapeContent cell for the newline is 2 (after the
	// combining mark advanced the cell counter); expansion does not
	// remap it because its display byte offset is past the cluster.
	if line.ByteCells[3] != [2]int{2, 2} {
		t.Fatalf("ByteCells[3] = %v, want [2, 2) (newline zero-width at end-of-line)", line.ByteCells[3])
	}
}
