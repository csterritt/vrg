package filebuffer_test

import (
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// --- Unsupported-encoding detection (Issue #30) ---
//
// These tests cover the Issue #30 contracts: detection of UTF-16
// LE/BE and UTF-32 LE/BE BOMs with longer BOMs checked before
// overlapping shorter ones (so FF FE 00 00 classifies as UTF-32 LE
// rather than UTF-16 LE), a UTF-8 BOM not misclassified, a detected
// file showing "(unsupported encoding)" with no file text, no
// highlights, and an explanatory diagnostic, while remaining an
// indexed cursor stop that r can reload. The stale-match guard
// (Issue #29) does not run against these raw encoded bytes.

// TestUnsupportedUTF16LEBOM verifies that a file beginning with the
// UTF-16 LE BOM (FF FE) is detected as an unsupported encoding with
// no file text, no highlights, and an explanatory diagnostic.
func TestUnsupportedUTF16LEBOM(t *testing.T) {
	dir := t.TempDir()
	// FF FE is the UTF-16 LE BOM; "hi" in UTF-16 LE is h\0i\0.
	p := writeFile(t, dir, "utf16le.txt", []byte{0xFF, 0xFE, 'h', 0x00, 'i', 0x00, '\n', 0x00})
	stops := []searchindex.Stop{
		mkStop("utf16le.txt", 1, "hi\n", sm("hi", 0, 2)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.UnsupportedEncoding {
		t.Fatalf("UnsupportedEncoding = false, want true for UTF-16 LE BOM")
	}
	if buf.EncodingDiagnostic == "" {
		t.Fatalf("EncodingDiagnostic is empty, want an explanatory diagnostic")
	}
	if !strings.Contains(buf.EncodingDiagnostic, "UTF-16") {
		t.Fatalf("EncodingDiagnostic = %q, want it to mention UTF-16", buf.EncodingDiagnostic)
	}
	if buf.LineCount != 0 {
		t.Fatalf("LineCount = %d, want 0 (no file text for unsupported encoding)", buf.LineCount)
	}
	if len(buf.Lines) != 0 {
		t.Fatalf("Lines len = %d, want 0 (no file text for unsupported encoding)", len(buf.Lines))
	}
}

// TestUnsupportedUTF16BEBOM verifies that a file beginning with the
// UTF-16 BE BOM (FE FF) is detected as an unsupported encoding.
func TestUnsupportedUTF16BEBOM(t *testing.T) {
	dir := t.TempDir()
	// FE FF is the UTF-16 BE BOM; "hi" in UTF-16 BE is \0h\0i.
	p := writeFile(t, dir, "utf16be.txt", []byte{0xFE, 0xFF, 0x00, 'h', 0x00, 'i', 0x00, 0x0A})
	stops := []searchindex.Stop{
		mkStop("utf16be.txt", 1, "hi\n", sm("hi", 0, 2)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.UnsupportedEncoding {
		t.Fatalf("UnsupportedEncoding = false, want true for UTF-16 BE BOM")
	}
	if !strings.Contains(buf.EncodingDiagnostic, "UTF-16") {
		t.Fatalf("EncodingDiagnostic = %q, want it to mention UTF-16", buf.EncodingDiagnostic)
	}
	if buf.LineCount != 0 {
		t.Fatalf("LineCount = %d, want 0", buf.LineCount)
	}
}

// TestUnsupportedUTF32LEBOM verifies that a file beginning with the
// UTF-32 LE BOM (FF FE 00 00) is detected as UTF-32 LE, not UTF-16 LE.
// This is the critical overlap-ordering test: FF FE is both the
// UTF-16 LE BOM and the first two bytes of the UTF-32 LE BOM. Longer
// BOMs must be checked before overlapping shorter ones.
func TestUnsupportedUTF32LEBOM(t *testing.T) {
	dir := t.TempDir()
	// FF FE 00 00 is the UTF-32 LE BOM.
	p := writeFile(t, dir, "utf32le.txt", []byte{0xFF, 0xFE, 0x00, 0x00, 'h', 0x00, 0x00, 0x00, 'i', 0x00, 0x00, 0x00})
	stops := []searchindex.Stop{
		mkStop("utf32le.txt", 1, "hi", sm("hi", 0, 2)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.UnsupportedEncoding {
		t.Fatalf("UnsupportedEncoding = false, want true for UTF-32 LE BOM")
	}
	if !strings.Contains(buf.EncodingDiagnostic, "UTF-32") {
		t.Fatalf("EncodingDiagnostic = %q, want it to mention UTF-32 (not UTF-16)", buf.EncodingDiagnostic)
	}
	if strings.Contains(buf.EncodingDiagnostic, "UTF-16") {
		t.Fatalf("EncodingDiagnostic = %q, should not mention UTF-16 for a UTF-32 LE file (overlap ordering)", buf.EncodingDiagnostic)
	}
}

// TestUnsupportedUTF32BEBOM verifies that a file beginning with the
// UTF-32 BE BOM (00 00 FE FF) is detected as an unsupported encoding.
func TestUnsupportedUTF32BEBOM(t *testing.T) {
	dir := t.TempDir()
	// 00 00 FE FF is the UTF-32 BE BOM.
	p := writeFile(t, dir, "utf32be.txt", []byte{0x00, 0x00, 0xFE, 0xFF, 0x00, 0x00, 0x00, 'h', 0x00, 0x00, 0x00, 'i'})
	stops := []searchindex.Stop{
		mkStop("utf32be.txt", 1, "hi", sm("hi", 0, 2)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.UnsupportedEncoding {
		t.Fatalf("UnsupportedEncoding = false, want true for UTF-32 BE BOM")
	}
	if !strings.Contains(buf.EncodingDiagnostic, "UTF-32") {
		t.Fatalf("EncodingDiagnostic = %q, want it to mention UTF-32", buf.EncodingDiagnostic)
	}
}

// TestUTF8BOMNotMisclassified verifies that a file beginning with the
// UTF-8 BOM (EF BB BF) is NOT detected as an unsupported encoding.
// UTF-8 BOMs are supported and invisible at the beginning of a file.
func TestUTF8BOMNotMisclassified(t *testing.T) {
	dir := t.TempDir()
	content := []byte{0xEF, 0xBB, 0xBF, 'h', 'e', 'l', 'l', 'o', '\n'}
	p := writeFile(t, dir, "utf8bom.txt", content)
	stops := []searchindex.Stop{
		mkStop("utf8bom.txt", 1, "hello\n", sm("hello", 0, 5)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.UnsupportedEncoding {
		t.Fatalf("UnsupportedEncoding = true, want false (UTF-8 BOM is supported)")
	}
	if buf.EncodingDiagnostic != "" {
		t.Fatalf("EncodingDiagnostic = %q, want empty for UTF-8 BOM", buf.EncodingDiagnostic)
	}
	if buf.LineCount != 1 {
		t.Fatalf("LineCount = %d, want 1 (UTF-8 BOM file is supported)", buf.LineCount)
	}
	if len(buf.Lines) != 1 {
		t.Fatalf("Lines len = %d, want 1", len(buf.Lines))
	}
	if buf.Lines[0].Display != "hello" {
		t.Fatalf("Display = %q, want %q (UTF-8 BOM stripped, content visible)", buf.Lines[0].Display, "hello")
	}
}

// TestUnsupportedNoHighlights verifies that a detected unsupported
// encoding produces no highlight spans, even when stops with
// submatches are provided. The file remains an indexed cursor stop
// (the stops are passed to Load), but no file text or highlights are
// produced.
func TestUnsupportedNoHighlights(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "utf16.txt", []byte{0xFF, 0xFE, 'h', 0x00, 'i', 0x00})
	stops := []searchindex.Stop{
		mkStop("utf16.txt", 1, "hi", sm("hi", 0, 2)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.UnsupportedEncoding {
		t.Fatalf("UnsupportedEncoding = false, want true")
	}
	for i, line := range buf.Lines {
		if len(line.Highlights) != 0 {
			t.Fatalf("Line %d has %d highlights, want 0 (no highlights for unsupported encoding)", i, len(line.Highlights))
		}
	}
}

// TestUnsupportedNoStaleValidation verifies that the stale-match guard
// (Issue #29) does not run against the raw encoded bytes of an
// unsupported-encoding file. Even with submatches that would fail
// validation against any text, Stale remains false because
// validation is excluded for these files.
func TestUnsupportedNoStaleValidation(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "utf16.txt", []byte{0xFF, 0xFE, 'h', 0x00, 'i', 0x00})
	// Submatches with ranges that would fail validation if the guard
	// ran against the raw UTF-16 bytes.
	stops := []searchindex.Stop{
		mkStop("utf16.txt", 1, "hi", sm("hi", 0, 100)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.UnsupportedEncoding {
		t.Fatalf("UnsupportedEncoding = false, want true")
	}
	if buf.Stale {
		t.Fatalf("Stale = true, want false (stale validation excluded for unsupported encodings)")
	}
}

// TestUnsupportedReloadPreservesPlaceholder verifies that reloading
// (calling Load again) re-detects the BOM and returns the same
// unsupported-encoding placeholder. The file remains reloadable: the
// placeholder is preserved when the content is unchanged.
func TestUnsupportedReloadPreservesPlaceholder(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "utf16.txt", []byte{0xFF, 0xFE, 'h', 0x00, 'i', 0x00})
	stops := []searchindex.Stop{
		mkStop("utf16.txt", 1, "hi", sm("hi", 0, 2)),
	}
	buf1, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf1.UnsupportedEncoding {
		t.Fatalf("first Load: UnsupportedEncoding = false, want true")
	}
	// Reload: call Load again on the same file.
	buf2, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Reload Load error: %v", err)
	}
	if !buf2.UnsupportedEncoding {
		t.Fatalf("reload Load: UnsupportedEncoding = false, want true (placeholder preserved on reload)")
	}
	if buf2.EncodingDiagnostic != buf1.EncodingDiagnostic {
		t.Fatalf("reload diagnostic changed: %q vs %q", buf2.EncodingDiagnostic, buf1.EncodingDiagnostic)
	}
}

// TestUnsupportedGutterWidth verifies that an unsupported-encoding
// buffer has a usable gutter width (at least the minimum 1 digit + 2
// spaces) so the filename row and layout do not produce negative
// dimensions.
func TestUnsupportedGutterWidth(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "utf16.txt", []byte{0xFF, 0xFE, 'h', 0x00})
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.UnsupportedEncoding {
		t.Fatalf("UnsupportedEncoding = false, want true")
	}
	if buf.GutterWidth < 3 {
		t.Fatalf("GutterWidth = %d, want >= 3 (1 digit + 2 spaces minimum)", buf.GutterWidth)
	}
}

// TestUnsupportedNoBOMNotUnsupported verifies that a file without any
// BOM is not detected as an unsupported encoding.
func TestUnsupportedNoBOMNotUnsupported(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "plain.txt", []byte("hello\nworld\n"))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.UnsupportedEncoding {
		t.Fatalf("UnsupportedEncoding = true, want false (no BOM)")
	}
}

// TestUnsupportedShortFileNoFalsePositive verifies that a file too
// short to contain any BOM is not misclassified.
func TestUnsupportedShortFileNoFalsePositive(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "short.txt", []byte("a"))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.UnsupportedEncoding {
		t.Fatalf("UnsupportedEncoding = true, want false (file too short for any BOM)")
	}
}

// TestUnsupportedUTF16LETwoByteOnly verifies that a file with exactly
// the two-byte UTF-16 LE BOM (FF FE) and nothing else is detected as
// UTF-16 LE, not UTF-32 LE (which requires four bytes FF FE 00 00).
func TestUnsupportedUTF16LETwoByteOnly(t *testing.T) {
	dir := t.TempDir()
	// FF FE followed by non-zero bytes so it is not UTF-32 LE.
	p := writeFile(t, dir, "utf16le.txt", []byte{0xFF, 0xFE, 'h', 0x00})
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.UnsupportedEncoding {
		t.Fatalf("UnsupportedEncoding = false, want true for UTF-16 LE BOM")
	}
	if !strings.Contains(buf.EncodingDiagnostic, "UTF-16") {
		t.Fatalf("EncodingDiagnostic = %q, want it to mention UTF-16", buf.EncodingDiagnostic)
	}
	if strings.Contains(buf.EncodingDiagnostic, "UTF-32") {
		t.Fatalf("EncodingDiagnostic = %q, should not mention UTF-32 for a two-byte FF FE file", buf.EncodingDiagnostic)
	}
}
