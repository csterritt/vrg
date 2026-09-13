package safepresentation_test

import (
	"strings"
	"testing"

	"vrg/internal/safepresentation"
	"vrg/internal/sinkfixtures"
)

// cellRange is a test helper for readability.
type cellRange = [2]int

// cr constructs a cell range [start, end).
func cr(start, end int) cellRange { return cellRange{start, end} }

// --- Path escaping tests ---

// TestPathEscapeNewline verifies that \n in a path is escaped as the
// two-character sequence \n and the byte→cell map covers both cells.
func TestPathEscapeNewline(t *testing.T) {
	d := safepresentation.EscapePath([]byte("foo\nbar"))
	if d.Text != `foo\nbar` {
		t.Fatalf("Text = %q, want %q", d.Text, `foo\nbar`)
	}
	// Byte 3 (\n) maps to cells [3, 5).
	if got := d.ByteCells[3]; got[0] != 3 || got[1] != 5 {
		t.Fatalf("ByteCells[3] = %v, want [3, 5)", got)
	}
}

// TestPathEscapeCarriageReturn verifies that \r is escaped as \r.
func TestPathEscapeCarriageReturn(t *testing.T) {
	d := safepresentation.EscapePath([]byte("a\rb"))
	if d.Text != `a\rb` {
		t.Fatalf("Text = %q, want %q", d.Text, `a\rb`)
	}
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 3 {
		t.Fatalf("ByteCells[1] = %v, want [1, 3)", got)
	}
}

// TestPathEscapeTab verifies that \t is escaped as \t.
func TestPathEscapeTab(t *testing.T) {
	d := safepresentation.EscapePath([]byte("a\tb"))
	if d.Text != `a\tb` {
		t.Fatalf("Text = %q, want %q", d.Text, `a\tb`)
	}
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 3 {
		t.Fatalf("ByteCells[1] = %v, want [1, 3)", got)
	}
}

// TestPathEscapeBackslash verifies that a literal backslash is escaped as \\.
func TestPathEscapeBackslash(t *testing.T) {
	d := safepresentation.EscapePath([]byte(`a\b`))
	if d.Text != `a\\b` {
		t.Fatalf("Text = %q, want %q", d.Text, `a\\b`)
	}
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 3 {
		t.Fatalf("ByteCells[1] = %v, want [1, 3)", got)
	}
}

// TestPathEscapeInvalidUTF8 verifies that invalid UTF-8 bytes are escaped
// as \xNN (4 cells) and the byte→cell map covers all 4 cells.
func TestPathEscapeInvalidUTF8(t *testing.T) {
	d := safepresentation.EscapePath([]byte("a\xffb"))
	if d.Text != `a\xffb` {
		t.Fatalf("Text = %q, want %q", d.Text, `a\xffb`)
	}
	// Byte 1 (0xff) maps to cells [1, 5).
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 5 {
		t.Fatalf("ByteCells[1] = %v, want [1, 5)", got)
	}
}

// TestPathEscapeMultipleInvalidUTF8 verifies several invalid bytes in a row.
func TestPathEscapeMultipleInvalidUTF8(t *testing.T) {
	d := safepresentation.EscapePath([]byte("\xff\xfe"))
	if d.Text != `\xff\xfe` {
		t.Fatalf("Text = %q, want %q", d.Text, `\xff\xfe`)
	}
	if got := d.ByteCells[0]; got[0] != 0 || got[1] != 4 {
		t.Fatalf("ByteCells[0] = %v, want [0, 4)", got)
	}
	if got := d.ByteCells[1]; got[0] != 4 || got[1] != 8 {
		t.Fatalf("ByteCells[1] = %v, want [4, 8)", got)
	}
}

// TestPathEscapeC0Control verifies that other C0 controls (not \n, \r, \t,
// \\) use caret notation. BEL (0x07) becomes ^G.
func TestPathEscapeC0Control(t *testing.T) {
	d := safepresentation.EscapePath([]byte("a\x07b"))
	if d.Text != "a^Gb" {
		t.Fatalf("Text = %q, want %q", d.Text, "a^Gb")
	}
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 3 {
		t.Fatalf("ByteCells[1] = %v, want [1, 3)", got)
	}
}

