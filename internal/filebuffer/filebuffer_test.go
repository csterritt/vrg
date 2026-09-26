package filebuffer

import (
	"os"
	"path/filepath"
	"testing"

	"vrg/internal/present"
	"vrg/internal/searchindex"
)

// writeFile creates a fixture file and returns its path.
func writeFile(t *testing.T, dir, name, data string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

// load prepares path with the given per-file stops, failing on error.
func load(t *testing.T, path string, stops ...searchindex.Stop) *Buffer {
	t.Helper()
	b, err := Load([]byte(path), stops)
	if err != nil {
		t.Fatalf("Load(%q): %v", path, err)
	}
	return b
}

// Loading a file's bytes yields the source-line count: LF and CRLF
// terminate lines, a missing final newline still yields a final line, a
// trailing newline invents no extra line, and an empty file has zero
// lines.
func TestLoadLineCount(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		data string
		want int
	}{
		{"empty", "", 0},
		{"one unterminated", "a", 1},
		{"one terminated", "a\n", 1},
		{"three", "a\nb\nc\n", 3},
		{"trailing blank line", "a\n\n", 2},
		{"crlf", "a\r\nb\r\n", 2},
		{"blank first", "\na\n", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := load(t, writeFile(t, dir, "f", tc.data))
			if b.LineCount() != tc.want {
				t.Fatalf("LineCount for %q = %d, want %d", tc.data, b.LineCount(), tc.want)
			}
		})
	}
}

// The gutter is the digit width of the largest line number plus two
// spaces, with a minimum one-digit slot for placeholder-sized files.
func TestGutterWidth(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name  string
		lines int
		want  int
	}{
		{"empty", 0, 3},
		{"one", 1, 3},
		{"nine", 9, 3},
		{"ten", 10, 4},
		{"hundred", 100, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := ""
			for i := 0; i < tc.lines; i++ {
				data += "x\n"
			}
			b := load(t, writeFile(t, dir, "f", data))
			if b.GutterWidth() != tc.want {
				t.Fatalf("GutterWidth for %d lines = %d, want %d", tc.lines, b.GutterWidth(), tc.want)
			}
		})
	}
}

// Each line's display form is the escaped text; raw bytes are retained
// for comparison and mapping.
func TestLinePresentation(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "alpha\x1b\nbeta\n"))
	if got := b.Text(0); got != "alpha^[" {
		t.Fatalf("Text(0) = %q, want %q", got, "alpha^[")
	}
	if got := b.Text(1); got != "beta" {
		t.Fatalf("Text(1) = %q, want %q", got, "beta")
	}
}

// A matched line's recorded submatches become display-cell highlight
// spans, including over escaped forms: the match covering an ESC byte
// covers both caret cells.
func TestHighlightSpans(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "a\x1bb hit\n"), searchindex.Stop{
		Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{
			{Start: 1, End: 2, Bytes: []byte("\x1b")},
			{Start: 4, End: 7, Bytes: []byte("hit")},
		},
	})
	spans := b.Spans(0)
	want := []present.Span{{Start: 1, End: 3}, {Start: 5, End: 8}}
	if len(spans) != len(want) {
		t.Fatalf("Spans(0) = %+v, want %+v", spans, want)
	}
	for i := range want {
		if spans[i] != want[i] {
			t.Fatalf("Spans(0)[%d] = %+v, want %+v", i, spans[i], want[i])
		}
	}
	if got := b.Spans(1); len(got) != 0 {
		t.Fatalf("Spans(1) = %+v, want none", got)
	}
}

// Submatches that do not describe the loaded line — out of bounds or
// text-mismatched against their recorded bytes — are dropped.
func TestSubmatchValidation(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "hit\n"), searchindex.Stop{
		Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{
			{Start: 0, End: 3, Bytes: []byte("xyz")}, // text moved on
			{Start: 2, End: 9, Bytes: []byte("hit")}, // beyond the line
		},
	}, searchindex.Stop{
		Path: []byte("f"), Line: 9, // a line that does not exist
		Submatches: []searchindex.Submatch{{Start: 0, End: 1, Bytes: []byte("x")}},
	})
	if got := b.Spans(0); len(got) != 0 {
		t.Fatalf("Spans(0) = %+v, want invalid submatches dropped", got)
	}
}

// A read failure is a load error, never a partial buffer.
func TestLoadFailure(t *testing.T) {
	_, err := Load([]byte(filepath.Join(t.TempDir(), "missing")), nil)
	if err == nil {
		t.Fatal("Load of a missing file succeeded, want error")
	}
}
