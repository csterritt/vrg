package filebuffer_test

import (
	"bytes"
	"os"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// --- Stale-match validation (Issue #29) ---
//
// These tests cover the Issue #29 contracts: each submatch is checked
// on first load and every reload for line existence, range validity
// against original line bytes including terminators with the Issue #22
// UTF-8 BOM adjustment, and byte equality with the recorded submatch
// bytes (accepting both JSON encodings, already decoded by
// searchindex). Any failure drops that submatch, marks the buffer
// stale, and keeps other valid highlights. A stale stop with surviving
// submatches exposes the first survivor as the reveal target. A stop
// whose line exists but has no survivors exposes the first recorded
// start clamped to the available line bytes and mapped to a valid
// display cell, with the end-of-line fallback clamped to the last
// rendered cell when there is no marker cell. A missing line lands at
// the last source line's start. An empty file remains a zero-line
// panel. Reload recomputes staleness so the note clears only on fully
// validating content. Validation compares against original and search
// bytes rather than stripped display text, with a CRLF terminator
// match still validating.

// TestStaleValidSubmatchNotStale verifies that a file whose content
// matches the recorded search bytes is not stale and keeps its
// highlight.
func TestStaleValidSubmatchNotStale(t *testing.T) {
	dir := t.TempDir()
	content := "hello\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("hello", 0, 5)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.Stale {
		t.Fatalf("Stale = true, want false (content matches search bytes)")
	}
	if len(buf.Lines[0].Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1 (valid submatch retained)", len(buf.Lines[0].Highlights))
	}
}

// TestStaleNoStopsNotStale verifies that a file with no stops is not
// stale.
func TestStaleNoStopsNotStale(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.txt", []byte("hello\n"))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.Stale {
		t.Fatalf("Stale = true, want false (no stops to validate)")
	}
}

// TestStaleOutOfBoundsSubmatchDropped verifies that a submatch whose
// End exceeds the original line bytes is dropped, the buffer is marked
// stale, and no highlight is produced for it.
func TestStaleOutOfBoundsSubmatchDropped(t *testing.T) {
	dir := t.TempDir()
	content := "hit\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// End=10 exceeds len("hit\n")=4.
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("hit", 0, 10)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true (out-of-bounds submatch dropped)")
	}
	if len(buf.Lines[0].Highlights) != 0 {
		t.Fatalf("Highlights len = %d, want 0 (out-of-bounds submatch dropped)", len(buf.Lines[0].Highlights))
	}
}

// TestStaleNegativeStartDropped verifies that a submatch with a
// negative Start is dropped and the buffer is stale.
func TestStaleNegativeStartDropped(t *testing.T) {
	dir := t.TempDir()
	content := "hit\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("hit", -1, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true (negative start)")
	}
	if len(buf.Lines[0].Highlights) != 0 {
		t.Fatalf("Highlights len = %d, want 0 (negative-start submatch dropped)", len(buf.Lines[0].Highlights))
	}
}

// TestStaleSameLengthReplacementDropped verifies that when the matched
// text is replaced on disk with a same-length different word, byte
// equality fails, the submatch is dropped, and the buffer is stale.
// This is the undetectable-by-display case: the display width is
// unchanged but the bytes differ.
func TestStaleSameLengthReplacementDropped(t *testing.T) {
	dir := t.TempDir()
	// Disk has "hat\n" but the search recorded "hit".
	p := writeFile(t, dir, "test.txt", []byte("hat\n"))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, "hit\n", sm("hit", 0, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true (same-length replacement detected by byte equality)")
	}
	if len(buf.Lines[0].Highlights) != 0 {
		t.Fatalf("Highlights len = %d, want 0 (mismatched bytes dropped)", len(buf.Lines[0].Highlights))
	}
}