// TestPathEscapeESC verifies that ESC (0x1b) is escaped as ^[ in paths.
func TestPathEscapeESC(t *testing.T) {
	d := safepresentation.EscapePath([]byte("\x1b"))
	if d.Text != "^[" {
		t.Fatalf("Text = %q, want %q", d.Text, "^[")
	}
	if got := d.ByteCells[0]; got[0] != 0 || got[1] != 2 {
		t.Fatalf("ByteCells[0] = %v, want [0, 2)", got)
	}
}

// TestPathEscapeDEL verifies that DEL (0x7f) is escaped as ^?.
func TestPathEscapeDEL(t *testing.T) {
	d := safepresentation.EscapePath([]byte("a\x7fb"))
	if d.Text != "a^?b" {
		t.Fatalf("Text = %q, want %q", d.Text, "a^?b")
	}
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 3 {
		t.Fatalf("ByteCells[1] = %v, want [1, 3)", got)
	}
}

// TestPathEscapeC1Control verifies that C1 controls (valid UTF-8 for
// U+0080–U+009F) are escaped as \u00XX (6 cells).
func TestPathEscapeC1Control(t *testing.T) {
	// U+0085 (NEL) encoded as 0xc2 0x85.
	d := safepresentation.EscapePath([]byte("a\xc2\x85b"))
	if d.Text != `a\u0085b` {
		t.Fatalf("Text = %q, want %q", d.Text, `a\u0085b`)
	}
	// Both bytes of the 2-byte UTF-8 sequence map to the same 6 cells.
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 7 {
		t.Fatalf("ByteCells[1] = %v, want [1, 7)", got)
	}
	if got := d.ByteCells[2]; got[0] != 1 || got[1] != 7 {
		t.Fatalf("ByteCells[2] = %v, want [1, 7)", got)
	}
}

// TestPathPreservePrintableUnicode verifies that valid printable Unicode
// passes through unchanged and each codepoint occupies the expected cells.
func TestPathPreservePrintableUnicode(t *testing.T) {
	d := safepresentation.EscapePath([]byte("café"))
	// é is 2 bytes (0xc3 0xa9), displays as 1 cell.
	if d.Text != "café" {
		t.Fatalf("Text = %q, want %q", d.Text, "café")
	}
	// 'c' at byte 0 → cell [0, 1)
	if got := d.ByteCells[0]; got[0] != 0 || got[1] != 1 {
		t.Fatalf("ByteCells[0] = %v, want [0, 1)", got)
	}
	// 'a' at byte 1 → cell [1, 2)
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 2 {
		t.Fatalf("ByteCells[1] = %v, want [1, 2)", got)
	}
	// 'f' at byte 2 → cell [2, 3)
	if got := d.ByteCells[2]; got[0] != 2 || got[1] != 3 {
		t.Fatalf("ByteCells[2] = %v, want [2, 3)", got)
	}
	// é at bytes 3-4 → cell [3, 4) (both bytes map to the same cell)
	if got := d.ByteCells[3]; got[0] != 3 || got[1] != 4 {
		t.Fatalf("ByteCells[3] = %v, want [3, 4)", got)
	}
	if got := d.ByteCells[4]; got[0] != 3 || got[1] != 4 {
		t.Fatalf("ByteCells[4] = %v, want [3, 4)", got)
	}
}

// TestPathPreserveNonASCIIPrintable verifies that non-ASCII printable
// characters like CJK pass through.
func TestPathPreserveNonASCIIPrintable(t *testing.T) {
	d := safepresentation.EscapePath([]byte("文件"))
	if d.Text != "文件" {
		t.Fatalf("Text = %q, want %q", d.Text, "文件")
	}
}

