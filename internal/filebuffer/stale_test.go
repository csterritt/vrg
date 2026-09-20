package filebuffer_test

import (
	"path/filepath"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// sub builds one recorded ripgrep submatch: its byte range in rg's view
// of the line and the match bytes ripgrep reported. Both JSON blob
// forms — text and base64 bytes — land in Text before a stop exists,
// so validating against Text covers either encoding.
func sub(start, end int, text string) searchindex.Submatch {
	return searchindex.Submatch{
		Range: searchindex.Range{Start: start, End: end},
		Text:  []byte(text),
	}
}

// subStop builds a matched-line stop carrying recorded submatches — the
// stale-validation subject — where the stop helper's Coverage-only
// fixture cannot express the recorded bytes validation needs.
func subStop(line int64, subs ...searchindex.Submatch) searchindex.Stop {
	return searchindex.Stop{Line: line, Submatches: subs}
}

// Every recorded submatch is checked on load against the loaded line's
// retained original bytes: an out-of-bounds range or a byte mismatch
// drops that submatch — no highlight, no marker — and marks the buffer
// stale.
func TestStaleValidationDropsFailedSubmatches(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		line    int64
		sub     searchindex.Submatch
	}{
		{"same-length replacement", "hat a\n", 1, sub(0, 3, "hit")},
		{"range end beyond the line's bytes", "hit\n", 1, sub(0, 99, "hit")},
		{"range start beyond the line's bytes", "hit\n", 1, sub(10, 12, "xx")},
		{"zero-width range carrying bytes", "hit\n", 1, sub(4, 4, "x")},
		{"bytes differing inside the line", "hit\n", 1, sub(1, 3, "IT")},
		{"display-equal text with different bytes", "café\n", 1, sub(0, 4, "cafe")},
		{"missing line", "hit\n", 9, sub(0, 3, "hit")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), tc.content)
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
				subStop(tc.line, tc.sub),
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !buf.Stale() {
				t.Fatal("buffer with a dropped submatch is not stale")
			}
			i := int(tc.line) - 1
			if i >= 0 && i < buf.LineCount() {
				if got := buf.Highlights(i); len(got) != 0 {
					t.Fatalf("Highlights(%d) = %+v, want the dropped submatch gone", i, got)
				}
				if got := buf.Markers(i); len(got) != 0 {
					t.Fatalf("Markers(%d) = %v, want the dropped submatch gone", i, got)
				}
			}
		})
	}
}

// A clean load — every recorded submatch validating — is not stale.
func TestCleanLoadIsNotStale(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "hit me\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		subStop(1, sub(0, 3, "hit")),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.Stale() {
		t.Fatal("a fully validating buffer is stale")
	}
	if got := buf.Highlights(0); len(got) != 1 || got[0] != (filebuffer.Span{Start: 0, End: 3}) {
		t.Fatalf("Highlights(0) = %+v, want [{0 3}]", got)
	}
}

// A dropped submatch never takes its siblings down: the survivors keep
// their highlights while the buffer is still marked stale.
func TestStalePartialSurvivalKeepsValidHighlights(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "hit xx hit\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		subStop(1, sub(0, 3, "hit"), sub(4, 6, "yy"), sub(7, 10, "hit")),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !buf.Stale() {
		t.Fatal("buffer with one dropped submatch is not stale")
	}
	want := []filebuffer.Span{{Start: 0, End: 3}, {Start: 7, End: 10}}
	got := buf.Highlights(0)
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Highlights(0) = %+v, want %+v — the survivors", got, want)
	}
}

// A stale stop that still has a surviving submatch exposes that
// survivor — not the dropped first recorded submatch — as its reveal
// target.
func TestStaleSurvivorIsRevealTarget(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "bbbb zzzz\n")
	st := subStop(1, sub(0, 4, "aaaa"), sub(5, 9, "zzzz"))
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{st})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !buf.Stale() {
		t.Fatal("buffer with a dropped first submatch is not stale")
	}
	if line, cell := buf.TargetCell(st); line != 0 || cell != 5 {
		t.Fatalf("TargetCell = (%d, %d), want (0, 5) — the first survivor's start", line, cell)
	}
	if got := buf.Highlights(0); len(got) != 1 || got[0] != (filebuffer.Span{Start: 5, End: 9}) {
		t.Fatalf("Highlights(0) = %+v, want [{5 9}] — the survivor alone", got)
	}
}