// TestStalePartialSurvival verifies that when a line has multiple
// submatches and only some validate, the valid ones keep their
// highlights, the invalid ones are dropped, and the buffer is stale.
func TestStalePartialSurvival(t *testing.T) {
	dir := t.TempDir()
	// Disk: "hit hat\n". Search recorded "hit" (valid) and "cat"
	// (invalid: disk has "hat" at bytes 4-7).
	content := "hit hat\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content,
			sm("hit", 0, 3), // valid: disk[0:3]="hit"
			sm("cat", 4, 7), // invalid: disk[4:7]="hat"
		),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true (one submatch dropped)")
	}
	line := buf.Lines[0]
	if !hasHighlight(line.Highlights, 0, 3) {
		t.Fatalf("Highlights = %v, want a [0, 3) survivor for 'hit'", line.Highlights)
	}
	if hasHighlight(line.Highlights, 4, 7) {
		t.Fatalf("Highlights = %v, want no [4, 7) (invalid 'cat' dropped)", line.Highlights)
	}
}

// TestStalePartialSurvivalRevealTarget verifies that a stale stop with
// surviving submatches exposes the first survivor as the reveal
// target. The first recorded submatch is dropped; the second survives
// and becomes the reveal target.
func TestStalePartialSurvivalRevealTarget(t *testing.T) {
	dir := t.TempDir()
	// Disk: "xat hit\n". Search recorded "cat" (bytes 0-3, invalid:
	// disk has "xat") and "hit" (bytes 4-7, valid).
	content := "xat hit\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content,
			sm("cat", 0, 3), // invalid: disk[0:3]="xat"
			sm("hit", 4, 7), // valid: disk[4:7]="hit"
		),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true")
	}
	lineIdx, byteStart, cell := buf.RevealTarget(stops[0])
	if lineIdx != 0 {
		t.Fatalf("RevealTarget lineIdx = %d, want 0", lineIdx)
	}
	// First survivor is "hit" at byte 4.
	if byteStart != 4 {
		t.Fatalf("RevealTarget byteStart = %d, want 4 (first survivor 'hit')", byteStart)
	}
	// Byte 4 maps to display cell 4.
	if cell != 4 {
		t.Fatalf("RevealTarget cell = %d, want 4", cell)
	}
}

// TestStaleClampedStartFallback verifies that a stop whose line exists
// but has no surviving submatches exposes the first recorded start
// clamped to the available line bytes. The reveal cell maps to a valid
// display cell.
func TestStaleClampedStartFallback(t *testing.T) {
	dir := t.TempDir()
	// Disk: "abc\n". Search recorded "xyz" at bytes 0-3 (mismatch).
	content := "abc\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("xyz", 0, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true (all submatches dropped)")
	}
	lineIdx, byteStart, cell := buf.RevealTarget(stops[0])
	if lineIdx != 0 {
		t.Fatalf("RevealTarget lineIdx = %d, want 0", lineIdx)
	}
	// First recorded start is 0, clamped to [0, len("abc\n")=4] → 0.
	if byteStart != 0 {
		t.Fatalf("RevealTarget byteStart = %d, want 0 (clamped first recorded start)", byteStart)
	}
	// Byte 0 maps to display cell 0.
	if cell != 0 {
		t.Fatalf("RevealTarget cell = %d, want 0", cell)
	}
}

// TestStaleClampedStartEOLFallback verifies that when the clamped
// first recorded start maps to the end-of-line position and there is
// no marker cell, the reveal cell clamps to the last rendered cell.
func TestStaleClampedStartEOLFallback(t *testing.T) {
	dir := t.TempDir()
	// Disk: "ab\n" (3 bytes: a, b, \n). Search recorded a submatch at
	// byte 2 (the \n terminator) that does not match (e.g. "X").
	content := "ab\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("X", 2, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true")
	}
	lineIdx, byteStart, cell := buf.RevealTarget(stops[0])
	if lineIdx != 0 {
		t.Fatalf("RevealTarget lineIdx = %d, want 0", lineIdx)
	}
	// First recorded start is 2, clamped to [0, 3] → 2 (the \n byte).
	if byteStart != 2 {
		t.Fatalf("RevealTarget byteStart = %d, want 2 (clamped to terminator byte)", byteStart)
	}
	// Byte 2 (\n) maps to the end-of-line position (content width 2).
	// No marker cell, so clamp to the last rendered cell (2-1 = 1).
	if cell != 1 {
		t.Fatalf("RevealTarget cell = %d, want 1 (EOL fallback to last rendered cell, no marker)", cell)
	}
}

