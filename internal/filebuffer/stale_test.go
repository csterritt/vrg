package filebuffer

import (
	"testing"

	"vrg/internal/present"
	"vrg/internal/searchindex"
)

// prepare is Load's twin for in-memory bytes: the reload path's
// revalidation is a fresh Prepare over newly read data, so tests drive
// the contract without touching the filesystem.
func prepare(data string, stops ...searchindex.Stop) *Buffer {
	return Prepare([]byte(data), stops)
}

// A buffer whose every recorded submatch validates — line present,
// range in bounds against the raw bytes, recorded bytes equal — is not
// stale.
func TestFullyValidatingBufferIsNotStale(t *testing.T) {
	b := prepare("hit\nbye\n",
		searchindex.Stop{Path: []byte("f"), Line: 1,
			Submatches: []searchindex.Submatch{{Start: 0, End: 3, Bytes: []byte("hit")}}},
		searchindex.Stop{Path: []byte("f"), Line: 2,
			Submatches: []searchindex.Submatch{{Start: 0, End: 3, Bytes: []byte("bye")}}})
	if b.Stale() {
		t.Fatal("a fully validating buffer reported stale")
	}
}

// Every validation failure drops its submatch and marks the buffer
// stale: an out-of-bounds range, a same-length replacement whose bytes
// moved on, and a stop whose line no longer exists. The checks run on
// first load and every reload alike — Prepare is both.
func TestDroppedSubmatchesMarkStale(t *testing.T) {
	for _, tc := range []struct {
		name  string
		data  string
		stops []searchindex.Stop
	}{
		{"same-length replacement", "hit\n", []searchindex.Stop{
			{Path: []byte("f"), Line: 1,
				Submatches: []searchindex.Submatch{{Start: 0, End: 3, Bytes: []byte("xyz")}}},
		}},
		{"end out of bounds", "hit\n", []searchindex.Stop{
			{Path: []byte("f"), Line: 1,
				Submatches: []searchindex.Submatch{{Start: 2, End: 9, Bytes: []byte("hit")}}},
		}},
		{"start out of bounds", "hit\n", []searchindex.Stop{
			{Path: []byte("f"), Line: 1,
				Submatches: []searchindex.Submatch{{Start: 9, End: 10, Bytes: []byte("x")}}},
		}},
		{"line missing", "hit\n", []searchindex.Stop{
			{Path: []byte("f"), Line: 9,
				Submatches: []searchindex.Submatch{{Start: 0, End: 1, Bytes: []byte("x")}}},
		}},
		{"one of two dropped", "zzz mid\n", []searchindex.Stop{
			{Path: []byte("f"), Line: 1,
				Submatches: []searchindex.Submatch{
					{Start: 0, End: 3, Bytes: []byte("hit")},
					{Start: 4, End: 7, Bytes: []byte("mid")},
				}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := prepare(tc.data, tc.stops...)
			if !b.Stale() {
				t.Fatalf("Prepare(%q) did not mark the buffer stale", tc.data)
			}
		})
	}
}

// A dropped submatch leaves the line's other valid submatches in
// place: the survivor keeps its highlight and becomes the stale
// entry's reveal target — the first surviving submatch's start cell.
func TestStalePartialSurvivalKeepsValidHighlights(t *testing.T) {
	b := prepare("zzz mid\n", searchindex.Stop{
		Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{
			{Start: 0, End: 3, Bytes: []byte("hit")}, // text moved on
			{Start: 4, End: 7, Bytes: []byte("mid")}, // survives
		},
	})
	if !b.Stale() {
		t.Fatal("a dropped submatch did not mark the buffer stale")
	}
	want := []present.Span{{Start: 4, End: 7}}
	if got := b.Spans(0); len(got) != 1 || got[0] != want[0] {
		t.Fatalf("Spans(0) = %+v, want only the survivor %+v", got, want)
	}
	line, cell := b.RevealTarget(searchindex.Stop{Line: 1})
	if line != 0 || cell != 4 {
		t.Fatalf("RevealTarget = (%d, %d), want the first survivor's (0, 4)", line, cell)
	}
}

// A stale stop whose line still exists but has no surviving submatches
// lands at the first recorded start clamped to the line's raw bytes
// and mapped to a display cell. A mapping onto the end-of-line
// position falls back to the last rendered cell — the fallback invents
// no marker — and an empty line has no cell to clamp below 0.
func TestStaleFallbackClampsRecordedStart(t *testing.T) {
	for _, tc := range []struct {
		name       string
		data       string
		start, end int
		match      string
		wantCell   int
	}{
		{"recorded start mid-line", "zzzzzzz\n", 2, 5, "hit", 2},
		// Start on the removed LF maps to EOL — no marker paints there.
		{"recorded start on the terminator", "zz\n", 2, 5, "hit", 1},
		// A start past the raw line clamps to the end, which is EOL.
		{"recorded start past the line", "zz\n", 8, 11, "hit", 1},
		// No cells at all: the position lands on cell 0.
		{"empty line", "\n", 4, 7, "hit", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stop := searchindex.Stop{Path: []byte("f"), Line: 1,
				Submatches: []searchindex.Submatch{
					{Start: tc.start, End: tc.end, Bytes: []byte(tc.match)},
				}}
			b := prepare(tc.data, stop)
			if !b.Stale() {
				t.Fatalf("Prepare(%q) not stale — the submatch must drop", tc.data)
			}
			line, cell := b.RevealTarget(stop)
			if line != 0 || cell != tc.wantCell {
				t.Fatalf("RevealTarget = (%d, %d), want the clamped (0, %d)", line, cell, tc.wantCell)
			}
		})
	}
}