// A stale stop whose line exists but has no surviving submatch lands on
// the first recorded start clamped to the available line bytes and
// mapped to a valid display cell; an end-of-line position clamps to the
// last rendered cell when no marker cell sits there. The fallback
// invents no highlight and no marker.
func TestStaleClampedStartFallback(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		sub     searchindex.Submatch
		want    int
	}{
		// The recorded start lands inside 'y': byte 1 is that cell.
		{"recorded start inside the line", "xyz\n", sub(1, 4, "abc"), 1},
		// Byte 2 of "a世b" is inside the 世 cluster's cell.
		{"recorded start inside a cluster", "a世b\n", sub(2, 3, "x"), 1},
		// The recorded start is beyond the line entirely: clamped to
		// its end, then to the last rendered cell — no marker exists
		// to land on.
		{"recorded start past the line end", "xyz\n", sub(50, 53, "abc"), 2},
		// The recorded start sits in the terminator: mapped to the
		// end-of-line position, then clamped back to the last cell.
		{"recorded start in the terminator", "hit\n", sub(4, 6, "xy"), 2},
		{"recorded start in the CRLF terminator", "hit\r\n", sub(3, 5, "XY"), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), tc.content)
			st := subStop(1, tc.sub)
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{st})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !buf.Stale() {
				t.Fatal("buffer with no surviving submatch is not stale")
			}
			if line, cell := buf.TargetCell(st); line != 0 || cell != tc.want {
				t.Fatalf("TargetCell = (%d, %d), want (0, %d) — the clamped start",
					line, cell, tc.want)
			}
			if got := buf.Highlights(0); len(got) != 0 {
				t.Fatalf("Highlights(0) = %+v, want none — no invented highlight", got)
			}
			if got := buf.Markers(0); len(got) != 0 {
				t.Fatalf("Markers(0) = %v, want none — no invented marker", got)
			}
		})
	}
}

// A stale stop whose recorded line is gone lands at the last source
// line's start, with nothing highlighted anywhere.
func TestStaleMissingLineLandsAtLastLine(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "a\nb\nc\n")
	st := subStop(9, sub(0, 3, "hit"))
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{st})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !buf.Stale() {
		t.Fatal("buffer with a missing recorded line is not stale")
	}
	if line, cell := buf.TargetCell(st); line != 2 || cell != 0 {
		t.Fatalf("TargetCell = (%d, %d), want (2, 0) — the last line's start", line, cell)
	}
	for i := 0; i < buf.LineCount(); i++ {
		if got := buf.Highlights(i); len(got) != 0 {
			t.Fatalf("Highlights(%d) = %+v, want none", i, got)
		}
	}
}

// An empty file stays a zero-line panel: the stale stop targets the
// zero position and nothing renders.
func TestStaleEmptyFileRemainsZeroLinePanel(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "")
	st := subStop(1, sub(0, 3, "hit"))
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{st})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.LineCount() != 0 {
		t.Fatalf("LineCount = %d, want 0", buf.LineCount())
	}
	if !buf.Stale() {
		t.Fatal("an empty file against a recorded submatch is not stale")
	}
	if line, cell := buf.TargetCell(st); line != 0 || cell != 0 {
		t.Fatalf("TargetCell = (%d, %d), want (0, 0)", line, cell)
	}
}

