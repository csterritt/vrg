package filebuffer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// writeFile creates a fixture file with exact bytes.
func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

// stop builds a matched-line stop: a 1-based source line with highlight
// coverage byte ranges.
func stop(line int64, ranges ...searchindex.Range) searchindex.Stop {
	return searchindex.Stop{Line: line, Coverage: ranges}
}

// text joins the cells of one display line.
func text(cells []safepresentation.Cell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteString(c.Text)
	}
	return b.String()
}

// Load reads a file's bytes and splits them into display-ready source
// lines: LF and CRLF are terminators, an unterminated final line counts,
// and an empty file has zero lines.
func TestLoadSourceLines(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		lines   int
		first   string
	}{
		{"two lines", "alpha\nbeta\n", 2, "alpha"},
		{"no final newline", "alpha\nbeta", 2, "alpha"},
		{"trailing newline adds no line", "alpha\n", 1, "alpha"},
		{"blank line kept", "a\n\n", 2, "a"},
		{"empty file", "", 0, ""},
		{"crlf", "a\r\nb\r\n", 2, "a"},
		{"standalone carriage return", "a\rb\n", 1, "a^Mb"},
		{"lone newline is one empty line", "\n", 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), tc.content)
			buf, err := filebuffer.Load([]byte(path), nil)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if buf.LineCount() != tc.lines {
				t.Fatalf("LineCount = %d, want %d", buf.LineCount(), tc.lines)
			}
			if tc.lines > 0 {
				if got := text(buf.Cells(0)); got != tc.first {
					t.Fatalf("line 0 = %q, want %q", got, tc.first)
				}
			}
		})
	}
}

// The gutter width is the digit width of the largest line number plus
// two spaces, with at least one digit slot.
func TestGutterWidth(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines int
		want  int
	}{
		{"empty file keeps one digit slot", 0, 3},
		{"three lines", 3, 3},
		{"twelve lines", 12, 4},
		{"one fifty lines", 150, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var content strings.Builder
			for i := 0; i < tc.lines; i++ {
				content.WriteString("x\n")
			}
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), content.String())
			buf, err := filebuffer.Load([]byte(path), nil)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := buf.GutterWidth(); got != tc.want {
				t.Fatalf("GutterWidth = %d, want %d", got, tc.want)
			}
		})
	}
}

// Highlights maps each stop's coverage byte ranges onto the escaped
// display cells of its line.
func TestHighlights(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "hit me\nno match\nhit again\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 0, End: 3}),
		stop(3, searchindex.Range{Start: 4, End: 9}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := buf.Highlights(0)
	if len(got) != 1 || got[0].Start != 0 || got[0].End != 3 {
		t.Fatalf("Highlights(0) = %+v, want [{0 3}]", got)
	}
	if got := buf.Highlights(1); len(got) != 0 {
		t.Fatalf("Highlights(1) = %+v, want none", got)
	}
	got = buf.Highlights(2)
	if len(got) != 1 || got[0].Start != 4 || got[0].End != 9 {
		t.Fatalf("Highlights(2) = %+v, want [{4 9}]", got)
	}
}

// A highlight covering an escaped control byte covers every cell of the
// escaped form: matching the ESC byte highlights both ^ and [.
func TestHighlightCoversEscapedCells(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.bin"), "x\x1by\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 1, End: 2}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := text(buf.Cells(0)); got != "x^[y" {
		t.Fatalf("line 0 = %q, want %q", got, "x^[y")
	}
	got := buf.Highlights(0)
	if len(got) != 1 || got[0].Start != 1 || got[0].End != 3 {
		t.Fatalf("Highlights(0) = %+v, want [{1 3}] covering both caret cells", got)
	}
}

// Coverage reaching past the line's bytes — for example into its
// terminator — is clamped to the displayed cells.
func TestHighlightClampedToLine(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "hit\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 0, End: 99}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := buf.Highlights(0)
	if len(got) != 1 || got[0].Start != 0 || got[0].End != 3 {
		t.Fatalf("Highlights(0) = %+v, want [{0 3}] clamped to the line", got)
	}
}

// Invalid UTF-8 content displays U+FFFD while retaining raw-byte
// mappings, so a highlight over the invalid byte covers the replacement
// cell.
func TestInvalidUTF8Content(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.bin"), "a\xffb\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 1, End: 2}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := text(buf.Cells(0)); got != "a\uFFFDb" {
		t.Fatalf("line 0 = %q, want %q", got, "a\uFFFDb")
	}
	got := buf.Highlights(0)
	if len(got) != 1 || got[0].Start != 1 || got[0].End != 2 {
		t.Fatalf("Highlights(0) = %+v, want [{1 2}] covering the replacement cell", got)
	}
}

// A read failure reports an error rather than a partial buffer.
func TestLoadReadFailure(t *testing.T) {
	_, err := filebuffer.Load([]byte(filepath.Join(t.TempDir(), "missing.txt")), nil)
	if err == nil {
		t.Fatal("Load of a missing file reported no error")
	}
}