// The fallback lands where the clamped recorded start says even when
// the earliest recorded submatch is not the first in the slice — the
// start ordering of the recorded set decides, not slice position.
func TestStaleFallbackUsesEarliestRecordedStart(t *testing.T) {
	stop := searchindex.Stop{Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{
			{Start: 4, End: 7, Bytes: []byte("hit")},
			{Start: 0, End: 3, Bytes: []byte("hit")},
		}}
	b := prepare("zzzzzzzz\n", stop)
	line, cell := b.RevealTarget(stop)
	if line != 0 || cell != 0 {
		t.Fatalf("RevealTarget = (%d, %d), want the earliest recorded start (0, 0)", line, cell)
	}
}

// A stale stop whose line is gone lands at the last source line's
// start; an empty file has no lines at all, so its stop lands on the
// inert zero position of a zero-line panel.
func TestStaleFallbackMissingLineAndEmptyFile(t *testing.T) {
	gone := prepare("a\nb\nc\n", searchindex.Stop{Path: []byte("f"), Line: 9,
		Submatches: []searchindex.Submatch{{Start: 0, End: 1, Bytes: []byte("x")}}})
	if !gone.Stale() {
		t.Fatal("a stop on a missing line did not mark the buffer stale")
	}
	if line, cell := gone.RevealTarget(searchindex.Stop{Line: 9}); line != 2 || cell != 0 {
		t.Fatalf("RevealTarget on a missing line = (%d, %d), want the last line's start (2, 0)",
			line, cell)
	}

	empty := prepare("", searchindex.Stop{Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{{Start: 0, End: 3, Bytes: []byte("hit")}}})
	if empty.LineCount() != 0 {
		t.Fatalf("empty file LineCount = %d, want a zero-line panel", empty.LineCount())
	}
	if !empty.Stale() {
		t.Fatal("the empty file's vanished stop did not mark it stale")
	}
	if line, cell := empty.RevealTarget(searchindex.Stop{Line: 1}); line != 0 || cell != 0 {
		t.Fatalf("empty-file RevealTarget = (%d, %d), want the inert (0, 0)", line, cell)
	}
}

// Validation recomputes on every prepare — the reload path is a fresh
// Prepare over newly read bytes — so reverting the file clears the
// stale mark while a still-changed file keeps it, and a clean file can
// go stale on a later load.
func TestRevalidationRecomputesStale(t *testing.T) {
	stop := searchindex.Stop{Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{{Start: 0, End: 3, Bytes: []byte("hit")}}}

	if b := prepare("zzz\n", stop); !b.Stale() {
		t.Fatal("changed content did not mark the buffer stale")
	}
	if b := prepare("hit\n", stop); b.Stale() {
		t.Fatal("reverted content still stale — validation must recompute")
	}
	if b := prepare("zzz\n", stop); !b.Stale() {
		t.Fatal("a clean file changed again did not go stale")
	}
}

