package safepresentation_test

import (
	"testing"

	"vrg/internal/safepresentation"
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