// TestStaleClampedStartEOLWithMarker verifies that when the clamped
// first recorded start maps to the end-of-line position and there IS a
// marker cell (from a surviving zero-width submatch), the reveal cell
// is the marker cell, not clamped to the last content cell.
func TestStaleClampedStartEOLWithMarker(t *testing.T) {
	dir := t.TempDir()
	// Disk: "ab\n". Two submatches: a zero-width at byte 0 (valid,
	// produces a marker at cell 0) and a non-zero-width "X" at byte 2
	// (invalid, dropped). The fallback for the dropped submatch maps
	// to EOL; since there is a marker cell (from the survivor), the
	// cell is the marker position, not clamped.
	content := "ab\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content,
			sm("", 0, 0),  // valid zero-width at byte 0 → marker at cell 0
			sm("X", 2, 3), // invalid: disk[2:3]="\n" != "X"
		),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true (one submatch dropped)")
	}
	// The first survivor is the zero-width at byte 0, so the reveal
	// target is byte 0, cell 0 (the marker). This tests that survivors
	// take precedence over the fallback.
	lineIdx, byteStart, cell := buf.RevealTarget(stops[0])
	if lineIdx != 0 {
		t.Fatalf("RevealTarget lineIdx = %d, want 0", lineIdx)
	}
	if byteStart != 0 {
		t.Fatalf("RevealTarget byteStart = %d, want 0 (first survivor is zero-width at 0)", byteStart)
	}
	if cell != 0 {
		t.Fatalf("RevealTarget cell = %d, want 0 (marker cell)", cell)
	}
}

// TestStaleClampedStartPastEnd verifies that a first recorded start
// past the end of the line bytes is clamped down to the line byte
// length.
func TestStaleClampedStartPastEnd(t *testing.T) {
	dir := t.TempDir()
	// Disk: "ab\n" (3 bytes). Search recorded "X" at bytes 5-6 (past
	// end). Start=5 clamped to len(raw)=3.
	content := "ab\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("X", 5, 6)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true")
	}
	lineIdx, byteStart, cell := buf.RevealTarget(stops[0])
	if lineIdx != 0 {
		t.Fatalf("RevealTarget lineIdx = %d, want 0", lineIdx)
	}
	// Start 5 clamped to len("ab\n")=3.
	if byteStart != 3 {
		t.Fatalf("RevealTarget byteStart = %d, want 3 (clamped to line byte length)", byteStart)
	}
	// Byte 3 is past ByteCells; EOL position is content width 2, no
	// marker → clamp to last rendered cell 1.
	if cell != 1 {
		t.Fatalf("RevealTarget cell = %d, want 1 (EOL fallback, no marker)", cell)
	}
}

// TestStaleMissingLineFallback verifies that a stop whose line is gone
// (line number exceeds the file's line count) lands at the last
// source line's start.
func TestStaleMissingLineFallback(t *testing.T) {
	dir := t.TempDir()
	// Disk: "a\nb\n" (2 lines). Stop references line 5 (missing).
	content := "a\nb\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 5, "gone\n", sm("gone", 0, 4)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true (missing line)")
	}
	lineIdx, byteStart, cell := buf.RevealTarget(stops[0])
	// Last source line is line 2 (index 1), byte 0, cell 0.
	if lineIdx != 1 {
		t.Fatalf("RevealTarget lineIdx = %d, want 1 (last source line)", lineIdx)
	}
	if byteStart != 0 {
		t.Fatalf("RevealTarget byteStart = %d, want 0 (last source line start)", byteStart)
	}
	if cell != 0 {
		t.Fatalf("RevealTarget cell = %d, want 0", cell)
	}
}

