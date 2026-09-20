package filebuffer_test

import (
	"path/filepath"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// markerCell returns the cells of line i's last cluster when a marker
// sits at the line's end.
func markerCell(buf *filebuffer.Buffer, i int) safepresentation.Cell {
	cells := buf.Cells(i)
	return cells[len(cells)-1]
}

// A zero-width submatch at a text position marks the existing cell
// there: the line keeps its cells — following text is not shifted —
// and the marked cell is painted as one inverse cell, underlined on the
// current matched line by the same span machinery as any match.
func TestZeroWidthMarkerMarksExistingCell(t *testing.T) {
	for _, tc := range []struct {
		name       string
		start      int
		wantMarker int
		wantSpan   filebuffer.Span
	}{
		{"at beginning of line", 0, 0, filebuffer.Span{Start: 0, End: 1}},
		{"mid-line", 1, 1, filebuffer.Span{Start: 1, End: 2}},
		{"on the last cell", 2, 2, filebuffer.Span{Start: 2, End: 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "abc\n")
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
				stop(1, searchindex.Range{Start: tc.start, End: tc.start}),
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := text(buf.Cells(0)); got != "abc" {
				t.Fatalf("line 0 = %q, want %q — no shift, no added cell", got, "abc")
			}
			if got := buf.Markers(0); len(got) != 1 || got[0] != tc.wantMarker {
				t.Fatalf("Markers(0) = %v, want [%d]", got, tc.wantMarker)
			}
			if got := buf.Highlights(0); len(got) != 1 || got[0] != tc.wantSpan {
				t.Fatalf("Highlights(0) = %+v, want [%+v] — the one marked cell",
					got, tc.wantSpan)
			}
		})
	}
}

// A zero-width position inside a grapheme cluster maps to the cluster's
// start cell: the marker marks that one cell and the cluster is never
// split — no half-drawn wide glyph, no partially marked escaped form.
func TestZeroWidthMarkerInsideClusterMapsToClusterStart(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		start   int
		want    int
	}{
		// 世 holds bytes [1,4) and cell 1 of "a世b".
		{"inside a wide rune", "a世b\n", 2, 1},
		{"at the wide rune's last byte", "a世b\n", 3, 1},
		// The "e" + combining acute cluster holds bytes [3,6) and cells
		// [3,5) of "caféx"; the marker lands on the cluster's first cell.
		{"inside a combining cluster", "caféx\n", 4, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), tc.content)
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
				stop(1, searchindex.Range{Start: tc.start, End: tc.start}),
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := buf.Markers(0); len(got) != 1 || got[0] != tc.want {
				t.Fatalf("Markers(0) = %v, want [%d] — the cluster's start cell", got, tc.want)
			}
			want := filebuffer.Span{Start: tc.want, End: tc.want + 1}
			if got := buf.Highlights(0); len(got) != 1 || got[0] != want {
				t.Fatalf("Highlights(0) = %+v, want [%+v] — one marked cell", got, want)
			}
		})
	}
}

// A zero-width match at end of line appends one marker cell — a space
// painted inverse — extending the line's effective width by one cell.
// The marker is a real cell and cluster, so wrap, clip, extent, and
// indicator rules treat it like any other cell.
func TestZeroWidthMarkerAtEndOfLine(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{"LF-terminated line", "abc\n"},
		{"CRLF-terminated line", "abc\r\n"},
		{"unterminated final line", "abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), tc.content)
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
				stop(1, searchindex.Range{Start: 3, End: 3}),
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := text(buf.Cells(0)); got != "abc " {
				t.Fatalf("line 0 = %q, want %q — the marker is one appended space cell",
					got, "abc ")
			}
			if c := markerCell(buf, 0); c.Text != " " {
				t.Fatalf("marker cell = %q, want a space", c.Text)
			}
			clusters := buf.Clusters(0)
			last := clusters[len(clusters)-1]
			if want := (safepresentation.Cluster{Start: 3, End: 4, Width: 1}); last != want {
				t.Fatalf("last cluster = %+v, want %+v — the one-cell marker", last, want)
			}
			if got := buf.Markers(0); len(got) != 1 || got[0] != 3 {
				t.Fatalf("Markers(0) = %v, want [3]", got)
			}
			if got := buf.Highlights(0); len(got) != 1 || got[0] != (filebuffer.Span{Start: 3, End: 4}) {
				t.Fatalf("Highlights(0) = %+v, want [{3 4}] — the marker cell", got)
			}
		})
	}
}

// An empty matched line has effective width one: its zero-width marker
// is the line's only cell.
func TestZeroWidthMarkerOnEmptyLine(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "a\n\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(2, searchindex.Range{Start: 0, End: 0}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cells := buf.Cells(1)
	if len(cells) != 1 || cells[0].Text != " " {
		t.Fatalf("Cells(1) = %+v, want the single marker space cell", cells)
	}
	clusters := buf.Clusters(1)
	if len(clusters) != 1 || clusters[0] != (safepresentation.Cluster{Start: 0, End: 1, Width: 1}) {
		t.Fatalf("Clusters(1) = %+v, want the one marker cluster", clusters)
	}
	if got := buf.Markers(1); len(got) != 1 || got[0] != 0 {
		t.Fatalf("Markers(1) = %v, want [0]", got)
	}
}

