package filebuffer_test

import (
	"os"
	"path/filepath"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// writeFile stores content in a fresh temp dir and returns its raw
// path bytes.
func writeFile(t *testing.T, content string) []byte {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return []byte(p)
}

// stop is one matched-line stop carrying the given union highlight
// byte ranges.
func stop(line int64, spans ...[2]int) searchindex.Stop {
	st := searchindex.Stop{Number: line}
	for _, s := range spans {
		st.Highlights = append(st.Highlights, searchindex.Span{Start: s[0], End: s[1]})
	}
	return st
}

func load(t *testing.T, path []byte, stops ...searchindex.Stop) *filebuffer.Buffer {
	t.Helper()
	b, err := filebuffer.Load(path, stops)
	if err != nil {
		t.Fatalf("Load(%q): %v", path, err)
	}
	return b
}

// Loading a file's bytes yields the source-line count and the escaped
// per-line display text.
func TestLoadCountsLines(t *testing.T) {
	b := load(t, writeFile(t, "one\ntwo\nthree\n"))
	if b.LineCount() != 3 {
		t.Fatalf("LineCount = %d, want 3", b.LineCount())
	}
	lines := b.Lines()
	for i, want := range []string{"one", "two", "three"} {
		if lines[i].Text != want {
			t.Fatalf("line %d text = %q, want %q", i, lines[i].Text, want)
		}
		if lines[i].Number != int64(i+1) {
			t.Fatalf("line %d number = %d, want %d", i, lines[i].Number, i+1)
		}
	}
}

// A missing final newline still yields the last line; a trailing
// newline does not invent an empty line.
func TestLoadLineEndings(t *testing.T) {
	if b := load(t, writeFile(t, "one\ntwo")); b.LineCount() != 2 {
		t.Fatalf("unterminated tail: LineCount = %d, want 2", b.LineCount())
	}
	if b := load(t, writeFile(t, "one\n\n")); b.LineCount() != 2 {
		t.Fatalf("trailing newline: LineCount = %d, want 2", b.LineCount())
	}
	// CRLF is a terminator, never displayed.
	b := load(t, writeFile(t, "a\r\nb\r\n"))
	if b.LineCount() != 2 || b.Lines()[0].Text != "a" || b.Lines()[1].Text != "b" {
		t.Fatalf("CRLF lines = %q, %q", b.Lines()[0].Text, b.Lines()[1].Text)
	}
}

// An empty file has zero source lines and the minimum one-digit-slot
// gutter.
func TestLoadEmptyFile(t *testing.T) {
	b := load(t, writeFile(t, ""))
	if b.LineCount() != 0 {
		t.Fatalf("LineCount = %d, want 0", b.LineCount())
	}
	if b.GutterWidth() != 3 {
		t.Fatalf("GutterWidth = %d, want 3 (one digit slot + two spaces)", b.GutterWidth())
	}
}

// The gutter is the digit width of the largest line number plus two
// spaces.
func TestGutterWidth(t *testing.T) {
	if b := load(t, writeFile(t, "a\nb\nc\n")); b.GutterWidth() != 3 {
		t.Fatalf("3-line file: GutterWidth = %d, want 3", b.GutterWidth())
	}
	var ten string
	for i := 0; i < 10; i++ {
		ten += "x\n"
	}
	if b := load(t, writeFile(t, ten)); b.GutterWidth() != 4 {
		t.Fatalf("10-line file: GutterWidth = %d, want 4", b.GutterWidth())
	}
}

// Content escapes land per the safe-presentation core: caret notation
// for C0/DEL, U+FFFD for invalid UTF-8, ^M for a standalone CR, → for a
// tab.
func TestLoadEscapedContent(t *testing.T) {
	b := load(t, writeFile(t, "a\x1bb\xffc\rd\te\n"))
	l := b.Lines()[0]
	if l.Text != "a^[bc^Md→e" {
		t.Fatalf("escaped text = %q, want %q", l.Text, "a^[bc^Md→e")
	}
	// One cell per display cell: the text's ten printable cells plus the
	// invalid byte's U+FFFD cell, whose Text is empty.
	if len(l.Cells) != 11 {
		t.Fatalf("cells = %d, want 11 (one per display cell)", len(l.Cells))
	}
}

// Highlight spans for a matched line map byte ranges through the
// line's byte→cell map.
func TestHighlightSpans(t *testing.T) {
	b := load(t, writeFile(t, "zero\nalpha here\n"), stop(2, [2]int{0, 5}))
	lines := b.Lines()
	if len(lines[0].Highlights) != 0 {
		t.Fatalf("line 1 highlights = %v, want none", lines[0].Highlights)
	}
	got := lines[1].Highlights
	if len(got) != 1 || got[0].Start != 0 || got[0].End != 5 {
		t.Fatalf("line 2 highlights = %v, want [{0 5}]", got)
	}
}

// A match covering the ESC byte highlights both cells of its ^[ form.
func TestHighlightCoversEscapedCells(t *testing.T) {
	b := load(t, writeFile(t, "a\x1bb\n"), stop(1, [2]int{1, 2}))
	got := b.Lines()[0].Highlights
	if len(got) != 1 || got[0].Start != 1 || got[0].End != 3 {
		t.Fatalf("highlights = %v, want [{1 3}] covering ^[", got)
	}
}

// Stops outside the loaded file's range contribute nothing.
func TestStopBeyondFileIgnored(t *testing.T) {
	b := load(t, writeFile(t, "one\ntwo\n"), stop(99, [2]int{0, 2}))
	for i, l := range b.Lines() {
		if len(l.Highlights) != 0 {
			t.Fatalf("line %d highlights = %v, want none", i, l.Highlights)
		}
	}
}

// A read failure is an error, never a partial buffer.
func TestLoadReadFailure(t *testing.T) {
	b, err := filebuffer.Load([]byte(filepath.Join(t.TempDir(), "gone")), nil)
	if err == nil || b != nil {
		t.Fatalf("Load(missing) = %v, %v, want nil buffer and error", b, err)
	}
}
