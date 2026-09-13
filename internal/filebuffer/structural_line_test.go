package filebuffer_test

import (
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// --- Structural line handling (Issue #22) ---
//
// These tests cover the Issue #22 contracts: LF/CRLF/mixed terminators
// with retained original bytes, unterminated final lines, no phantom
// trailing line, the zero-line empty file with its three-cell gutter,
// terminator-to-EOL display mapping, text-plus-terminator spans
// highlighting visible text only, the leading UTF-8 BOM offset
// adjustment between raw-file and rg-line coordinates, and non-leading
// U+FEFF as ordinary content. Raw-file and rg-line coordinate views
// are maintained throughout.

// TestStructuralMixedTerminators verifies that a file with both LF and
// CRLF terminators splits into the correct lines with terminators
// removed from display. Line 1 ends with LF, line 2 with CRLF, line 3
// is unterminated. Each display string contains no terminator bytes.
func TestStructuralMixedTerminators(t *testing.T) {
	dir := t.TempDir()
	content := "a\nb\r\nc"
	p := writeFile(t, dir, "mixed.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 3 {
		t.Fatalf("LineCount = %d, want 3 (mixed LF/CRLF/unterminated)", buf.LineCount)
	}
	wantDisplays := []string{"a", "b", "c"}
	for i, want := range wantDisplays {
		if buf.Lines[i].Display != want {
			t.Fatalf("Line %d Display = %q, want %q (no terminator)", i, buf.Lines[i].Display, want)
		}
	}
}

// TestStructuralCRLFNotDisplayed verifies that CRLF terminators produce
// no display cells and no visible characters. The display text for a
// CRLF line contains neither \r nor \n.
func TestStructuralCRLFNotDisplayed(t *testing.T) {
	dir := t.TempDir()
	content := "hit\r\n"
	p := writeFile(t, dir, "crlf.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 1 {
		t.Fatalf("LineCount = %d, want 1", buf.LineCount)
	}
	if buf.Lines[0].Display != "hit" {
		t.Fatalf("Display = %q, want %q (CRLF not displayed)", buf.Lines[0].Display, "hit")
	}
}

// TestStructuralTerminatorMapsToEOLColumn verifies that removed
// terminator bytes map to the display end-of-line position. For
// "hit\r\n", byte 4 (\n) and byte 3 (\r) both map to display column 3
// (the position after "hit"). This is the terminator-to-EOL mapping
// for zero-width positions.
func TestStructuralTerminatorMapsToEOLColumn(t *testing.T) {
	dir := t.TempDir()
	content := "hit\r\n"
	p := writeFile(t, dir, "crlf.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	bc := buf.Lines[0].ByteCells
	// Raw bytes: h(0), i(1), t(2), \r(3), \n(4) = 5 entries.
	if len(bc) != 5 {
		t.Fatalf("ByteCells len = %d, want 5 (hit + CRLF)", len(bc))
	}
	// Byte 3 (\r): [3, 3) — zero-width at end-of-line.
	if bc[3] != [2]int{3, 3} {
		t.Fatalf("byte 3 (\\r) = %v, want [3, 3) (CRLF CR maps to EOL column)", bc[3])
	}
	// Byte 4 (\n): [3, 3) — zero-width at end-of-line.
	if bc[4] != [2]int{3, 3} {
		t.Fatalf("byte 4 (\\n) = %v, want [3, 3) (CRLF LF maps to EOL column)", bc[4])
	}
}

// TestStructuralLFTerminatorMapsToEOLColumn verifies that a lone LF
// terminator maps to the display end-of-line position. For "hit\n",
// byte 3 (\n) maps to display column 3.
func TestStructuralLFTerminatorMapsToEOLColumn(t *testing.T) {
	dir := t.TempDir()
	content := "hit\n"
	p := writeFile(t, dir, "lf.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	bc := buf.Lines[0].ByteCells
	// Raw bytes: h(0), i(1), t(2), \n(3) = 4 entries.
	if len(bc) != 4 {
		t.Fatalf("ByteCells len = %d, want 4 (hit + LF)", len(bc))
	}
	// Byte 3 (\n): [3, 3) — zero-width at end-of-line.
	if bc[3] != [2]int{3, 3} {
		t.Fatalf("byte 3 (\\n) = %v, want [3, 3) (LF maps to EOL column)", bc[3])
	}
}

// TestStructuralSpanTextPlusTerminatorHighlightsVisibleOnly verifies
// that a submatch span covering visible text plus the terminator
// highlights only the visible text. For "hit\r\n", a submatch from
// byte 0 to byte 5 (covering "hit\r\n") produces a highlight of
// [0, 3) — only "hit", not the terminator bytes.
func TestStructuralSpanTextPlusTerminatorHighlightsVisibleOnly(t *testing.T) {
	dir := t.TempDir()
	content := "hit\r\n"
	p := writeFile(t, dir, "crlf.txt", []byte(content))
	// Submatch covers the entire line including terminator: bytes 0-5.
	stops := []searchindex.Stop{
		mkStop("crlf.txt", 1, content, sm("hit\r\n", 0, 5)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1 (text+terminator span)", len(line.Highlights))
	}
	hl := line.Highlights[0]
	// The highlight must cover only the visible text "hit" (cells [0, 3)),
	// not the terminator bytes which map to the zero-width EOL position.
	if hl[0] != 0 || hl[1] != 3 {
		t.Fatalf("Highlight = [%d, %d), want [0, 3) (visible text only, not terminator)", hl[0], hl[1])
	}
}

// TestStructuralSpanTextPlusLFTerminatorHighlightsVisibleOnly verifies
// the text-plus-terminator span contract for a lone LF terminator. For
// "hit\n", a submatch from byte 0 to byte 4 highlights only "hit".
func TestStructuralSpanTextPlusLFTerminatorHighlightsVisibleOnly(t *testing.T) {
	dir := t.TempDir()
	content := "hit\n"
	p := writeFile(t, dir, "lf.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("lf.txt", 1, content, sm("hit\n", 0, 4)),
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
	if hl[0] != 0 || hl[1] != 3 {
		t.Fatalf("Highlight = [%d, %d), want [0, 3) (visible text only)", hl[0], hl[1])
	}
}

// TestStructuralStandaloneCREscapedAsCaretM verifies that a standalone
// CR (not followed by LF) is escaped as ^M by the safe-presentation
// core rather than treated as a terminator. The display text contains
// ^M for the standalone CR, and the line is not split at the CR.
func TestStructuralStandaloneCREscapedAsCaretM(t *testing.T) {
	dir := t.TempDir()
	// "a\rb\n" — CR at position 1 is standalone (followed by 'b', not LF).
	content := "a\rb\n"
	p := writeFile(t, dir, "cr.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 1 {
		t.Fatalf("LineCount = %d, want 1 (standalone CR is not a terminator)", buf.LineCount)
	}
	// Display: "a^Mb" — CR escaped as ^M, not split into two lines.
	if buf.Lines[0].Display != "a^Mb" {
		t.Fatalf("Display = %q, want %q (standalone CR escaped as ^M)", buf.Lines[0].Display, "a^Mb")
	}
}

// TestStructuralStandaloneCRByteCells verifies that the standalone CR
// byte maps to the two display cells of ^M. For "a\rb\n", byte 1 (\r)
// maps to display cells [1, 3) (the ^ and M cells).
func TestStructuralStandaloneCRByteCells(t *testing.T) {
	dir := t.TempDir()
	content := "a\rb\n"
	p := writeFile(t, dir, "cr.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	bc := buf.Lines[0].ByteCells
	// Raw bytes: a(0), \r(1), b(2), \n(3) = 4 entries.
	if len(bc) != 4 {
		t.Fatalf("ByteCells len = %d, want 4", len(bc))
	}
	// Byte 1 (\r): [1, 3) — the two cells of ^M.
	if bc[1] != [2]int{1, 3} {
		t.Fatalf("byte 1 (\\r) = %v, want [1, 3) (^M two cells)", bc[1])
	}
}

// --- Leading UTF-8 BOM (Issue #22) ---

// TestStructuralLeadingBOMInvisibleInDisplay verifies that a leading
// UTF-8 BOM (EF BB BF) is invisible in the display. The display text
// for line 1 of a BOM file contains no U+FEFF and no BOM bytes. The
// BOM is stripped before display so the content starts with the
// first real character.
func TestStructuralLeadingBOMInvisibleInDisplay(t *testing.T) {
	dir := t.TempDir()
	// BOM + "hit\n"
	content := []byte{0xEF, 0xBB, 0xBF, 'h', 'i', 't', '\n'}
	p := writeFile(t, dir, "bom.txt", content)
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 1 {
		t.Fatalf("LineCount = %d, want 1", buf.LineCount)
	}
	// Display must be "hit" — no BOM, no U+FEFF.
	if buf.Lines[0].Display != "hit" {
		t.Fatalf("Display = %q, want %q (leading BOM must be invisible)", buf.Lines[0].Display, "hit")
	}
}

// TestStructuralLeadingBOMFirstLineMatchMapsToRawByte3 verifies that a
// first-line match at rg offset 0 maps to raw byte 3 (after the
// three-byte BOM) and highlights the correct display cell. Ripgrep
// removes the leading BOM from searched line data, so rg offset 0 is
// the first content byte ('h'). The highlight must land on the 'h'
// cell (display column 0 after BOM stripping), not on a phantom BOM
// cell.
func TestStructuralLeadingBOMFirstLineMatchMapsToRawByte3(t *testing.T) {
	dir := t.TempDir()
	// BOM + "hit\r\n" on disk.
	content := []byte{0xEF, 0xBB, 0xBF, 'h', 'i', 't', '\r', '\n'}
	p := writeFile(t, dir, "bom.txt", content)
	// rg reports line 1 without the BOM: "hit\r\n". A match at rg
	// offset 0 is 'h'.
	stops := []searchindex.Stop{
		mkStop("bom.txt", 1, "hit\r\n", sm("h", 0, 1)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	// Display must be "hit" (BOM invisible).
	if line.Display != "hit" {
		t.Fatalf("Display = %q, want %q (BOM invisible)", line.Display, "hit")
	}
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1 (first-line match at rg offset 0)", len(line.Highlights))
	}
	hl := line.Highlights[0]
	// The match at rg offset 0 is 'h', which after BOM stripping is at
	// display cell 0. The highlight must be [0, 1).
	if hl[0] != 0 || hl[1] != 1 {
		t.Fatalf("Highlight = [%d, %d), want [0, 1) (rg offset 0 → raw byte 3 → display cell 0)", hl[0], hl[1])
	}
}

// TestStructuralLeadingBOMMatchAtRGOffset1 verifies that a first-line
// match at rg offset 1 (the second content byte, 'i') highlights the
// correct display cell. After BOM stripping, 'i' is at display cell 1.
func TestStructuralLeadingBOMMatchAtRGOffset1(t *testing.T) {
	dir := t.TempDir()
	content := []byte{0xEF, 0xBB, 0xBF, 'h', 'i', 't', '\r', '\n'}
	p := writeFile(t, dir, "bom.txt", content)
	// rg offset 1 = 'i' (BOM removed from rg's line view).
	stops := []searchindex.Stop{
		mkStop("bom.txt", 1, "hit\r\n", sm("i", 1, 2)),
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
	// 'i' is at display cell 1 after BOM stripping.
	if hl[0] != 1 || hl[1] != 2 {
		t.Fatalf("Highlight = [%d, %d), want [1, 2) (rg offset 1 → display cell 1)", hl[0], hl[1])
	}
}

// TestStructuralLeadingBOMDoesNotAffectSecondLine verifies that the
// BOM adjustment only applies to line 1. A match on line 2 at rg
// offset 0 highlights the correct cell with no BOM shift.
func TestStructuralLeadingBOMDoesNotAffectSecondLine(t *testing.T) {
	dir := t.TempDir()
	// BOM + "hit\nworld\n"
	content := []byte{0xEF, 0xBB, 0xBF, 'h', 'i', 't', '\n', 'w', 'o', 'r', 'l', 'd', '\n'}
	p := writeFile(t, dir, "bom.txt", content)
	// Line 2 match at rg offset 0 = 'w'. No BOM adjustment for line 2.
	stops := []searchindex.Stop{
		mkStop("bom.txt", 2, "world\n", sm("w", 0, 1)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 2 {
		t.Fatalf("LineCount = %d, want 2", buf.LineCount)
	}
	// Line 1: BOM stripped, display "hit".
	if buf.Lines[0].Display != "hit" {
		t.Fatalf("Line 1 Display = %q, want %q (BOM invisible on line 1)", buf.Lines[0].Display, "hit")
	}
	// Line 2: no BOM, display "world".
	if buf.Lines[1].Display != "world" {
		t.Fatalf("Line 2 Display = %q, want %q", buf.Lines[1].Display, "world")
	}
	line2 := buf.Lines[1]
	if len(line2.Highlights) != 1 {
		t.Fatalf("Line 2 Highlights len = %d, want 1", len(line2.Highlights))
	}
	hl := line2.Highlights[0]
	// 'w' at display cell 0 on line 2 (no BOM adjustment).
	if hl[0] != 0 || hl[1] != 1 {
		t.Fatalf("Line 2 Highlight = [%d, %d), want [0, 1) (no BOM shift on line 2)", hl[0], hl[1])
	}
}

// TestStructuralLeadingBOMOnlyFile verifies that a file containing only
// the BOM (no content) produces zero source lines with a three-cell
// gutter, just like an empty file. The BOM is a file-level prefix, not
// a line.
func TestStructuralLeadingBOMOnlyFile(t *testing.T) {
	dir := t.TempDir()
	content := []byte{0xEF, 0xBB, 0xBF}
	p := writeFile(t, dir, "bom_only.txt", content)
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 0 {
		t.Fatalf("LineCount = %d, want 0 (BOM-only file has no lines)", buf.LineCount)
	}
	if buf.GutterWidth != 3 {
		t.Fatalf("GutterWidth = %d, want 3 (one digit slot + two spaces)", buf.GutterWidth)
	}
	if len(buf.Lines) != 0 {
		t.Fatalf("Lines len = %d, want 0 (no source rows)", len(buf.Lines))
	}
}

// TestStructuralLeadingBOMWithNewline verifies that a BOM followed by
// just a newline produces one empty line (the newline terminates line
// 1, which has no visible content after BOM stripping).
func TestStructuralLeadingBOMWithNewline(t *testing.T) {
	dir := t.TempDir()
	content := []byte{0xEF, 0xBB, 0xBF, '\n'}
	p := writeFile(t, dir, "bom_nl.txt", content)
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 1 {
		t.Fatalf("LineCount = %d, want 1 (BOM + newline = one empty line)", buf.LineCount)
	}
	if buf.Lines[0].Display != "" {
		t.Fatalf("Display = %q, want %q (empty line after BOM stripping)", buf.Lines[0].Display, "")
	}
}

// TestStructuralNonLeadingFEFFIsOrdinaryContent verifies that a
// non-leading U+FEFF (not at the file start) is ordinary content: it
// is displayed and not stripped. A match on it highlights the correct
// cell. This contrasts with a leading BOM which is invisible.
func TestStructuralNonLeadingFEFFIsOrdinaryContent(t *testing.T) {
	dir := t.TempDir()
	// "a\uFEFFb\n" — U+FEFF is in the middle, not at the start.
	content := "a\uFEFFb\n"
	p := writeFile(t, dir, "feff.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 1 {
		t.Fatalf("LineCount = %d, want 1", buf.LineCount)
	}
	// The display must contain U+FEFF (it is ordinary content, not
	// stripped like a leading BOM).
	if !stringsContainsRune(buf.Lines[0].Display, '\uFEFF') {
		t.Fatalf("Display = %q, want it to contain U+FEFF (non-leading U+FEFF is content)", buf.Lines[0].Display)
	}
}

// TestStructuralNonLeadingFEFFMatchHighlights verifies that a submatch
// on a non-leading U+FEFF produces a highlight. The U+FEFF is ordinary
// content and can be matched and highlighted.
func TestStructuralNonLeadingFEFFMatchHighlights(t *testing.T) {
	dir := t.TempDir()
	// "a\uFEFFb\n" — U+FEFF at byte offset 1 (3 UTF-8 bytes: EF BB BF).
	content := "a\uFEFFb\n"
	p := writeFile(t, dir, "feff.txt", []byte(content))
	// Submatch on U+FEFF: bytes 1-4 (the 3 BOM bytes).
	stops := []searchindex.Stop{
		mkStop("feff.txt", 1, content, sm("\uFEFF", 1, 4)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line := buf.Lines[0]
	if len(line.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1 (non-leading U+FEFF is matchable content)", len(line.Highlights))
	}
}

// --- Empty file and final-line contracts (Issue #22) ---

// TestStructuralEmptyFileZeroLinesThreeCellGutter verifies that an
// empty file produces zero source lines, a three-cell gutter (one
// digit slot plus two spaces), and no source rows in the Lines slice.
func TestStructuralEmptyFileZeroLinesThreeCellGutter(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "empty.txt", nil)
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 0 {
		t.Fatalf("LineCount = %d, want 0 (empty file has zero lines)", buf.LineCount)
	}
	if buf.GutterWidth != 3 {
		t.Fatalf("GutterWidth = %d, want 3 (one digit slot + two spaces)", buf.GutterWidth)
	}
	if len(buf.Lines) != 0 {
		t.Fatalf("Lines len = %d, want 0 (no source rows for empty file)", len(buf.Lines))
	}
}

// TestStructuralNoFinalNewlineYieldsFinalLine verifies that a file
// without a trailing newline still counts and displays the last line.
func TestStructuralNoFinalNewlineYieldsFinalLine(t *testing.T) {
	dir := t.TempDir()
	content := "alpha\nbeta\ngamma"
	p := writeFile(t, dir, "no_nl.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 3 {
		t.Fatalf("LineCount = %d, want 3 (unterminated final line counted)", buf.LineCount)
	}
	if buf.Lines[2].Display != "gamma" {
		t.Fatalf("Line 2 Display = %q, want %q (unterminated final line displayed)", buf.Lines[2].Display, "gamma")
	}
}

// TestStructuralTrailingNewlineNoPhantomLine verifies that a trailing
// newline does not invent an extra empty line. "a\nb\n" has two lines,
// not three.
func TestStructuralTrailingNewlineNoPhantomLine(t *testing.T) {
	dir := t.TempDir()
	content := "a\nb\n"
	p := writeFile(t, dir, "trailing.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 2 {
		t.Fatalf("LineCount = %d, want 2 (trailing newline does not add phantom line)", buf.LineCount)
	}
	if len(buf.Lines) != 2 {
		t.Fatalf("Lines len = %d, want 2 (no phantom empty line)", len(buf.Lines))
	}
	if buf.Lines[1].Display != "b" {
		t.Fatalf("Line 1 Display = %q, want %q", buf.Lines[1].Display, "b")
	}
}

// TestStructuralCRLFTrailingNewlineNoPhantomLine verifies that a
// trailing CRLF does not invent an extra empty line. "a\r\nb\r\n" has
// two lines, not three.
func TestStructuralCRLFTrailingNewlineNoPhantomLine(t *testing.T) {
	dir := t.TempDir()
	content := "a\r\nb\r\n"
	p := writeFile(t, dir, "crlf_trailing.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 2 {
		t.Fatalf("LineCount = %d, want 2 (trailing CRLF does not add phantom line)", buf.LineCount)
	}
	if len(buf.Lines) != 2 {
		t.Fatalf("Lines len = %d, want 2", len(buf.Lines))
	}
}

// TestStructuralRetainedOriginalBytesForMapping verifies that the
// ByteCells slice has one entry per original raw byte including
// terminator bytes. The original line bytes including terminators
// are retained for byte-coordinate mapping and later validation
// (Issue #29). For "hit\r\n", ByteCells has 5 entries (h, i, t, \r,
// \n).
func TestStructuralRetainedOriginalBytesForMapping(t *testing.T) {
	dir := t.TempDir()
	content := "hit\r\n"
	p := writeFile(t, dir, "crlf.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	bc := buf.Lines[0].ByteCells
	// 5 raw bytes (h, i, t, \r, \n) → 5 ByteCells entries.
	if len(bc) != 5 {
		t.Fatalf("ByteCells len = %d, want 5 (all original bytes including terminators retained)", len(bc))
	}
	// Visible text bytes map to their display cells.
	if bc[0] != [2]int{0, 1} {
		t.Fatalf("byte 0 (h) = %v, want [0, 1)", bc[0])
	}
	if bc[1] != [2]int{1, 2} {
		t.Fatalf("byte 1 (i) = %v, want [1, 2)", bc[1])
	}
	if bc[2] != [2]int{2, 3} {
		t.Fatalf("byte 2 (t) = %v, want [2, 3)", bc[2])
	}
}

// stringsContainsRune is a small helper to avoid importing strings in
// this test file.
func stringsContainsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
