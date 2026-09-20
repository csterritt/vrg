package filebuffer_test

import (
	"path/filepath"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// Mixed LF and CRLF terminators each end a source line without
// appearing in the display cells; a standalone CR inside a line is
// content and displays escaped.
func TestMixedTerminators(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "one\r\ntwo\nthree\r\nfour")
	buf, err := filebuffer.Load([]byte(path), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"one", "two", "three", "four"}
	if buf.LineCount() != len(want) {
		t.Fatalf("LineCount = %d, want %d", buf.LineCount(), len(want))
	}
	for i, w := range want {
		if got := text(buf.Cells(i)); got != w {
			t.Fatalf("line %d = %q, want %q", i, got, w)
		}
	}
}

// Every source line's original bytes — line terminator included — stay
// available for byte-coordinate mapping and Issue 29's stale-match
// validation, separate from the escaped display cells.
func TestRetainedLineBytes(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "hit\r\nmiss\nlast")
	buf, err := filebuffer.Load([]byte(path), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for i, want := range []string{"hit\r\n", "miss\n", "last"} {
		if got := string(buf.LineBytes(i)); got != want {
			t.Fatalf("LineBytes(%d) = %q, want %q", i, got, want)
		}
	}
	if got := buf.LineBytes(3); got != nil {
		t.Fatalf("LineBytes(3) = %q, want nil out of range", got)
	}
}

// A standalone carriage return is not a terminator: it stays inside
// the line's retained bytes and reaches the display cells as ^M.
func TestStandaloneCarriageReturnIsContent(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "a\rb\r\n")
	buf, err := filebuffer.Load([]byte(path), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.LineCount() != 1 {
		t.Fatalf("LineCount = %d, want 1", buf.LineCount())
	}
	if got := string(buf.LineBytes(0)); got != "a\rb\r\n" {
		t.Fatalf("LineBytes(0) = %q, want %q", got, "a\rb\r\n")
	}
	if got := text(buf.Cells(0)); got != "a^Mb" {
		t.Fatalf("line 0 = %q, want %q", got, "a^Mb")
	}
}

// Byte positions inside a line's removed terminator — including
// zero-width positions — map to the display end-of-line position:
// byte 4 of "hit\r\n" is display column 3.
func TestTerminatorBytesMapToEndOfLine(t *testing.T) {
	for _, tc := range []struct {
		name       string
		content    string
		start, end int
		want       int
	}{
		{"zero-width before LF of CRLF", "hit\r\n", 4, 4, 3},
		{"zero-width at CR", "hit\r\n", 3, 3, 3},
		{"covering LF only", "hit\r\n", 4, 5, 3},
		{"covering all of CRLF", "hit\r\n", 3, 5, 3},
		{"zero-width past the line", "hit\r\n", 5, 5, 3},
		{"zero-width at LF", "hit\n", 3, 3, 3},
		{"covering LF", "hit\n", 3, 4, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), tc.content)
			buf, err := filebuffer.Load([]byte(path), nil)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			stop := stop(1, searchindex.Range{Start: tc.start, End: tc.end})
			line, cell := buf.TargetCell(stop)
			if line != 0 || cell != tc.want {
				t.Fatalf("TargetCell(%+v) = (%d, %d), want (0, %d)",
					stop.Coverage[0], line, cell, tc.want)
			}
		})
	}
}

// A coverage span reaching from visible text into the line's terminator
// highlights only the visible text; a span covering only terminator
// bytes adds no highlight — its end-of-line marker is Issue 23's.
func TestSpanAcrossTerminatorHighlightsTextOnly(t *testing.T) {
	for _, tc := range []struct {
		name       string
		start, end int
		want       *filebuffer.Span
	}{
		{"whole line plus terminator", 0, 5, &filebuffer.Span{Start: 0, End: 3}},
		{"tail of text plus terminator", 1, 5, &filebuffer.Span{Start: 1, End: 3}},
		{"terminator only", 3, 5, nil},
		{"zero-width at terminator", 4, 4, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "hit\r\n")
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
				stop(1, searchindex.Range{Start: tc.start, End: tc.end}),
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got := buf.Highlights(0)
			if tc.want == nil {
				if len(got) != 0 {
					t.Fatalf("Highlights(0) = %+v, want none", got)
				}
				return
			}
			if len(got) != 1 || got[0] != *tc.want {
				t.Fatalf("Highlights(0) = %+v, want [%+v]", got, *tc.want)
			}
		})
	}
}

