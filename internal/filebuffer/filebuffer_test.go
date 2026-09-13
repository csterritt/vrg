package filebuffer_test

import (
	"os"
	"path/filepath"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// writeFile creates a file in dir with the given content and returns its
// full path.
func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// mkStop creates a searchindex.Stop for testing.
func mkStop(path string, lineNo int, line string, subs ...searchindex.Submatch) searchindex.Stop {
	return searchindex.Stop{
		RawPath:    []byte(path),
		LineNumber: lineNo,
		Line:       []byte(line),
		Submatches: subs,
	}
}

// sm creates a searchindex.Submatch for testing.
func sm(match string, start, end int) searchindex.Submatch {
	return searchindex.Submatch{Match: []byte(match), Start: start, End: end}
}

// TestLoadLineCount verifies that loading a file yields the correct
// source-line count.
func TestLoadLineCount(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.txt", []byte("hello\nworld\nfoo\n"))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 3 {
		t.Fatalf("LineCount = %d, want 3", buf.LineCount)
	}
}

// TestLoadLineCountNoFinalNewline verifies that a file without a trailing
// newline still counts the last line.
func TestLoadLineCountNoFinalNewline(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.txt", []byte("hello\nworld"))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 2 {
		t.Fatalf("LineCount = %d, want 2", buf.LineCount)
	}
}

// TestLoadLineCountEmptyFile verifies that an empty file has zero lines.
func TestLoadLineCountEmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "empty.txt", nil)
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 0 {
		t.Fatalf("LineCount = %d, want 0", buf.LineCount)
	}
}

// TestLoadLineCountCRLF verifies that CRLF is counted as one line ending.
func TestLoadLineCountCRLF(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "crlf.txt", []byte("hello\r\nworld\r\n"))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 2 {
		t.Fatalf("LineCount = %d, want 2", buf.LineCount)
	}
}

// TestLoadLineCountTrailingNewline verifies that a trailing newline does
// not invent an extra empty line.
func TestLoadLineCountTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.txt", []byte("hello\n"))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.LineCount != 1 {
		t.Fatalf("LineCount = %d, want 1 (trailing newline does not add empty line)", buf.LineCount)
	}
}

// TestLoadGutterWidth verifies that the gutter width is the digit width
// of the largest line number plus two spaces.
func TestLoadGutterWidth(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		wantGutter int
	}{
		{"3 lines (1 digit + 2)", "a\nb\nc\n", 3},
		{"10 lines (2 digits + 2)", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n", 4},
		{"1 line (1 digit + 2)", "hello\n", 3},
		{"0 lines (min 1 digit + 2)", "", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := writeFile(t, dir, "test.txt", []byte(tc.content))
			buf, err := filebuffer.Load([]byte(p), nil)
			if err != nil {
				t.Fatalf("Load error: %v", err)
			}
			if buf.GutterWidth != tc.wantGutter {
				t.Fatalf("GutterWidth = %d, want %d", buf.GutterWidth, tc.wantGutter)
			}
		})
	}
}

// TestLoadHighlightSpans verifies that a matched line has highlight spans
// covering the match's display cells.
func TestLoadHighlightSpans(t *testing.T) {
	dir := t.TempDir()
	content := "hello world\nfoo bar\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, "hello world\n", sm("hello", 0, 5)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(buf.Lines) != 2 {
		t.Fatalf("Lines len = %d, want 2", len(buf.Lines))
	}
	line1 := buf.Lines[0]
	if line1.Number != 1 {
		t.Fatalf("Line 0 Number = %d, want 1", line1.Number)
	}
	if line1.Display != "hello world" {
		t.Fatalf("Line 0 Display = %q, want %q", line1.Display, "hello world")
	}
	if len(line1.Highlights) != 1 {
		t.Fatalf("Line 0 Highlights len = %d, want 1", len(line1.Highlights))
	}
	hl := line1.Highlights[0]
	if hl[0] != 0 || hl[1] != 5 {
		t.Fatalf("Highlight = %v, want [0, 5)", hl)
	}
}