// TestPathMixedEscapes verifies a path with multiple escape types and
// checks the byte→cell map is consistent.
func TestPathMixedEscapes(t *testing.T) {
	raw := []byte("a\nb\\\xff")
	d := safepresentation.EscapePath(raw)
	want := `a\nb\\\xff`
	if d.Text != want {
		t.Fatalf("Text = %q, want %q", d.Text, want)
	}
	// 'a' → [0, 1)
	// '\n' → [1, 3)
	// 'b' → [3, 4)
	// '\\' → [4, 6)
	// '\xff' → [6, 10)
	wantCells := []cellRange{cr(0, 1), cr(1, 3), cr(3, 4), cr(4, 6), cr(6, 10)}
	if len(d.ByteCells) != len(wantCells) {
		t.Fatalf("ByteCells len = %d, want %d", len(d.ByteCells), len(wantCells))
	}
	for i, want := range wantCells {
		if d.ByteCells[i] != want {
			t.Fatalf("ByteCells[%d] = %v, want %v", i, d.ByteCells[i], want)
		}
	}
}

// TestPathEmpty verifies that an empty path produces empty output.
func TestPathEmpty(t *testing.T) {
	d := safepresentation.EscapePath(nil)
	if d.Text != "" {
		t.Fatalf("Text = %q, want empty", d.Text)
	}
	if len(d.ByteCells) != 0 {
		t.Fatalf("ByteCells len = %d, want 0", len(d.ByteCells))
	}
}

// TestPathByteCellsLenMatchesInput verifies that the byte→cell map has
// one entry per input byte.
func TestPathByteCellsLenMatchesInput(t *testing.T) {
	raw := []byte("hello\xff\x1b\n")
	d := safepresentation.EscapePath(raw)
	if len(d.ByteCells) != len(raw) {
		t.Fatalf("ByteCells len = %d, want %d (one per input byte)", len(d.ByteCells), len(raw))
	}
}

// --- Content escaping tests ---

// TestContentInvalidUTF8 verifies that invalid UTF-8 renders as U+FFFD
// while the raw-byte mapping is retained (the byte→cell map covers the
// replacement cell).
func TestContentInvalidUTF8(t *testing.T) {
	d := safepresentation.EscapeContent([]byte("a\xffb"))
	if d.Text != "a\ufffdb" {
		t.Fatalf("Text = %q, want %q", d.Text, "a\ufffdb")
	}
	// Byte 1 (0xff) maps to cell [1, 2).
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 2 {
		t.Fatalf("ByteCells[1] = %v, want [1, 2)", got)
	}
}

// TestContentInvalidUTF8RetainedMapping verifies that a match covering
// an invalid UTF-8 byte maps to the U+FFFD cell.
func TestContentInvalidUTF8RetainedMapping(t *testing.T) {
	raw := []byte("\xff\xff")
	d := safepresentation.EscapeContent(raw)
	if d.Text != "\ufffd\ufffd" {
		t.Fatalf("Text = %q, want %q", d.Text, "\ufffd\ufffd")
	}
	// Each invalid byte maps to one cell.
	if got := d.ByteCells[0]; got[0] != 0 || got[1] != 1 {
		t.Fatalf("ByteCells[0] = %v, want [0, 1)", got)
	}
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 2 {
		t.Fatalf("ByteCells[1] = %v, want [1, 2)", got)
	}
}

// TestContentC0ControlCaretNotation verifies that C0 controls (except
// LF, CR, tab) use caret notation. ESC becomes ^[.
func TestContentC0ControlCaretNotation(t *testing.T) {
	d := safepresentation.EscapeContent([]byte("a\x1bb"))
	if d.Text != "a^[b" {
		t.Fatalf("Text = %q, want %q", d.Text, "a^[b")
	}
	// ESC byte maps to 2 cells [1, 3).
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 3 {
		t.Fatalf("ByteCells[1] = %v, want [1, 3)", got)
	}
}

// TestContentESCByteToCellMapping verifies that a match covering an ESC
// byte highlights both ^ and [ — the byte→cell map for the ESC byte
// covers both cells of the ^[ escape.
func TestContentESCByteToCellMapping(t *testing.T) {
	d := safepresentation.EscapeContent([]byte("\x1b"))
	if d.Text != "^[" {
		t.Fatalf("Text = %q, want %q", d.Text, "^[")
	}
	// The ESC byte (index 0) maps to cells [0, 2) — covering both ^ and [.
	if got := d.ByteCells[0]; got[0] != 0 || got[1] != 2 {
		t.Fatalf("ByteCells[0] = %v, want [0, 2) (must cover both ^ and [)", got)
	}
}

