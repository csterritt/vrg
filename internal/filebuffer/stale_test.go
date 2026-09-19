package filebuffer_test

import (
	"os"
	"testing"

	"vrg/internal/searchindex"
)

// sub builds one recorded submatch: its byte range in rg-line
// coordinates and the bytes ripgrep reported for it.
func sub(start, end int, match string) searchindex.Submatch {
	return searchindex.Submatch{Start: start, End: end, Bytes: []byte(match)}
}

// stopSubs is one matched-line stop carrying recorded submatches — the
// form Issue #29's validation consumes: each submatch's bytes plus the
// union highlight coverage the index prepares.
func stopSubs(line int64, subs ...searchindex.Submatch) searchindex.Stop {
	st := searchindex.Stop{Number: line, Submatches: subs}
	for _, s := range subs {
		st.Highlights = append(st.Highlights, searchindex.Span{Start: s.Start, End: s.End})
	}
	return st
}

// rewrite replaces a fixture file's bytes between loads — the disk
// edit a reload observes.
func rewrite(t *testing.T, path []byte, content string) {
	t.Helper()
	if err := os.WriteFile(string(path), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Content whose every recorded submatch still holds is not stale and
// resolves its target to the first submatch's start cell.
func TestCleanContentNotStale(t *testing.T) {
	st := stopSubs(1, sub(0, 5, "alpha"), sub(11, 16, "alpha"))
	b := load(t, writeFile(t, "alpha beta alpha\n"), st)
	if b.Stale() {
		t.Fatal("fully validating content marked the buffer stale")
	}
	if line, cell := b.StopTarget(st); line != 1 || cell != 0 {
		t.Fatalf("StopTarget = (%d, %d), want (1, 0) — the first submatch's start cell",
			line, cell)
	}
}

// A submatch whose recorded range runs past the loaded line's bytes
// fails range validity: it is dropped — no highlight — and the buffer
// is marked stale.
func TestStaleOutOfBoundsRangeDrops(t *testing.T) {
	b := load(t, writeFile(t, "hit\n"), stopSubs(1, sub(0, 9, "hit")))
	if !b.Stale() {
		t.Fatal("an out-of-bounds submatch did not mark the buffer stale")
	}
	if got := b.Lines()[0].Highlights; len(got) != 0 {
		t.Fatalf("highlights = %v, want none — the out-of-bounds submatch dropped", got)
	}
}

// A same-length replacement: the loaded bytes occupy the recorded
// range but no longer equal the recorded submatch bytes — dropped,
// stale, no highlight.
func TestStaleSameLengthReplacementDrops(t *testing.T) {
	b := load(t, writeFile(t, "ALPHA beta\n"), stopSubs(1, sub(0, 5, "alpha")))
	if !b.Stale() {
		t.Fatal("a same-length replacement did not mark the buffer stale")
	}
	if got := b.Lines()[0].Highlights; len(got) != 0 {
		t.Fatalf("highlights = %v, want none — the mismatched submatch dropped", got)
	}
}

// One of two recorded submatches surviving keeps its highlight; the
// dropped one marks the buffer stale, and the reveal target is the
// first survivor's start cell.
func TestStalePartialSurvivalKeepsValidHighlight(t *testing.T) {
	// Recorded submatches [0,5) and [9,14) both "alpha"; the loaded
	// line reads "omega xx alpha" — the first dropped, the second
	// surviving.
	st := stopSubs(1, sub(0, 5, "alpha"), sub(9, 14, "alpha"))
	b := load(t, writeFile(t, "omega xx alpha\n"), st)
	if !b.Stale() {
		t.Fatal("a partially surviving stop did not mark the buffer stale")
	}
	got := b.Lines()[0].Highlights
	if len(got) != 1 || got[0].Start != 9 || got[0].End != 14 {
		t.Fatalf("highlights = %v, want [{9 14}] — the surviving submatch only", got)
	}
	if line, cell := b.StopTarget(st); line != 1 || cell != 9 {
		t.Fatalf("StopTarget = (%d, %d), want (1, 9) — the first survivor's start cell",
			line, cell)
	}
}

// A stop whose line exists but whose submatches all dropped lands at
// the first recorded start clamped to the line's bytes and mapped to
// a valid display cell — inventing no highlight.
func TestStaleAllDroppedClampedStart(t *testing.T) {
	st := stopSubs(1, sub(10, 15, "alpha"))
	b := load(t, writeFile(t, "totally different text\n"), st)
	if !b.Stale() {
		t.Fatal("an all-dropped stop did not mark the buffer stale")
	}
	if line, cell := b.StopTarget(st); line != 1 || cell != 10 {
		t.Fatalf("StopTarget = (%d, %d), want (1, 10) — the recorded start's cell",
			line, cell)
	}
	if got := b.Lines()[0].Highlights; len(got) != 0 {
		t.Fatalf("highlights = %v, want none — the fallback invents none", got)
	}
}

// A recorded start beyond the shortened line clamps to the available
// bytes: it maps to the end-of-line position, which — with no marker
// cell there — clamps to the last rendered cell.
func TestStaleClampedStartEOLFallbackLastCell(t *testing.T) {
	st := stopSubs(1, sub(10, 15, "alpha"))
	b := load(t, writeFile(t, "ab\n"), st)
	// The search view "ab\n" clamps start 10 to byte 3 — the
	// end-of-line position, cell 2 — and with no marker cell the
	// fallback lands on the last rendered cell, 1.
	if line, cell := b.StopTarget(st); line != 1 || cell != 1 {
		t.Fatalf("StopTarget = (%d, %d), want (1, 1) — the last rendered cell", line, cell)
	}
}

// A stop whose recorded line no longer exists lands at the last
// source line's start.
func TestStaleMissingLineLandsOnLastLine(t *testing.T) {
	st := stopSubs(5, sub(0, 5, "alpha"))
	b := load(t, writeFile(t, "one\ntwo\n"), st)
	if !b.Stale() {
		t.Fatal("a missing recorded line did not mark the buffer stale")
	}
	if line, cell := b.StopTarget(st); line != 2 || cell != 0 {
		t.Fatalf("StopTarget = (%d, %d), want (2, 0) — the last source line's start",
			line, cell)
	}
}

// An empty file stays a zero-line panel: the missing line marks the
// buffer stale and the stop keeps its bare recorded line — no rows
// exist to land on.
func TestStaleEmptyFileZeroLines(t *testing.T) {
	st := stopSubs(1, sub(0, 5, "alpha"))
	b := load(t, writeFile(t, ""), st)
	if b.LineCount() != 0 || len(b.Lines()) != 0 {
		t.Fatalf("empty file lines = %d/%d, want a zero-line panel",
			b.LineCount(), len(b.Lines()))
	}
	if !b.Stale() {
		t.Fatal("an empty file did not mark the buffer stale")
	}
	if line, cell := b.StopTarget(st); line != 1 || cell != 0 {
		t.Fatalf("StopTarget = (%d, %d), want (1, 0) — the bare recorded line",
			line, cell)
	}
}

// Staleness is recomputed on every load: still-mismatched content
// keeps the mark while fully validating content clears it — the note
// disappears only on clean content.
func TestStaleRecomputedPerLoad(t *testing.T) {
	st := stopSubs(1, sub(0, 5, "alpha"))
	path := writeFile(t, "omega\n")
	if b := load(t, path, st); !b.Stale() {
		t.Fatal("mismatched content did not mark the buffer stale")
	}
	rewrite(t, path, "alpha\n")
	if b := load(t, path, st); b.Stale() {
		t.Fatal("fully validating content stayed stale")
	}
	rewrite(t, path, "omegA\n")
	if b := load(t, path, st); !b.Stale() {
		t.Fatal("still-mismatched content did not keep the buffer stale")
	}
}

// Validation compares the recorded submatch bytes against the line's
// search-byte view — never the stripped display text: a match on a
// control byte displays as an escape yet validates by its raw byte.
func TestStaleValidationComparesSearchBytesNotDisplay(t *testing.T) {
	// \x1b displays as the three-cell escape "^[". The recorded byte
	// is the raw ESC; comparing display text would never match.
	b := load(t, writeFile(t, "a\x1bb\n"), stopSubs(1, sub(1, 2, "\x1b")))
	if b.Stale() {
		t.Fatal("the control-byte match did not validate against raw search bytes")
	}
	got := b.Lines()[0].Highlights
	if len(got) != 1 || got[0].Start != 1 || got[0].End != 3 {
		t.Fatalf("highlights = %v, want [{1 3}] covering the ^[ escape", got)
	}
}

// A match recorded on the removed CRLF terminator bytes — undisplayed
// but retained in the search-byte view — still validates and lands on
// the end-of-line marker.
func TestStaleCRLFTerminatorMatchValidates(t *testing.T) {
	b := load(t, writeFile(t, "hit\r\n"), stopSubs(1, sub(3, 5, "\r\n")))
	if b.Stale() {
		t.Fatal("the CRLF terminator match did not validate")
	}
	l := b.Lines()[0]
	if len(l.Highlights) != 1 || l.Highlights[0].Start != 3 || l.Highlights[0].End != 3 {
		t.Fatalf("highlights = %v, want the empty span {3 3} — the end-of-line marker",
			l.Highlights)
	}
	if !l.MarkerAt(3) {
		t.Fatal("no marker at the end-of-line cell")
	}
}

// The Issue #22 BOM adjustment applies before validation: rg-line
// offsets index the BOM-stripped search view, so a recorded first-line
// match validates at [0,3) while the unadjusted raw-file range misses
// the adjustment entirely.
func TestStaleBOMAdjustedValidation(t *testing.T) {
	b := load(t, writeFile(t, "\xef\xbb\xbfhit\n"), stopSubs(1, sub(0, 3, "hit")))
	if b.Stale() {
		t.Fatal("the rg-coordinate match on a BOM line did not validate")
	}
	b2 := load(t, writeFile(t, "\xef\xbb\xbfhit\n"), stopSubs(1, sub(3, 6, "hit")))
	if !b2.Stale() {
		t.Fatal("an unadjusted range on a BOM line did not mark the buffer stale")
	}
}

// A recorded zero-width submatch validates trivially — its recorded
// bytes are the empty range — and survives as a marker.
func TestStaleZeroWidthSurvives(t *testing.T) {
	b := load(t, writeFile(t, "hit\n"), stopSubs(1, sub(3, 3, "")))
	if b.Stale() {
		t.Fatal("the zero-width match did not validate")
	}
	if !b.Lines()[0].MarkerAt(3) {
		t.Fatal("no marker at the end-of-line cell")
	}
}