// Validation compares the recorded bytes against the line's raw bytes
// — never the escaped display text. A match covering the ESC byte
// validates against the raw span even though it displays as "^[", and
// a match on an invalid UTF-8 byte validates even though it displays
// as U+FFFD.
func TestValidationUsesOriginalNotDisplayBytes(t *testing.T) {
	esc := prepare("a\x1bb\n", searchindex.Stop{Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{{Start: 1, End: 2, Bytes: []byte("\x1b")}}})
	if esc.Stale() {
		t.Fatal("a match on the raw ESC byte dropped — validation must not see \"^[\"")
	}
	if got := esc.Spans(0); len(got) != 1 || got[0] != (present.Span{Start: 1, End: 3}) {
		t.Fatalf("Spans(0) = %+v, want the escape's cells %+v", got, present.Span{Start: 1, End: 3})
	}

	bad := prepare("a\xffb\n", searchindex.Stop{Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{{Start: 1, End: 2, Bytes: []byte("\xff")}}})
	if bad.Stale() {
		t.Fatal("a match on the raw invalid byte dropped — validation must not see U+FFFD")
	}
	if got := bad.Spans(0); len(got) != 1 || got[0] != (present.Span{Start: 1, End: 2}) {
		t.Fatalf("Spans(0) = %+v, want the replacement cell %+v", got, present.Span{Start: 1, End: 2})
	}
}

// A match solely covering a line's CRLF terminator validates — the
// recorded bytes equal the raw terminator bytes even though nothing
// displays for them — so the buffer stays clean and the match becomes
// the ordinary end-of-line marker.
func TestCRLFTerminatorMatchValidatesClean(t *testing.T) {
	b := prepare("hit\r\n", searchindex.Stop{Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{{Start: 3, End: 5, Bytes: []byte("\r\n")}}})
	if b.Stale() {
		t.Fatal("a terminator-only match marked the buffer stale")
	}
	if got := b.Spans(0); len(got) != 1 || got[0] != (present.Span{Start: 3, End: 3}) {
		t.Fatalf("Spans(0) = %+v, want the end-of-line marker %+v", got, present.Span{Start: 3, End: 3})
	}
}

// The leading UTF-8 BOM's coordinate split applies to validation and
// the fallback alike: rg offsets on line one shift by three into the
// raw view, so a changed line one still drops its submatch, and the
// clamped fallback start shifts past the BOM before mapping.
func TestStaleFallbackShiftsPastLeadingBOM(t *testing.T) {
	// rg's first line is "zzz\n": offset 0 names raw byte 3.
	b := prepare("\xef\xbb\xbfzzz\n", searchindex.Stop{Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{{Start: 0, End: 3, Bytes: []byte("hit")}}})
	if !b.Stale() {
		t.Fatal("the BOM-shifted submatch did not drop")
	}
	// Recorded start 0 shifts to raw byte 3 — the first 'z', cell 0.
	if line, cell := b.RevealTarget(searchindex.Stop{Line: 1,
		Submatches: []searchindex.Submatch{{Start: 0, End: 3, Bytes: []byte("hit")}}}); line != 0 || cell != 0 {
		t.Fatalf("BOM fallback RevealTarget = (%d, %d), want (0, 0)", line, cell)
	}

	// The same shift makes a matching submatch validate: raw bytes
	// 3-5 are "zzz", so the buffer stays clean.
	clean := prepare("\xef\xbb\xbfzzz\n", searchindex.Stop{Path: []byte("f"), Line: 1,
		Submatches: []searchindex.Submatch{{Start: 0, End: 3, Bytes: []byte("zzz")}}})
	if clean.Stale() {
		t.Fatal("a BOM-shifted validating submatch marked the buffer stale")
	}
}