// TestContentBELCaretNotation verifies that BEL (0x07) becomes ^G.
func TestContentBELCaretNotation(t *testing.T) {
	d := safepresentation.EscapeContent([]byte("\x07"))
	if d.Text != "^G" {
		t.Fatalf("Text = %q, want %q", d.Text, "^G")
	}
	if got := d.ByteCells[0]; got[0] != 0 || got[1] != 2 {
		t.Fatalf("ByteCells[0] = %v, want [0, 2)", got)
	}
}

// TestContentDEL verifies that DEL (0x7f) is escaped as ^?.
func TestContentDEL(t *testing.T) {
	d := safepresentation.EscapeContent([]byte("a\x7f"))
	if d.Text != "a^?" {
		t.Fatalf("Text = %q, want %q", d.Text, "a^?")
	}
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 3 {
		t.Fatalf("ByteCells[1] = %v, want [1, 3)", got)
	}
}

// TestContentC1Control verifies that C1 controls use \u00XX-style escapes
// (6 cells) and both bytes of the UTF-8 sequence map to the same range.
func TestContentC1Control(t *testing.T) {
	// U+0085 (NEL) = 0xc2 0x85.
	d := safepresentation.EscapeContent([]byte("a\xc2\x85b"))
	if d.Text != `a\u0085b` {
		t.Fatalf("Text = %q, want %q", d.Text, `a\u0085b`)
	}
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 7 {
		t.Fatalf("ByteCells[1] = %v, want [1, 7)", got)
	}
	if got := d.ByteCells[2]; got[0] != 1 || got[1] != 7 {
		t.Fatalf("ByteCells[2] = %v, want [1, 7)", got)
	}
}

// TestContentLFNeverDisplayed verifies that LF is a line terminator and
// is never displayed. The byte→cell map maps it to the end-of-line
// position (the cell count of the display text).
func TestContentLFNeverDisplayed(t *testing.T) {
	d := safepresentation.EscapeContent([]byte("hello\n"))
	if d.Text != "hello" {
		t.Fatalf("Text = %q, want %q", d.Text, "hello")
	}
	// LF (byte 5) maps to end-of-line position [5, 5).
	if got := d.ByteCells[5]; got[0] != 5 || got[1] != 5 {
		t.Fatalf("ByteCells[5] = %v, want [5, 5) (end-of-line)", got)
	}
}

// TestContentCRLFNeverDisplayed verifies that CRLF is a line terminator
// and both bytes map to the end-of-line position.
func TestContentCRLFNeverDisplayed(t *testing.T) {
	d := safepresentation.EscapeContent([]byte("hi\r\n"))
	if d.Text != "hi" {
		t.Fatalf("Text = %q, want %q", d.Text, "hi")
	}
	// CR (byte 2) and LF (byte 3) both map to end-of-line [2, 2).
	if got := d.ByteCells[2]; got[0] != 2 || got[1] != 2 {
		t.Fatalf("ByteCells[2] (CR) = %v, want [2, 2) (end-of-line)", got)
	}
	if got := d.ByteCells[3]; got[0] != 2 || got[1] != 2 {
		t.Fatalf("ByteCells[3] (LF) = %v, want [2, 2) (end-of-line)", got)
	}
}

// TestContentStandaloneCR verifies that a standalone CR (not followed by
// LF) is escaped as ^M (caret notation), not treated as a terminator.
func TestContentStandaloneCR(t *testing.T) {
	d := safepresentation.EscapeContent([]byte("a\rb"))
	if d.Text != "a^Mb" {
		t.Fatalf("Text = %q, want %q", d.Text, "a^Mb")
	}
	// CR (byte 1) maps to 2 cells [1, 3).
	if got := d.ByteCells[1]; got[0] != 1 || got[1] != 3 {
		t.Fatalf("ByteCells[1] = %v, want [1, 3)", got)
	}
}

