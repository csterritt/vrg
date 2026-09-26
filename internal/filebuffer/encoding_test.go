package filebuffer

import (
	"testing"

	"vrg/internal/searchindex"
)

// The four unsupported BOMs classify their file by encoding name: the
// buffer carries the "(unsupported encoding)" placeholder's empty
// state — no lines, no highlight spans, no stale verdict — and the
// reveal target stays the inert zero position so the file remains an
// indexed cursor stop without an invented landing.
func TestUnsupportedBOMsClassify(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		want string
	}{
		{"UTF-16 LE", "\xff\xfeh\x00i\x00\n\x00", "UTF-16 LE"},
		{"UTF-16 BE", "\xfe\xff\x00h\x00i\x00\n", "UTF-16 BE"},
		{"UTF-32 LE", "\xff\xfe\x00\x00h\x00\x00\x00i\x00\x00\x00", "UTF-32 LE"},
		{"UTF-32 BE", "\x00\x00\xfe\xff\x00\x00\x00h\x00\x00\x00i", "UTF-32 BE"},
		{"UTF-16 LE bare mark", "\xff\xfe", "UTF-16 LE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := prepare(tc.data,
				searchindex.Stop{Path: []byte("f"), Line: 1,
					Submatches: []searchindex.Submatch{{Start: 0, End: 2, Bytes: []byte("hi")}}})
			if got := b.Unsupported(); got != tc.want {
				t.Fatalf("Unsupported() = %q, want %q", got, tc.want)
			}
			if b.LineCount() != 0 {
				t.Fatalf("LineCount = %d, want no file text", b.LineCount())
			}
			if got := b.Spans(0); len(got) != 0 {
				t.Fatalf("Spans(0) = %+v, want no highlights", got)
			}
			if b.Stale() {
				t.Fatal("an unsupported buffer reported stale")
			}
			if line, cell := b.RevealTarget(searchindex.Stop{Path: []byte("f"), Line: 1}); line != 0 || cell != 0 {
				t.Fatalf("RevealTarget = (%d,%d), want the inert (0,0)", line, cell)
			}
		})
	}
}

// The overlap ordering: FF FE 00 00 opens with the UTF-16 LE mark's
// own bytes, so the longer UTF-32 LE mark must be tried first —
// checking UTF-16 first would misclassify this file.
func TestUTF32LEWinsOverlapWithUTF16LE(t *testing.T) {
	b := prepare("\xff\xfe\x00\x00h\x00\x00\x00\n\x00\x00\x00")
	if got := b.Unsupported(); got != "UTF-32 LE" {
		t.Fatalf("Unsupported() = %q, want UTF-32 LE — the longer mark wins the overlap", got)
	}
}

// Load detects the same marks on the ReadFile path Prepare wraps.
func TestLoadReportsUnsupported(t *testing.T) {
	dir := t.TempDir()
	b := load(t, writeFile(t, dir, "u16.txt", "\xff\xfeh\x00i\x00\n\x00"))
	if got := b.Unsupported(); got != "UTF-16 LE" {
		t.Fatalf("Load(...).Unsupported() = %q, want UTF-16 LE", got)
	}
}

// A leading UTF-8 BOM is supported content, not an unsupported
// encoding: the file loads, its invisible BOM shifts the first-line
// coordinates, and the recorded submatch still validates.
func TestUTF8BOMIsNotMisclassified(t *testing.T) {
	b := prepare("\xef\xbb\xbfhit\n",
		searchindex.Stop{Path: []byte("f"), Line: 1,
			Submatches: []searchindex.Submatch{{Start: 0, End: 3, Bytes: []byte("hit")}}})
	if got := b.Unsupported(); got != "" {
		t.Fatalf("Unsupported() = %q, want the UTF-8 BOM supported", got)
	}
	if b.LineCount() != 1 || b.Text(0) != "hit" {
		t.Fatalf("UTF-8 BOM file: LineCount=%d Text=%q, want 1/\"hit\"", b.LineCount(), b.Text(0))
	}
	if b.Stale() {
		t.Fatal("the shifted first-line submatch must still validate")
	}
	if got := b.Spans(0); len(got) != 1 || got[0].Start != 0 || got[0].End != 3 {
		t.Fatalf("Spans(0) = %+v, want the validated hit span", got)
	}
}

// Only a leading mark classifies: a lone FF, a mark not at the file's
// start, and a truncated UTF-32 BE prefix are ordinary (escaped)
// content, not encodings.
func TestNonBOMPrefixesStaySupported(t *testing.T) {
	for _, tc := range []struct{ name, data string }{
		{"single FF", "\xffhi\n"},
		{"FF FE not at start", "x\xff\xfehi\n"},
		{"truncated UTF-32 BE", "\x00\x00\xfe\n"},
		{"plain text", "hi\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := prepare(tc.data).Unsupported(); got != "" {
				t.Fatalf("Unsupported() = %q, want supported content", got)
			}
		})
	}
}

// Unsupported bytes never face stale-match validation: rg's recorded
// submatches name transcoded offsets that can never equal the raw
// encoded bytes, so a mismatching or out-of-range submatch marks
// nothing — the buffer is neither stale nor highlighted.
func TestUnsupportedBytesSkipStaleValidation(t *testing.T) {
	b := prepare("\xff\xfeh\x00i\x00\n\x00",
		searchindex.Stop{Path: []byte("f"), Line: 1,
			Submatches: []searchindex.Submatch{{Start: 0, End: 2, Bytes: []byte("hi")}}},
		searchindex.Stop{Path: []byte("f"), Line: 9,
			Submatches: []searchindex.Submatch{{Start: 0, End: 1, Bytes: []byte("x")}}})
	if b.Unsupported() == "" {
		t.Fatal("the UTF-16 LE file was not classified")
	}
	if b.Stale() {
		t.Fatal("stale validation ran against unsupported bytes")
	}
	if got := b.Spans(0); len(got) != 0 {
		t.Fatalf("Spans(0) = %+v, want no highlights on a placeholder", got)
	}
}
