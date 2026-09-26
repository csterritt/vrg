package filebuffer

import (
	"testing"

	"vrg/internal/present"
	"vrg/internal/searchindex"
)

// LF and CRLF are line terminators, never displayed content, and the
// two forms can mix freely inside one file. A missing final newline
// still yields the final line and a trailing newline invents no extra
// one — exercised here through each line's display text.
func TestTerminatorsUndisplayed(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		data string
		want []string
	}{
		{"lf", "a\nb\n", []string{"a", "b"}},
		{"crlf", "a\r\nb\r\n", []string{"a", "b"}},
		{"mixed lf and crlf", "a\r\nb\nc\r\nd", []string{"a", "b", "c", "d"}},
		{"crlf unterminated final line", "a\r\nb", []string{"a", "b"}},
		{"crlf blank line", "a\r\n\r\n", []string{"a", ""}},
		{"lf blank line", "a\n\n", []string{"a", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := load(t, writeFile(t, dir, "f", tc.data))
			if b.LineCount() != len(tc.want) {
				t.Fatalf("LineCount for %q = %d, want %d", tc.data, b.LineCount(), len(tc.want))
			}
			for i, w := range tc.want {
				if got := b.Text(i); got != w {
					t.Fatalf("Text(%d) for %q = %q, want %q", i, tc.data, got, w)
				}
			}
		})
	}
}

// A standalone CR — one not followed by LF — is not a terminator: the
// safe-presentation core escapes it as ^M inside the line it sits in.
func TestStandaloneCREscapes(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "a\rb\rc"))
	if b.LineCount() != 1 {
		t.Fatalf("LineCount for %q = %d, want 1 — a standalone CR is not a terminator",
			"a\rb\rc", b.LineCount())
	}
	if got := b.Text(0); got != "a^Mb^Mc" {
		t.Fatalf("Text(0) = %q, want %q", got, "a^Mb^Mc")
	}
}

// An empty file has zero source lines — no source rows at all — and
// still reserves the gutter's minimum one-digit slot plus two spaces:
// a three-cell gutter behind an empty panel.
func TestEmptyFile(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", ""))
	if b.LineCount() != 0 {
		t.Fatalf("LineCount for an empty file = %d, want 0", b.LineCount())
	}
	if b.GutterDigits() != 1 || b.GutterWidth() != 3 {
		t.Fatalf("empty-file gutter = %d digits/%d cells, want 1 digit/3 cells",
			b.GutterDigits(), b.GutterWidth())
	}
}

// Removed terminator bytes and zero-width positions map to the display
// end-of-line position: byte 4 of "hit\r\n" — its LF — maps to display
// column 3, and a match solely on removed terminator bytes yields an
// end-of-line marker position. The terminator bytes stay in the line's
// raw view, which is why a recorded match on them validates at all.
func TestTerminatorBytesMapToDisplayEOL(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name       string
		data       string
		start, end int
		match      string
		want       present.Span
	}{
		{"zero-width at the lf of a crlf", "hit\r\n", 4, 4, "", present.Span{Start: 3, End: 3}},
		{"crlf terminator only", "hit\r\n", 3, 5, "\r\n", present.Span{Start: 3, End: 3}},
		{"cr byte of a crlf", "hit\r\n", 3, 4, "\r", present.Span{Start: 3, End: 3}},
		{"lf terminator only", "hit\n", 3, 4, "\n", present.Span{Start: 3, End: 3}},
		{"zero-width at lf", "hit\n", 3, 3, "", present.Span{Start: 3, End: 3}},
		{"zero-width on an empty line", "\n", 0, 0, "", present.Span{Start: 0, End: 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := load(t, writeFile(t, dir, "f", tc.data), searchindex.Stop{
				Path: []byte("f"), Line: 1,
				Submatches: []searchindex.Submatch{
					{Start: tc.start, End: tc.end, Bytes: []byte(tc.match)},
				},
			})
			got := b.Spans(0)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("Spans(0) for %q[%d:%d] = %+v, want the end-of-line position %+v",
					tc.data, tc.start, tc.end, got, tc.want)
			}
		})
	}
}

// A span covering visible text plus the line's terminator highlights
// only the visible text: the removed terminator bytes map to the
// end-of-line position, which the half-open span already reaches.
func TestSpanAcrossTerminatorHighlightsTextOnly(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name       string
		data       string
		start, end int
		match      string
		want       present.Span
	}{
		{"text plus crlf", "hit\r\n", 0, 5, "hit\r\n", present.Span{Start: 0, End: 3}},
		{"trailing text plus cr", "hit\r\n", 2, 4, "t\r", present.Span{Start: 2, End: 3}},
		{"text plus lf", "hit\n", 0, 4, "hit\n", present.Span{Start: 0, End: 3}},
		{"mid-text through crlf", "hit\r\n", 1, 5, "it\r\n", present.Span{Start: 1, End: 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := load(t, writeFile(t, dir, "f", tc.data), searchindex.Stop{
				Path: []byte("f"), Line: 1,
				Submatches: []searchindex.Submatch{
					{Start: tc.start, End: tc.end, Bytes: []byte(tc.match)},
				},
			})
			got := b.Spans(0)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("Spans(0) for %q[%d:%d] = %+v, want the visible-text span %+v",
					tc.data, tc.start, tc.end, got, tc.want)
			}
		})
	}
}