// TestContentTabPlaceholder verifies that tab renders as a single →
// placeholder cell. The byte→cell map covers exactly 1 cell. No specific
// cell position is asserted (deferred to Issue #16).
func TestContentTabPlaceholder(t *testing.T) {
	d := safepresentation.EscapeContent([]byte("a\tb"))
	if d.Text != "a→b" {
		t.Fatalf("Text = %q, want %q", d.Text, "a→b")
	}
	// Tab byte maps to exactly 1 cell.
	got := d.ByteCells[1]
	if got[1]-got[0] != 1 {
		t.Fatalf("Tab ByteCells = %v, want width 1 cell", got)
	}
}

// TestContentPreservePrintableUnicode verifies that valid printable
// Unicode passes through in content.
func TestContentPreservePrintableUnicode(t *testing.T) {
	d := safepresentation.EscapeContent([]byte("héllo"))
	if d.Text != "héllo" {
		t.Fatalf("Text = %q, want %q", d.Text, "héllo")
	}
}

// TestContentMixed verifies content with multiple escape types.
func TestContentMixed(t *testing.T) {
	raw := []byte("a\x1b\t\xff\n")
	d := safepresentation.EscapeContent(raw)
	// a ^[ → \xff
	want := "a^[→\ufffd"
	if d.Text != want {
		t.Fatalf("Text = %q, want %q", d.Text, want)
	}
	// 'a' → [0, 1), ESC → [1, 3), tab → [3, 4), \xff → [4, 5), \n → [5, 5)
	wantCells := []cellRange{cr(0, 1), cr(1, 3), cr(3, 4), cr(4, 5), cr(5, 5)}
	if len(d.ByteCells) != len(wantCells) {
		t.Fatalf("ByteCells len = %d, want %d", len(d.ByteCells), len(wantCells))
	}
	for i, want := range wantCells {
		if d.ByteCells[i] != want {
			t.Fatalf("ByteCells[%d] = %v, want %v", i, d.ByteCells[i], want)
		}
	}
}

// TestContentEmpty verifies that empty content produces empty output.
func TestContentEmpty(t *testing.T) {
	d := safepresentation.EscapeContent(nil)
	if d.Text != "" {
		t.Fatalf("Text = %q, want empty", d.Text)
	}
	if len(d.ByteCells) != 0 {
		t.Fatalf("ByteCells len = %d, want 0", len(d.ByteCells))
	}
}

// TestContentByteCellsLenMatchesInput verifies that the byte→cell map has
// one entry per input byte.
func TestContentByteCellsLenMatchesInput(t *testing.T) {
	raw := []byte("hello\x1b\xff\n")
	d := safepresentation.EscapeContent(raw)
	if len(d.ByteCells) != len(raw) {
		t.Fatalf("ByteCells len = %d, want %d (one per input byte)", len(d.ByteCells), len(raw))
	}
}

// TestContentOSCSequence verifies that an OSC sequence (ESC ] 0 ; x BEL)
// is fully escaped — no raw control bytes survive in the display.
func TestContentOSCSequence(t *testing.T) {
	raw := []byte("\x1b]0;pwned\x07")
	d := safepresentation.EscapeContent(raw)
	if d.Text != "^[]0;pwned^G" {
		t.Fatalf("Text = %q, want %q", d.Text, "^[]0;pwned^G")
	}
	// No raw ESC or BEL in the display text.
	if !containsAllEscaped(d.Text, []byte{0x1b, 0x07}) {
		t.Fatalf("display text contains raw control bytes: %q", d.Text)
	}
}

// TestContentCSISequence verifies that a CSI sequence (ESC [ 2 J) is
// fully escaped — ESC becomes ^[, and the remaining printable bytes
// pass through, so the display is ^[[2J with no raw control bytes.
func TestContentCSISequence(t *testing.T) {
	raw := []byte("\x1b[2J")
	d := safepresentation.EscapeContent(raw)
	if d.Text != "^[[2J" {
		t.Fatalf("Text = %q, want %q", d.Text, "^[[2J")
	}
	// No raw ESC in the display.
	for i := 0; i < len(d.Text); i++ {
		if d.Text[i] == 0x1b {
			t.Fatalf("raw ESC byte in display: %q", d.Text)
		}
	}
}