// A leading UTF-8 BOM is invisible: the first line's display cells skip
// its three bytes, and rg's first-line offsets — which omit them — map
// to raw bytes three later. Later lines carry no shift.
func TestLeadingUTF8BOM(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "\xEF\xBB\xBFhit\r\nnext\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 0, End: 3}),
		stop(2, searchindex.Range{Start: 0, End: 4}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.LineCount() != 2 {
		t.Fatalf("LineCount = %d, want 2", buf.LineCount())
	}
	if got := text(buf.Cells(0)); got != "hit" {
		t.Fatalf("line 0 = %q, want %q — the BOM is not displayed", got, "hit")
	}
	// Cell byte mappings are raw-file offsets: the BOM's three bytes
	// stay accounted for, so the first cell of line 0 covers [3,4).
	if c := buf.Cells(0)[0]; c.Start != 3 || c.End != 4 {
		t.Fatalf("Cells(0)[0] maps to bytes [%d,%d), want [3,4)", c.Start, c.End)
	}
	if got := string(buf.LineBytes(0)); got != "\xEF\xBB\xBFhit\r\n" {
		t.Fatalf("LineBytes(0) = %q, want the BOM and terminator retained", got)
	}
	// rg offset 0 of the first line is raw byte 3: the match lands on
	// the "hit" cells, not on the invisible BOM position.
	if got := buf.Highlights(0); len(got) != 1 || got[0] != (filebuffer.Span{Start: 0, End: 3}) {
		t.Fatalf("Highlights(0) = %+v, want [{0 3}]", got)
	}
	if line, cell := buf.TargetCell(stop(1, searchindex.Range{Start: 0, End: 3})); line != 0 || cell != 0 {
		t.Fatalf("TargetCell = (%d, %d), want (0, 0)", line, cell)
	}
	// The shift is first-line only: line 2's rg offsets are already
	// raw offsets, and its cells map from byte 0.
	if c := buf.Cells(1)[0]; c.Start != 0 || c.End != 1 {
		t.Fatalf("Cells(1)[0] maps to bytes [%d,%d), want [0,1)", c.Start, c.End)
	}
	if got := buf.Highlights(1); len(got) != 1 || got[0] != (filebuffer.Span{Start: 0, End: 4}) {
		t.Fatalf("Highlights(1) = %+v, want [{0 4}]", got)
	}
}

// A terminator-region position on a BOM first line still reaches the
// display end of line: rg offset 4 of rg's "hit\r\n" view is raw byte
// 7, mapping to display column 3.
func TestBOMLineTerminatorMapsToEndOfLine(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "\xEF\xBB\xBFhit\r\n")
	buf, err := filebuffer.Load([]byte(path), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	line, cell := buf.TargetCell(stop(1, searchindex.Range{Start: 4, End: 4}))
	if line != 0 || cell != 3 {
		t.Fatalf("TargetCell = (%d, %d), want (0, 3)", line, cell)
	}
}

// U+FEFF anywhere but the file's first bytes is ordinary content:
// counted in byte offsets, displayed through the shared cluster policy,
// and mapped with no coordinate shift.
func TestNonLeadingFEFFIsContent(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "x\xEF\xBB\xBFy\na\n\xEF\xBB\xBFz\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 4, End: 5}),
		stop(3, searchindex.Range{Start: 3, End: 4}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Line 0 does not start the file's bytes with the BOM, so nothing
	// is stripped: three cells — x, the FEFF cluster, y — mapping to
	// raw bytes [0,1), [1,4), [4,5).
	cells := buf.Cells(0)
	if len(cells) != 3 {
		t.Fatalf("Cells(0) = %+v, want three cells", cells)
	}
	for i, want := range [][2]int{{0, 1}, {1, 4}, {4, 5}} {
		if cells[i].Start != want[0] || cells[i].End != want[1] {
			t.Fatalf("Cells(0)[%d] maps to bytes [%d,%d), want %v", i, cells[i].Start, cells[i].End, want)
		}
	}
	if got := buf.Highlights(0); len(got) != 1 || got[0] != (filebuffer.Span{Start: 2, End: 3}) {
		t.Fatalf("Highlights(0) = %+v, want [{2 3}] — y, with no BOM shift", got)
	}
	// A later line's leading U+FEFF is content too: line 2 keeps its
	// bytes and maps them from offset 0.
	if got := string(buf.LineBytes(2)); got != "\xEF\xBB\xBFz\n" {
		t.Fatalf("LineBytes(2) = %q, want the U+FEFF retained", got)
	}
	if c := buf.Cells(2)[0]; c.Start != 0 {
		t.Fatalf("Cells(2)[0].Start = %d, want 0 — no shift off the first line", c.Start)
	}
	if got := buf.Highlights(2); len(got) != 1 || got[0] != (filebuffer.Span{Start: 1, End: 2}) {
		t.Fatalf("Highlights(2) = %+v, want [{1 2}] — z after the FEFF cell", got)
	}
}

// An empty file produces zero source lines, an empty panel, and a
// one-digit-slot gutter three cells wide.
func TestEmptyFile(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 0, End: 3}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.LineCount() != 0 {
		t.Fatalf("LineCount = %d, want 0", buf.LineCount())
	}
	if got := buf.GutterWidth(); got != 3 {
		t.Fatalf("GutterWidth = %d, want 3 — one digit slot plus two spaces", got)
	}
	if got := buf.Cells(0); got != nil {
		t.Fatalf("Cells(0) = %+v, want nil — no source rows", got)
	}
	if got := buf.Highlights(0); len(got) != 0 {
		t.Fatalf("Highlights(0) = %+v, want none", got)
	}
}