// TestStaleEmptyFileRemainsZeroLines verifies that an empty file with a
// stop remains a zero-line panel and RevealTarget returns lineIdx -1
// (no target).
func TestStaleEmptyFileRemainsZeroLines(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "empty.txt", nil)
	stops := []searchindex.Stop{
		mkStop("empty.txt", 1, "gone\n", sm("gone", 0, 4)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 0 {
		t.Fatalf("LineCount = %d, want 0 (empty file)", buf.LineCount)
	}
	if len(buf.Lines) != 0 {
		t.Fatalf("Lines len = %d, want 0 (zero-line panel)", len(buf.Lines))
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true (missing line in empty file)")
	}
	lineIdx, _, _ := buf.RevealTarget(stops[0])
	if lineIdx != -1 {
		t.Fatalf("RevealTarget lineIdx = %d, want -1 (no target in empty file)", lineIdx)
	}
}

// TestStaleReloadRecomputes verifies that reload recomputes staleness:
// after a reload with content that validates, the note clears (Stale
// becomes false) and highlights return.
func TestStaleReloadRecomputes(t *testing.T) {
	dir := t.TempDir()
	// First load: disk has "hat\n", search recorded "hit" → stale.
	p := writeFile(t, dir, "test.txt", []byte("hat\n"))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, "hit\n", sm("hit", 0, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("first load: Stale = false, want true (mismatched bytes)")
	}
	// Reload: rewrite disk to "hit\n" (matches search bytes).
	if err := os.WriteFile(p, []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf2, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("reload Load error: %v", err)
	}
	if buf2.Stale {
		t.Fatalf("reload: Stale = true, want false (content now validates)")
	}
	if len(buf2.Lines[0].Highlights) != 1 {
		t.Fatalf("reload: Highlights len = %d, want 1 (valid submatch restored)", len(buf2.Lines[0].Highlights))
	}
}

// TestStaleReloadReintroducesStaleness verifies that a reload can
// reintroduce staleness: after a valid load, rewriting the file to
// mismatch and reloading marks the buffer stale again.
func TestStaleReloadReintroducesStaleness(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.txt", []byte("hit\n"))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, "hit\n", sm("hit", 0, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.Stale {
		t.Fatalf("first load: Stale = true, want false (content matches)")
	}
	// Rewrite to mismatch and reload.
	if err := os.WriteFile(p, []byte("hat\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf2, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("reload Load error: %v", err)
	}
	if !buf2.Stale {
		t.Fatalf("reload: Stale = false, want true (content now mismatches)")
	}
}

// TestStaleValidationAgainstOriginalBytes verifies that validation
// compares against the original line bytes (including terminators),
// not the stripped display text. A submatch covering text plus a CRLF
// terminator validates because the original bytes match, even though
// the display text has the terminator stripped.
func TestStaleValidationAgainstOriginalBytes(t *testing.T) {
	dir := t.TempDir()
	content := "hit\r\n"
	p := writeFile(t, dir, "crlf.txt", []byte(content))
	// Submatch covers the entire line including CRLF: bytes 0-5.
	stops := []searchindex.Stop{
		mkStop("crlf.txt", 1, content, sm("hit\r\n", 0, 5)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.Stale {
		t.Fatalf("Stale = true, want false (original bytes including CRLF match)")
	}
	// The highlight covers only visible text [0, 3) (Issue #22), but
	// validation passed because the raw bytes match.
	if len(buf.Lines[0].Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1 (valid text+terminator span)", len(buf.Lines[0].Highlights))
	}
}

// TestStaleCRLFTerminatorOnlyMatchValidates verifies that a
// zero-width terminator-only match on a CRLF line validates against
// the original bytes. The \n byte (byte 4 of "hit\r\n") maps to the
// end-of-line position; the zero-width match (empty bytes) validates.
func TestStaleCRLFTerminatorOnlyMatchValidates(t *testing.T) {
	dir := t.TempDir()
	content := "hit\r\n"
	p := writeFile(t, dir, "crlf.txt", []byte(content))
	// Zero-width match at byte 4 (the \n of CRLF).
	stops := []searchindex.Stop{
		mkStop("crlf.txt", 1, content, sm("", 4, 4)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.Stale {
		t.Fatalf("Stale = true, want false (CRLF terminator-only match validates)")
	}
	// The marker highlight is at the EOL cell [3, 4).
	if !hasHighlight(buf.Lines[0].Highlights, 3, 4) {
		t.Fatalf("Highlights = %v, want a [3, 4) marker at EOL for CRLF terminator-only", buf.Lines[0].Highlights)
	}
}

// TestStaleLFTerminatorOnlyMatchValidates verifies that a zero-width
// terminator-only match on an LF line validates.
func TestStaleLFTerminatorOnlyMatchValidates(t *testing.T) {
	dir := t.TempDir()
	content := "hit\n"
	p := writeFile(t, dir, "lf.txt", []byte(content))
	// Zero-width match at byte 3 (the \n).
	stops := []searchindex.Stop{
		mkStop("lf.txt", 1, content, sm("", 3, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.Stale {
		t.Fatalf("Stale = true, want false (LF terminator-only match validates)")
	}
}

// TestStaleBOMFirstLineValidates verifies that a BOM file's first-line
// match validates correctly. The BOM is stripped before line splitting
// (Issue #22), so the raw first line is in rg-line coordinates and the
// rg offsets align. The match validates and the buffer is not stale.
func TestStaleBOMFirstLineValidates(t *testing.T) {
	dir := t.TempDir()
	// BOM + "hit\n" on disk.
	disk := []byte{0xEF, 0xBB, 0xBF, 'h', 'i', 't', '\n'}
	p := writeFile(t, dir, "bom.txt", disk)
	// rg reports line 1 without the BOM: "hit\n". Match at rg offset 0.
	stops := []searchindex.Stop{
		mkStop("bom.txt", 1, "hit\n", sm("hit", 0, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.Stale {
		t.Fatalf("Stale = true, want false (BOM-adjusted first-line match validates)")
	}
	if len(buf.Lines[0].Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1 (BOM first-line match valid)", len(buf.Lines[0].Highlights))
	}
}

// TestStaleBOMFirstLineMismatchDropped verifies that a BOM file's
// first-line match with mismatched bytes is dropped and the buffer is
// stale. The BOM adjustment does not mask a real mismatch.
func TestStaleBOMFirstLineMismatchDropped(t *testing.T) {
	dir := t.TempDir()
	// BOM + "hat\n" on disk, but search recorded "hit".
	disk := []byte{0xEF, 0xBB, 0xBF, 'h', 'a', 't', '\n'}
	p := writeFile(t, dir, "bom.txt", disk)
	stops := []searchindex.Stop{
		mkStop("bom.txt", 1, "hit\n", sm("hit", 0, 3)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true (BOM first-line mismatch detected)")
	}
	if len(buf.Lines[0].Highlights) != 0 {
		t.Fatalf("Highlights len = %d, want 0 (mismatched BOM first-line match dropped)", len(buf.Lines[0].Highlights))
	}
}

// TestStaleRetainsRawBytes verifies that the original line bytes
// including terminators are retained on each Line for validation.
// The RawBytes field matches the original line bytes (in rg-line
// coordinates, BOM stripped for line 1).
func TestStaleRetainsRawBytes(t *testing.T) {
	dir := t.TempDir()
	content := "hit\r\nworld\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(buf.Lines) != 2 {
		t.Fatalf("Lines len = %d, want 2", len(buf.Lines))
	}
	if !bytes.Equal(buf.Lines[0].RawBytes, []byte("hit\r\n")) {
		t.Fatalf("Line 0 RawBytes = %q, want %q (including CRLF terminator)", buf.Lines[0].RawBytes, "hit\r\n")
	}
	if !bytes.Equal(buf.Lines[1].RawBytes, []byte("world\n")) {
		t.Fatalf("Line 1 RawBytes = %q, want %q (including LF terminator)", buf.Lines[1].RawBytes, "world\n")
	}
}

// TestStaleZeroWidthValidates verifies that a zero-width submatch with
// a valid start (in range) validates and is not dropped.
func TestStaleZeroWidthValidates(t *testing.T) {
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
	if buf.Stale {
		t.Fatalf("Stale = true, want false (zero-width with valid start)")
	}
	if !hasHighlight(buf.Lines[0].Highlights, 0, 1) {
		t.Fatalf("Highlights = %v, want a [0, 1) marker (zero-width validated)", buf.Lines[0].Highlights)
	}
}

// TestStaleZeroWidthOutOfBoundDropped verifies that a zero-width
// submatch with a start past the line bytes is dropped and the buffer
// is stale.
func TestStaleZeroWidthOutOfBoundDropped(t *testing.T) {
	dir := t.TempDir()
	content := "hi\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	// Zero-width at byte 10 (past len("hi\n")=3).
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("", 10, 10)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !buf.Stale {
		t.Fatalf("Stale = false, want true (zero-width out of bounds)")
	}
}