// TestLoadHighlightSpansMultipleMatches verifies that multiple submatches
// on the same line produce multiple highlight spans.
func TestLoadHighlightSpansMultipleMatches(t *testing.T) {
	dir := t.TempDir()
	content := "hello world\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, "hello world\n",
			sm("hello", 0, 5),
			sm("world", 6, 11),
		),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line1 := buf.Lines[0]
	if len(line1.Highlights) != 2 {
		t.Fatalf("Highlights len = %d, want 2", len(line1.Highlights))
	}
	if line1.Highlights[0] != [2]int{0, 5} {
		t.Fatalf("Highlight 0 = %v, want [0, 5)", line1.Highlights[0])
	}
	if line1.Highlights[1] != [2]int{6, 11} {
		t.Fatalf("Highlight 1 = %v, want [6, 11)", line1.Highlights[1])
	}
}

// TestLoadHighlightEscapedForm verifies that a highlight covering an ESC
// byte maps to both cells of the ^[ escape.
func TestLoadHighlightEscapedForm(t *testing.T) {
	dir := t.TempDir()
	content := "a\x1bb\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	stops := []searchindex.Stop{
		mkStop("test.txt", 1, content, sm("\x1b", 1, 2)),
	}
	buf, err := filebuffer.Load([]byte(p), stops)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	line1 := buf.Lines[0]
	// Display should be "a^[b" (ESC escaped as ^[, 'b' follows).
	if line1.Display != "a^[b" {
		t.Fatalf("Display = %q, want %q", line1.Display, "a^[b")
	}
	if len(line1.Highlights) != 1 {
		t.Fatalf("Highlights len = %d, want 1", len(line1.Highlights))
	}
	hl := line1.Highlights[0]
	// ESC byte (index 1) maps to cells [1, 3) — covering both ^ and [.
	if hl[0] != 1 || hl[1] != 3 {
		t.Fatalf("Highlight = %v, want [1, 3) (must cover both ^ and [)", hl)
	}
}

// TestLoadNonExistentFile verifies that loading a non-existent file
// returns an error.
func TestLoadNonExistentFile(t *testing.T) {
	_, err := filebuffer.Load([]byte("/nonexistent/path/file.txt"), nil)
	if err == nil {
		t.Fatal("Load returned nil error for non-existent file")
	}
}

// TestLoadDisplayText verifies that the display text has line terminators
// removed and content escaped.
func TestLoadDisplayText(t *testing.T) {
	dir := t.TempDir()
	content := "hello\nworld\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.Lines[0].Display != "hello" {
		t.Fatalf("Line 0 Display = %q, want %q", buf.Lines[0].Display, "hello")
	}
	if buf.Lines[1].Display != "world" {
		t.Fatalf("Line 1 Display = %q, want %q", buf.Lines[1].Display, "world")
	}
}

// TestLoadDisplayTextEscapesControls verifies that control bytes in
// content are escaped in the display text.
func TestLoadDisplayTextEscapesControls(t *testing.T) {
	dir := t.TempDir()
	content := "a\x1bb\n"
	p := writeFile(t, dir, "test.txt", []byte(content))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if buf.Lines[0].Display != "a^[b" {
		t.Fatalf("Display = %q, want %q", buf.Lines[0].Display, "a^[b")
	}
}

// TestLoadLineNumbers verifies that lines carry correct 1-based numbers.
func TestLoadLineNumbers(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.txt", []byte("a\nb\nc\n"))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	for i, want := range []int{1, 2, 3} {
		if buf.Lines[i].Number != want {
			t.Fatalf("Line %d Number = %d, want %d", i, buf.Lines[i].Number, want)
		}
	}
}

// TestLoadNoMatches verifies that a file with no stops has no highlights.
func TestLoadNoMatches(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "test.txt", []byte("hello\nworld\n"))
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	for i, line := range buf.Lines {
		if len(line.Highlights) != 0 {
			t.Fatalf("Line %d has %d highlights, want 0", i, len(line.Highlights))
		}
	}
}