// --- Sink-safety: no raw control bytes in display ---

// containsAllEscaped reports whether the display text contains none of
// the raw control bytes from the fixture.
func containsAllEscaped(display string, controls []byte) bool {
	for _, c := range controls {
		for i := 0; i < len(display); i++ {
			if display[i] == c {
				return false
			}
		}
	}
	return true
}

// TestPathNoRawControls verifies that no raw C0, C1, or DEL bytes survive
// in the escaped path display for a hostile fixture.
func TestPathNoRawControls(t *testing.T) {
	fixtures := [][]byte{
		[]byte("\x1b]0;x\x07"),   // OSC
		[]byte("\x1b[2J"),        // CSI
		[]byte("\x07\x08\x1b"),   // C0
		[]byte("\xc2\x85"),       // C1 (NEL)
		[]byte("\x7f"),           // DEL
		[]byte("\r"),             // standalone CR
		[]byte("foo\xff\xfebar"), // invalid UTF-8
		[]byte("file\nname"),     // embedded newline
	}
	for _, fx := range fixtures {
		d := safepresentation.EscapePath(fx)
		for i := 0; i < len(d.Text); i++ {
			b := d.Text[i]
			if b < 0x20 || b == 0x7f {
				t.Fatalf("raw control byte 0x%02x in path display for %q: %q", b, fx, d.Text)
			}
		}
	}
}

// TestContentNoRawControls verifies that no raw C0, C1, or DEL bytes
// survive in the escaped content display for a hostile fixture. LF/CRLF
// are terminators and are also absent from the display text.
func TestContentNoRawControls(t *testing.T) {
	fixtures := [][]byte{
		[]byte("\x1b]0;x\x07"),   // OSC
		[]byte("\x1b[2J"),        // CSI
		[]byte("\x07\x08\x1b"),   // C0
		[]byte("\xc2\x85"),       // C1 (NEL)
		[]byte("\x7f"),           // DEL
		[]byte("\r"),             // standalone CR
		[]byte("foo\xff\xfebar"), // invalid UTF-8
		[]byte("line1\nline2"),   // LF
		[]byte("line1\r\nline2"), // CRLF
	}
	for _, fx := range fixtures {
		d := safepresentation.EscapeContent(fx)
		for i := 0; i < len(d.Text); i++ {
			b := d.Text[i]
			if b < 0x20 || b == 0x7f {
				t.Fatalf("raw control byte 0x%02x in content display for %q: %q", b, fx, d.Text)
			}
		}
	}
}

// --- Diagnostic escaping tests (Issue #6) ---

// TestEscapeDiagnosticPreservesLFLineBoundaries verifies that LF in a
// diagnostic is preserved as a real line boundary, not escaped.
func TestEscapeDiagnosticPreservesLFLineBoundaries(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte("line1\nline2"))
	if got != "line1\nline2" {
		t.Fatalf("EscapeDiagnostic = %q, want %q", got, "line1\nline2")
	}
}

// TestEscapeDiagnosticPreservesCRLFLineBoundaries verifies that CRLF is
// preserved as a line boundary: the CR is consumed and the LF is
// preserved, so the line boundary stays without a raw CR control byte.
func TestEscapeDiagnosticPreservesCRLFLineBoundaries(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte("line1\r\nline2"))
	if got != "line1\nline2" {
		t.Fatalf("EscapeDiagnostic = %q, want %q", got, "line1\nline2")
	}
}

// TestEscapeDiagnosticExpandsTabs verifies that tabs are expanded to
// 8-column stops (spaces), with the column counter resetting at each
// newline.
func TestEscapeDiagnosticExpandsTabs(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte("a\tb"))
	// 'a' is at column 0, tab advances to column 8, so 7 spaces.
	if got != "a       b" {
		t.Fatalf("EscapeDiagnostic(%q) = %q, want %q", "a\tb", got, "a       b")
	}
}