// Validation compares the recorded search bytes with the retained
// original line bytes — terminator included — never the stripped
// display text: a match on the ESC byte validates and covers its ^[
// cells, and a match on a CRLF terminator validates into the ordinary
// end-of-line marker.
func TestStaleValidationUsesOriginalAndSearchBytes(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "a\x1bb\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		subStop(1, sub(1, 2, "\x1b")),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.Stale() {
		t.Fatal("a submatch on the raw ESC byte did not validate against the original bytes")
	}
	if got := text(buf.Cells(0)); got != "a^[b" {
		t.Fatalf("line 0 = %q, want %q", got, "a^[b")
	}
	if got := buf.Highlights(0); len(got) != 1 || got[0] != (filebuffer.Span{Start: 1, End: 3}) {
		t.Fatalf("Highlights(0) = %+v, want [{1 3}] — the ^[ cells", got)
	}

	path = writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "hit\r\n")
	st := subStop(1, sub(3, 5, "\r\n"))
	buf, err = filebuffer.Load([]byte(path), []searchindex.Stop{st})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.Stale() {
		t.Fatal("a submatch on the CRLF terminator did not validate")
	}
	if got := buf.Markers(0); len(got) != 1 || got[0] != 3 {
		t.Fatalf("Markers(0) = %v, want [3] — the end-of-line marker", got)
	}
	if line, cell := buf.TargetCell(st); line != 0 || cell != 3 {
		t.Fatalf("TargetCell = (%d, %d), want (0, 3) — the marker cell", line, cell)
	}
}

// A UTF-8-BOM first line validates rg-view ranges against the raw bytes
// shifted by the BOM's three bytes — never against the unadjusted raw
// start — and a BOM-line mismatch still drops the submatch. Later lines
// carry no shift.
func TestStaleBOMLineValidation(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "\xEF\xBB\xBFhit\nnext\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		subStop(1, sub(0, 3, "hit")),
		subStop(2, sub(0, 4, "next")),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.Stale() {
		t.Fatal("BOM-line submatches did not validate against the shifted raw bytes")
	}
	if got := buf.Highlights(0); len(got) != 1 || got[0] != (filebuffer.Span{Start: 0, End: 3}) {
		t.Fatalf("Highlights(0) = %+v, want [{0 3}]", got)
	}
	if got := buf.Highlights(1); len(got) != 1 || got[0] != (filebuffer.Span{Start: 0, End: 4}) {
		t.Fatalf("Highlights(1) = %+v, want [{0 4}]", got)
	}

	// rg offset 0 of a BOM first line is raw byte 3: "hat" there does
	// not equal the recorded "hit".
	path = writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "\xEF\xBB\xBFhat\n")
	buf, err = filebuffer.Load([]byte(path), []searchindex.Stop{
		subStop(1, sub(0, 3, "hit")),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !buf.Stale() {
		t.Fatal("a BOM-line byte mismatch is not stale")
	}
	if got := buf.Highlights(0); len(got) != 0 {
		t.Fatalf("Highlights(0) = %+v, want none", got)
	}

	// Range validity also counts in rg view: offset 4..7 of rg's
	// "hit\n" is raw 7..10 — past the retained 7 bytes.
	path = writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "\xEF\xBB\xBFhit\n")
	buf, err = filebuffer.Load([]byte(path), []searchindex.Stop{
		subStop(1, sub(4, 7, "xyz")),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !buf.Stale() {
		t.Fatal("a BOM-adjusted out-of-bounds submatch is not stale")
	}
}

// Staleness is recomputed on every load: a reload that finds corrected
// content validates cleanly, and one that finds still-different content
// stays stale — the note clears only on fully validating content.
func TestStaleRecomputedOnEveryLoad(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "zzz\n")
	stops := []searchindex.Stop{subStop(1, sub(0, 3, "hit"))}

	buf, err := filebuffer.Load([]byte(path), stops)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !buf.Stale() {
		t.Fatal("mismatched content is not stale")
	}
	if got := buf.Highlights(0); len(got) != 0 {
		t.Fatalf("Highlights(0) = %+v, want none", got)
	}

	writeFile(t, path, "hit\n")
	buf, err = filebuffer.Load([]byte(path), stops)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.Stale() {
		t.Fatal("fully validating reloaded content is stale")
	}
	if got := buf.Highlights(0); len(got) != 1 || got[0] != (filebuffer.Span{Start: 0, End: 3}) {
		t.Fatalf("Highlights(0) = %+v, want [{0 3}]", got)
	}

	writeFile(t, path, "hat\n")
	buf, err = filebuffer.Load([]byte(path), stops)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !buf.Stale() {
		t.Fatal("still-different reloaded content is not stale")
	}
}