// A match solely on removed terminator bytes becomes an ordinary
// end-of-line marker — not a special case: `$` at byte 4 of "hit\r\n"
// lands at display column 3, as do matches covering only the CR, only
// the LF, or the whole CRLF, and the same holds for LF lines.
func TestTerminatorOnlyMatchIsEndOfLineMarker(t *testing.T) {
	for _, tc := range []struct {
		name       string
		content    string
		start, end int
	}{
		{"zero-width at the LF of CRLF", "hit\r\n", 4, 4},
		{"zero-width at the CR of CRLF", "hit\r\n", 3, 3},
		{"covering the CR only", "hit\r\n", 3, 4},
		{"covering the LF only", "hit\r\n", 4, 5},
		{"covering all of CRLF", "hit\r\n", 3, 5},
		{"zero-width past the line", "hit\r\n", 5, 5},
		{"zero-width at LF", "hit\n", 3, 3},
		{"covering LF", "hit\n", 3, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), tc.content)
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
				stop(1, searchindex.Range{Start: tc.start, End: tc.end}),
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := buf.Markers(0); len(got) != 1 || got[0] != 3 {
				t.Fatalf("Markers(0) = %v, want [3] — display column 3", got)
			}
			cells := buf.Cells(0)
			if len(cells) != 4 || cells[3].Text != " " {
				t.Fatalf("Cells(0) = %+v, want hit plus one marker cell", cells)
			}
			if got := buf.Highlights(0); len(got) != 1 || got[0] != (filebuffer.Span{Start: 3, End: 4}) {
				t.Fatalf("Highlights(0) = %+v, want [{3 4}] — the marker cell", got)
			}
		})
	}
}

// Distinct byte positions in the terminator region — at the CR, at the
// LF, past the line's end — all map to the one display end-of-line
// position, so their markers coalesce onto a single marker cell.
func TestTerminatorRegionMarkersCoalesce(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "hit\r\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 4, End: 4}),
		stop(1, searchindex.Range{Start: 5, End: 5}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := buf.Markers(0); len(got) != 1 || got[0] != 3 {
		t.Fatalf("Markers(0) = %v, want [3] — one coalesced end-of-line marker", got)
	}
	if cells := buf.Cells(0); len(cells) != 4 {
		t.Fatalf("Cells(0) = %+v, want hit plus exactly one marker cell", cells)
	}
}

// Markers at distinct positions — one inside the line, one at its end —
// coexist: the interior marker marks its cell and the end-of-line
// marker appends one cell.
func TestInteriorAndEndOfLineMarkersCoexist(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "abc\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 0, End: 0}, searchindex.Range{Start: 3, End: 3}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := text(buf.Cells(0)); got != "abc " {
		t.Fatalf("line 0 = %q, want %q", got, "abc ")
	}
	if got := buf.Markers(0); len(got) != 2 || got[0] != 0 || got[1] != 3 {
		t.Fatalf("Markers(0) = %v, want [0 3]", got)
	}
	want := []filebuffer.Span{{Start: 0, End: 1}, {Start: 3, End: 4}}
	got := buf.Highlights(0)
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Highlights(0) = %+v, want %+v", got, want)
	}
}

// A terminator-region position on a UTF-8-BOM first line still lands on
// the end-of-line marker: rg offset 4 of rg's "hit\r\n" view is raw
// byte 7, mapping to the marker cell at display column 3.
func TestTerminatorMarkerOnBOMLine(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "\xEF\xBB\xBFhit\r\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 4, End: 4}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := buf.Markers(0); len(got) != 1 || got[0] != 3 {
		t.Fatalf("Markers(0) = %v, want [3]", got)
	}
	if cells := buf.Cells(0); len(cells) != 4 || cells[3].Text != " " {
		t.Fatalf("Cells(0) = %+v, want hit plus the marker cell", cells)
	}
}

// A marker cell is a navigable reveal target: the stop's display target
// is the marker cell itself — column 3 for `$` on "hit\r\n".
func TestMarkerCellIsTheDisplayTarget(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "hit\r\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 4, End: 4}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	line, cell := buf.TargetCell(stop(1, searchindex.Range{Start: 4, End: 4}))
	if line != 0 || cell != 3 {
		t.Fatalf("TargetCell = (%d, %d), want (0, 3) — the marker cell", line, cell)
	}
}

// Lines without marker coverage report no markers.
func TestNoMarkersWithoutZeroWidthCoverage(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "abc\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 0, End: 2}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := buf.Markers(0); len(got) != 0 {
		t.Fatalf("Markers(0) = %v, want none", got)
	}
	if got := buf.Markers(9); len(got) != 0 {
		t.Fatalf("Markers(9) = %v, want none out of range", got)
	}
}