// TestEscapeDiagnosticExpandsTabsToColumn8 verifies that a tab at
// column 0 expands to 8 spaces.
func TestEscapeDiagnosticExpandsTabsToColumn8(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte("\tx"))
	if got != "        x" {
		t.Fatalf("EscapeDiagnostic(%q) = %q, want %q", "\tx", got, "        x")
	}
}

// TestEscapeDiagnosticTabResetsAfterNewline verifies that the tab column
// counter resets at each newline.
func TestEscapeDiagnosticTabResetsAfterNewline(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte("aaaaa\tb\n\tx"))
	// Line 1: 'a'*5 at columns 0-4, tab at col 5 → col 8 (3 spaces), 'b'.
	// Line 2: tab at col 0 → col 8 (8 spaces), 'x'.
	want := "aaaaa   b\n        x"
	if got != want {
		t.Fatalf("EscapeDiagnostic(%q) = %q, want %q", "aaaaa\tb\n\tx", got, want)
	}
}

// TestEscapeDiagnosticC0Controls verifies that C0 controls (except LF)
// use caret notation. ESC becomes ^[, BEL becomes ^G.
func TestEscapeDiagnosticC0Controls(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte("a\x1bb"))
	if got != "a^[b" {
		t.Fatalf("EscapeDiagnostic = %q, want %q", got, "a^[b")
	}
	got = safepresentation.EscapeDiagnostic([]byte("a\x07b"))
	if got != "a^Gb" {
		t.Fatalf("EscapeDiagnostic = %q, want %q", got, "a^Gb")
	}
}

// TestEscapeDiagnosticDEL verifies that DEL (0x7f) is escaped as ^?.
func TestEscapeDiagnosticDEL(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte("a\x7fb"))
	if got != "a^?b" {
		t.Fatalf("EscapeDiagnostic = %q, want %q", got, "a^?b")
	}
}

// TestEscapeDiagnosticC1Control verifies that C1 controls use \u00XX
// escapes.
func TestEscapeDiagnosticC1Control(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte("a\xc2\x85b"))
	if got != `a\u0085b` {
		t.Fatalf("EscapeDiagnostic = %q, want %q", got, `a\u0085b`)
	}
}

// TestEscapeDiagnosticInvalidUTF8 verifies that invalid UTF-8 bytes
// are escaped as \xNN.
func TestEscapeDiagnosticInvalidUTF8(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte("a\xffb"))
	if got != `a\xffb` {
		t.Fatalf("EscapeDiagnostic = %q, want %q", got, `a\xffb`)
	}
}

// TestEscapeDiagnosticStandaloneCR verifies that a standalone CR (not
// followed by LF) is escaped as ^M, not treated as a line boundary.
func TestEscapeDiagnosticStandaloneCR(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte("a\rb"))
	if got != "a^Mb" {
		t.Fatalf("EscapeDiagnostic = %q, want %q", got, "a^Mb")
	}
}

// TestEscapeDiagnosticNoBackslashEscape verifies that backslashes are
// not escaped in diagnostics. The caller escapes embedded filenames
// through EscapePath first; EscapeDiagnostic must not double-escape
// the already-escaped filename.
func TestEscapeDiagnosticNoBackslashEscape(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte(`a\nb`))
	if got != `a\nb` {
		t.Fatalf("EscapeDiagnostic = %q, want %q (backslash must not be escaped)", got, `a\nb`)
	}
	got = safepresentation.EscapeDiagnostic([]byte(`a\\b`))
	if got != `a\\b` {
		t.Fatalf("EscapeDiagnostic = %q, want %q (backslash must not be escaped)", got, `a\\b`)
	}
}

// TestEscapeDiagnosticPreservesPrintableUnicode verifies that valid
// printable Unicode passes through unchanged.
func TestEscapeDiagnosticPreservesPrintableUnicode(t *testing.T) {
	got := safepresentation.EscapeDiagnostic([]byte("café 文件"))
	if got != "café 文件" {
		t.Fatalf("EscapeDiagnostic = %q, want %q", got, "café 文件")
	}
}

