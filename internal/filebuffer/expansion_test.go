package filebuffer_test

import (
	"path/filepath"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// A nonempty coverage range that partially covers a grapheme cluster
// expands outward to the whole cluster: the recorded span is the
// cluster-expanded cell span Viewport and App consume. The fixture's
// "e" + combining acute cluster holds bytes [3,6) and cells [3,5) of
// "caféx".
func TestPartialClusterSpanExpands(t *testing.T) {
	content := "caféx\n"
	for _, tc := range []struct {
		name       string
		start, end int
		want       filebuffer.Span
	}{
		{"span starts inside the cluster", 4, 7, filebuffer.Span{Start: 3, End: 6}},
		{"span ends inside the cluster", 0, 4, filebuffer.Span{Start: 0, End: 5}},
		{"span inside the cluster", 4, 5, filebuffer.Span{Start: 3, End: 5}},
		{"base byte only still covers the whole cluster", 3, 4, filebuffer.Span{Start: 3, End: 5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), content)
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
				stop(1, searchindex.Range{Start: tc.start, End: tc.end}),
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got := buf.Highlights(0)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("Highlights(0) = %+v, want [%+v]", got, tc.want)
			}
		})
	}
}

// A match consisting only of the combining marks inside a base cluster
// highlights that whole cluster: the combining acute's bytes [4,6)
// expand to the "e" + acute cluster's cells [3,5).
func TestCombiningOnlyMatchHighlightsWholeCluster(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "caféx\n")
	buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
		stop(1, searchindex.Range{Start: 4, End: 6}),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := buf.Highlights(0)
	want := filebuffer.Span{Start: 3, End: 5}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("Highlights(0) = %+v, want [%+v] — the whole base cluster", got, want)
	}
}

// A cluster with no base or independent visible cell — standalone
// combining marks — gains the recorded fallback cell: U+25CC DOTTED
// CIRCLE before the cluster's own bytes, occupying exactly one cell
// whose byte mapping still resolves to the original source bytes. A
// highlight over the cluster is therefore never zero cells.
func TestStandaloneCombiningClusterFallbackCell(t *testing.T) {
	for _, tc := range []struct {
		name      string
		content   string
		matchEnd  int
		wantFirst string
	}{
		{"single standalone mark", "́x\n", 2, "◌́"},
		{"standalone mark run", "́̂x\n", 4, "◌́̂"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), tc.content)
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
				stop(1, searchindex.Range{Start: 0, End: tc.matchEnd}),
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			cells := buf.Cells(0)
			if len(cells) != 2 || cells[0].Text != tc.wantFirst ||
				cells[0].Start != 0 || cells[0].End != tc.matchEnd {
				t.Fatalf("Cells(0) = %+v, want a %q fallback cell on bytes [0,%d) then x",
					cells, tc.wantFirst, tc.matchEnd)
			}
			clusters := buf.Clusters(0)
			want := []safepresentation.Cluster{{Start: 0, End: 1, Width: 1}, {Start: 1, End: 2, Width: 1}}
			if len(clusters) != len(want) || clusters[0] != want[0] || clusters[1] != want[1] {
				t.Fatalf("Clusters(0) = %+v, want %+v — the fallback is one real cell",
					clusters, want)
			}
			got := buf.Highlights(0)
			if len(got) != 1 || got[0] != (filebuffer.Span{Start: 0, End: 1}) {
				t.Fatalf("Highlights(0) = %+v, want [{0 1}] — the one fallback cell", got)
			}
		})
	}
}

// A two-cell-wide glyph is never split by a highlight boundary: a
// coverage range reaching only part of 世's bytes [1,4) still records
// the whole cluster's cell range.
func TestWideClusterSpanNeverSplits(t *testing.T) {
	content := "a世b\n"
	for _, tc := range []struct {
		name       string
		start, end int
	}{
		{"trailing bytes of the rune", 2, 4},
		{"leading byte of the rune", 1, 2},
		{"middle of the rune", 2, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), content)
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
				stop(1, searchindex.Range{Start: tc.start, End: tc.end}),
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got := buf.Highlights(0)
			want := filebuffer.Span{Start: 1, End: 2}
			if len(got) != 1 || got[0] != want {
				t.Fatalf("Highlights(0) = %+v, want [%+v] — 世's whole cell", got, want)
			}
		})
	}
}

// An emoji ZWJ sequence is one cluster under the shared policy: a match
// on any member's bytes — a whole emoji or a bare joiner — expands to
// the sequence's entire cell range.
func TestZWJSequenceSpanExpands(t *testing.T) {
	content := "x👨‍👩‍👧y\n"
	for _, tc := range []struct {
		name       string
		start, end int
	}{
		{"one emoji of the sequence", 8, 12},  // 👩
		{"a bare joiner", 5, 8},               // first ZWJ
		{"the sequence's tail bytes", 15, 19}, // 👧
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), content)
			buf, err := filebuffer.Load([]byte(path), []searchindex.Stop{
				stop(1, searchindex.Range{Start: tc.start, End: tc.end}),
			})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got := buf.Highlights(0)
			want := filebuffer.Span{Start: 1, End: 6}
			if len(got) != 1 || got[0] != want {
				t.Fatalf("Highlights(0) = %+v, want [%+v] — the whole ZWJ cluster", got, want)
			}
		})
	}
}

// The expanded span is the buffer's sole span source: a stop whose
// recorded bytes start mid-cluster hands Viewport the cluster's start
// cell, so reveal and indicators count from the cluster start.
func TestMidClusterCoverageTargetsClusterStart(t *testing.T) {
	path := writeFile(t, filepath.Join(t.TempDir(), "f.txt"), "caféx\n")
	buf, err := filebuffer.Load([]byte(path), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Bytes [4,6) are the combining mark alone inside the "e" + acute
	// cluster; the target is the cluster's first cell, not the cell the
	// raw byte offset would suggest.
	line, cell := buf.TargetCell(stop(1, searchindex.Range{Start: 4, End: 6}))
	if line != 0 || cell != 3 {
		t.Fatalf("TargetCell = (%d, %d), want (0, 3) — the cluster's start cell", line, cell)
	}
}