// A leading UTF-8 BOM is invisible in display. rg removes its three
// bytes from first-line data, so the buffer keeps separate views on
// that line: the raw line retains the BOM while rg-line submatch
// offsets omit it — an rg offset of 0 is raw byte 3, and the shift
// applies to positions and endpoints alike. Later lines are
// unaffected: their rg and raw offsets agree.
func TestLeadingUTF8BOM(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "\xef\xbb\xbfhit\nbye\n"),
		searchindex.Stop{
			Path: []byte("f"), Line: 1,
			Submatches: []searchindex.Submatch{
				// rg reports the match at offset 0 of its BOM-less
				// first line; it lands on raw bytes 3-5, cells 0-2.
				{Start: 0, End: 3, Bytes: []byte("hit")},
				// rg's end-of-line position 3 is raw byte 6, the LF —
				// a removed byte, so an end-of-line marker position.
				{Start: 3, End: 3, Bytes: []byte("")},
			},
		},
		searchindex.Stop{
			Path: []byte("f"), Line: 2,
			Submatches: []searchindex.Submatch{
				{Start: 0, End: 3, Bytes: []byte("bye")},
			},
		})
	if got := b.Text(0); got != "hit" {
		t.Fatalf("Text(0) = %q, want %q — the BOM paints nothing", got, "hit")
	}
	if n := len(b.Cells(0)); n != 3 {
		t.Fatalf("len(Cells(0)) = %d, want 3 — no cell for the BOM", n)
	}
	if got := b.Text(1); got != "bye" {
		t.Fatalf("Text(1) = %q, want %q", got, "bye")
	}
	want0 := []present.Span{{Start: 0, End: 3}, {Start: 3, End: 3}}
	if got := b.Spans(0); len(got) != len(want0) || got[0] != want0[0] || got[1] != want0[1] {
		t.Fatalf("Spans(0) = %+v, want %+v", got, want0)
	}
	want1 := []present.Span{{Start: 0, End: 3}}
	if got := b.Spans(1); len(got) != 1 || got[0] != want1[0] {
		t.Fatalf("Spans(1) = %+v, want %+v — line 2 offsets are unshifted", got, want1)
	}
}

// The shift is real, not a prefix coincidence: when the content after
// the file BOM opens with a second U+FEFF, rg's offset 0 names that
// content FEFF — raw byte 3 — and only the file-leading bytes are
// invisible. The BOM bytes themselves are unreachable in rg-line
// space.
func TestBOMShiftMapsRGOffsetsToRawBytes(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "\xef\xbb\xbf\xef\xbb\xbfhit\n"),
		searchindex.Stop{
			Path: []byte("f"), Line: 1,
			Submatches: []searchindex.Submatch{
				{Start: 0, End: 3, Bytes: []byte("\xef\xbb\xbf")}, // the content FEFF
				{Start: 3, End: 6, Bytes: []byte("hit")},
			},
		})
	// The content FEFF is an ordinary standalone zero-width cluster:
	// the provisional dotted-circle fallback cell, then hit.
	if got := b.Text(0); got != "◌\ufeffhit" {
		t.Fatalf("Text(0) = %q, want %q", got, "◌\ufeffhit")
	}
	want := []present.Span{{Start: 0, End: 1}, {Start: 1, End: 4}}
	if got := b.Spans(0); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Spans(0) = %+v, want %+v — rg offset 0 is raw byte 3", got, want)
	}
}

// A file whose only content is the BOM is one zero-display line, not
// an empty file and not a visible BOM.
func TestBOMOnlyFile(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		data string
	}{
		{"bom only", "\xef\xbb\xbf"},
		{"bom then lf", "\xef\xbb\xbf\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := load(t, writeFile(t, dir, "f", tc.data))
			if b.LineCount() != 1 {
				t.Fatalf("LineCount for %q = %d, want 1", tc.data, b.LineCount())
			}
			if got := b.Text(0); got != "" {
				t.Fatalf("Text(0) for %q = %q, want an empty display line", tc.data, got)
			}
			if n := len(b.Cells(0)); n != 0 {
				t.Fatalf("len(Cells(0)) for %q = %d, want 0", tc.data, n)
			}
		})
	}
}

// U+FEFF anywhere but the file's leading bytes is ordinary content,
// never treated as a BOM: mid-line it joins the previous cell as a
// zero-width cluster, and at a later line's start it takes the usual
// standalone-cluster fallback cell — its bytes match and highlight
// like any other content.
func TestNonLeadingFEFFIsContent(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "f", "a\ufeffb\n\ufeffz\n"),
		searchindex.Stop{
			Path: []byte("f"), Line: 1,
			Submatches: []searchindex.Submatch{
				{Start: 1, End: 4, Bytes: []byte("\xef\xbb\xbf")},
			},
		},
		searchindex.Stop{
			Path: []byte("f"), Line: 2,
			Submatches: []searchindex.Submatch{
				{Start: 0, End: 3, Bytes: []byte("\xef\xbb\xbf")},
			},
		})
	if got := b.Text(0); got != "a\ufeffb" {
		t.Fatalf("Text(0) = %q, want %q — the FEFF joins the a cell", got, "a\ufeffb")
	}
	if n := len(b.Cells(0)); n != 2 {
		t.Fatalf("len(Cells(0)) = %d, want 2", n)
	}
	if got := b.Spans(0); len(got) != 1 || got[0] != (present.Span{Start: 0, End: 1}) {
		t.Fatalf("Spans(0) = %+v, want the a cell %+v", got, present.Span{Start: 0, End: 1})
	}
	if got := b.Text(1); got != "◌\ufeffz" {
		t.Fatalf("Text(1) = %q, want %q — a line-initial FEFF is content, not a BOM",
			got, "◌\ufeffz")
	}
	if got := b.Spans(1); len(got) != 1 || got[0] != (present.Span{Start: 0, End: 1}) {
		t.Fatalf("Spans(1) = %+v, want the fallback cell %+v — no BOM shift on line 2",
			got, present.Span{Start: 0, End: 1})
	}
}