// TestEscapeDiagnosticEmpty verifies that empty input produces empty
// output.
func TestEscapeDiagnosticEmpty(t *testing.T) {
	got := safepresentation.EscapeDiagnostic(nil)
	if got != "" {
		t.Fatalf("EscapeDiagnostic(nil) = %q, want empty", got)
	}
}

// TestEscapeDiagnosticSingleLinedFilename verifies that a filename
// embedded in a diagnostic is first escaped through EscapePath (making
// it single-line), then the diagnostic is escaped through
// EscapeDiagnostic. The composition must not double-escape: the \n
// from EscapePath passes through EscapeDiagnostic unchanged because
// EscapeDiagnostic does not escape backslashes.
func TestEscapeDiagnosticSingleLinedFilename(t *testing.T) {
	rawFilename := []byte("file\nname")
	escapedName := safepresentation.EscapePath(rawFilename)
	// escapedName.Text is "file\nname" (with literal backslash-n).
	diag := "oversized record skipped for " + escapedName.Text
	got := safepresentation.EscapeDiagnostic([]byte(diag))
	// The diagnostic should contain the escaped filename with \n (not
	// a real newline) and no raw control bytes.
	if strings.Contains(got, "\n") {
		t.Fatalf("diagnostic contains a real newline from the filename: %q", got)
	}
	if !strings.Contains(got, `\n`) {
		t.Fatalf("diagnostic does not contain the escaped newline \\n: %q", got)
	}
}

// TestEscapeDiagnosticNoRawControls verifies that no raw C0, C1, or DEL
// bytes survive in the escaped diagnostic, except LF which is a
// preserved line boundary.
func TestEscapeDiagnosticNoRawControls(t *testing.T) {
	fixtures := [][]byte{
		[]byte("\x1b]0;x\x07"),   // OSC
		[]byte("\x1b[2J"),        // CSI
		[]byte("\x07\x08\x1b"),   // C0
		[]byte("\xc2\x85"),       // C1 (NEL)
		[]byte("\x7f"),           // DEL
		[]byte("\r"),             // standalone CR
		[]byte("foo\xff\xfebar"), // invalid UTF-8
		[]byte("file\nname"),     // embedded newline (LF preserved)
	}
	for _, fx := range fixtures {
		got := safepresentation.EscapeDiagnostic(fx)
		for i := 0; i < len(got); i++ {
			b := got[i]
			if b == '\n' {
				continue // LF is a preserved line boundary.
			}
			if b < 0x20 || b == 0x7f {
				t.Fatalf("raw control byte 0x%02x in diagnostic for %q: %q", b, fx, got)
			}
		}
	}
}

// --- Shared sink-safety table tests (Issue #6) ---

// TestSinkSafetyTableEscapePath verifies that the path sink (EscapePath)
// produces no raw control bytes for every shared fixture, rendered
// through the no-style composition path (EscapePath produces no ANSI
// sequences, so any control byte is unsanitized external data).
func TestSinkSafetyTableEscapePath(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			d := safepresentation.EscapePath(fx.Raw)
			if !sinkfixtures.NoControlBytes(d.Text) {
				t.Fatalf("raw control byte in path display for %s: %q", fx.Name, d.Text)
			}
		})
	}
}

// TestSinkSafetyTableEscapeContent verifies that the content sink
// (EscapeContent) produces no raw control bytes for every shared
// fixture, rendered through the no-style composition path.
func TestSinkSafetyTableEscapeContent(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			d := safepresentation.EscapeContent(fx.Raw)
			if !sinkfixtures.NoControlBytes(d.Text) {
				t.Fatalf("raw control byte in content display for %s: %q", fx.Name, d.Text)
			}
		})
	}
}

// TestSinkSafetyTableEscapeDiagnostic verifies that the diagnostic sink
// (EscapeDiagnostic) produces no raw control bytes (except preserved
// LF) for every shared fixture.
func TestSinkSafetyTableEscapeDiagnostic(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			got := safepresentation.EscapeDiagnostic(fx.Raw)
			if !sinkfixtures.NoControlBytes(got) {
				t.Fatalf("raw control byte in diagnostic for %s: %q", fx.Name, got)
			}
		})
	}
}
