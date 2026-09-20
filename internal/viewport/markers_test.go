package viewport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// bufferOf loads a real FileBuffer over content with the given stops:
// marker fixtures need the buffer's appended marker cells, clusters,
// and spans, which the counting fake source cannot inject.
func bufferOf(t *testing.T, content string, stops ...searchindex.Stop) *filebuffer.Buffer {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load([]byte(path), stops)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return buf
}

// A marker-only line has content extent one and a paintable-boundary
// maximum of zero: the marker cluster starts at column 0 and is the
// only cell, so the line is fully painted at offset 0 and no positive
// offset is reachable.
func TestMarkerOnlyLineExtentClampsToZero(t *testing.T) {
	src := bufferOf(t, "\n", stopAt(1, 0, 0))
	m := NewRowModel(RowModelKey{TextWidth: 10}, src)
	if m.LineCount() != 1 {
		t.Fatalf("LineCount = %d, want 1", m.LineCount())
	}
	if got := rowText(m, 0); got != " " {
		t.Fatalf("row 0 = %q, want the one marker cell", got)
	}
	if spans := m.Highlights(0); len(spans) != 1 || spans[0] != (filebuffer.Span{Start: 0, End: 1}) {
		t.Fatalf("row 0 spans = %+v, want [{0 1}] — the marker paints inverse", spans)
	}
	v := &Viewport{}
	v.SetLayout(m, 5)
	v.Pan(100)
	if got := v.Offset(); got != 0 {
		t.Fatalf("pan on a marker-only line: offset = %d, want the clamped 0", got)
	}
}

// An end-of-line marker extends the line's effective width by one cell
// and feeds the paintable-boundary maximum like any other cluster: an
// exactly-full line pans one column further so the marker is the last
// fitting start, and at the maximum the marker paints alone.
func TestEndOfLineMarkerExtendsPaintableBoundary(t *testing.T) {
	line := strings.Repeat("x", 10)
	src := bufferOf(t, line+"\n", stopAt(1, 10, 10))
	m := NewRowModel(RowModelKey{TextWidth: 10}, src)
	v := &Viewport{}
	v.SetLayout(m, 5)
	v.Pan(100)
	if got := v.Offset(); got != 10 {
		t.Fatalf("offset = %d, want 10 — the marker's start column", got)
	}
	cells, spans := m.Clip(0, 10, 10)
	if got := cellsText(cells); got != " " {
		t.Fatalf("clip at the maximum = %q, want the marker cell alone", got)
	}
	if len(spans) != 1 || spans[0] != (filebuffer.Span{Start: 0, End: 1}) {
		t.Fatalf("clip spans = %+v, want [{0 1}]", spans)
	}
}

// A marker after a completely full wrap row occupies another row: the
// one-cell marker cluster cannot fit the full row and wraps whole,
// like any cluster.
func TestMarkerAfterFullWrapRowOccupiesAnotherRow(t *testing.T) {
	line := strings.Repeat("x", 10)
	src := bufferOf(t, line+"\n", stopAt(1, 10, 10))
	m := NewRowModel(RowModelKey{TextWidth: 10, Wrap: true}, src)
	if m.LineCount() != 2 {
		t.Fatalf("LineCount = %d, want 2 — the marker wraps to its own row", m.LineCount())
	}
	if got := rowText(m, 0); got != line {
		t.Fatalf("row 0 = %q, want %q", got, line)
	}
	if got := rowText(m, 1); got != " " {
		t.Fatalf("row 1 = %q, want the marker cell", got)
	}
	if spans := m.Highlights(1); len(spans) != 1 || spans[0] != (filebuffer.Span{Start: 0, End: 1}) {
		t.Fatalf("row 1 spans = %+v, want [{0 1}]", spans)
	}
	if got := m.TargetRow(stopAt(1, 10, 10)); got != 1 {
		t.Fatalf("TargetRow = %d, want 1 — the marker's own row", got)
	}
}

// A marker at end of a wrap row that still has room joins that row: the
// effective row width grows by the marker's one cell.
func TestMarkerJoinsUnfullWrapRow(t *testing.T) {
	src := bufferOf(t, "abc\n", stopAt(1, 3, 3))
	m := NewRowModel(RowModelKey{TextWidth: 10, Wrap: true}, src)
	if m.LineCount() != 1 {
		t.Fatalf("LineCount = %d, want 1", m.LineCount())
	}
	if got := rowText(m, 0); got != "abc " {
		t.Fatalf("row 0 = %q, want %q — text plus marker cell", got, "abc ")
	}
	if spans := m.Highlights(0); len(spans) != 1 || spans[0] != (filebuffer.Span{Start: 3, End: 4}) {
		t.Fatalf("row 0 spans = %+v, want [{3 4}]", spans)
	}
}

// The terminator-only marker is no special case for clipping: inside
// the window its one cell paints as an inverse space; wholly left or
// right of the window it drops like any cluster.
func TestTerminatorMarkerClipsLikeAnyCell(t *testing.T) {
	for _, tc := range []struct {
		name       string
		content    string
		start, end int
	}{
		{"CRLF", "hit\r\n", 4, 4},
		{"LF", "hit\n", 3, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := bufferOf(t, tc.content, stopAt(1, tc.start, tc.end))
			m := NewRowModel(RowModelKey{TextWidth: 10}, src)

			cells, spans := m.Clip(0, 3, 10)
			if got := cellsText(cells); got != " " {
				t.Fatalf("clip [3,13) = %q, want the marker cell at column 3", got)
			}
			if len(spans) != 1 || spans[0] != (filebuffer.Span{Start: 0, End: 1}) {
				t.Fatalf("clip spans = %+v, want [{0 1}]", spans)
			}

			cells, spans = m.Clip(0, 4, 10)
			if got := cellsText(cells); got != "" {
				t.Fatalf("clip [4,14) = %q, want nothing — the marker dropped", got)
			}
			if len(spans) != 0 {
				t.Fatalf("clip spans = %+v, want none — a hidden marker marks no blank", spans)
			}
		})
	}
}

// An entirely hidden terminator-only marker feeds the indicator rules
// like any other match/marker: concealed left of the window it reports
// the gutter `*`, past the right edge it reports the right `*`, and
// painted it reports neither.
func TestTerminatorMarkerDrivesHiddenIndicators(t *testing.T) {
	src := bufferOf(t, "hit\r\n", stopAt(1, 4, 4))
	m := NewRowModel(RowModelKey{TextWidth: 10}, src)

	h := m.Hidden(0, 4, 10) // marker at column 3 is entirely hidden left
	if !h.LeftMatch {
		t.Fatalf("Hidden(0, 4, 10) = %+v, want LeftMatch — the marker is hidden left", h)
	}
	h = m.Hidden(0, 0, 2) // window [0,2) ends before the marker at column 3
	if !h.RightMatch {
		t.Fatalf("Hidden(0, 0, 2) = %+v, want RightMatch — the marker is hidden right", h)
	}
	if h = m.Hidden(0, 0, 10); h.LeftMatch || h.RightMatch {
		t.Fatalf("Hidden(0, 0, 10) = %+v, want neither — the marker paints", h)
	}
}

// An interior marker follows the same indicator rule: the cell it
// marks is the marker's extent, so concealment of that cell reports a
// hidden match.
func TestInteriorMarkerDrivesHiddenIndicators(t *testing.T) {
	src := bufferOf(t, strings.Repeat("x", 30), stopAt(1, 0, 0))
	m := NewRowModel(RowModelKey{TextWidth: 10}, src)

	h := m.Hidden(0, 5, 10) // the marked cell 0 is entirely hidden left
	if !h.LeftMatch {
		t.Fatalf("Hidden(0, 5, 10) = %+v, want LeftMatch", h)
	}
	if h = m.Hidden(0, 0, 10); h.LeftMatch || h.RightMatch {
		t.Fatalf("Hidden(0, 0, 10) = %+v, want neither — the marked cell paints", h)
	}
}

// Marker cells are navigable reveal targets: a hidden end-of-line
// marker reveals horizontally by the minimum that paints it — marker
// column plus one minus the text width — and a marker wrapped onto its
// own row reveals that rendered row vertically.
func TestMarkerIsARevealTarget(t *testing.T) {
	line := strings.Repeat("x", 50)
	src := bufferOf(t, line+"\n", stopAt(1, 50, 50))
	m := NewRowModel(RowModelKey{TextWidth: 10}, src)
	v := &Viewport{}
	v.SetLayout(m, 5)
	v.Reveal(stopAt(1, 50, 50))
	if got := v.Offset(); got != 41 {
		t.Fatalf("offset = %d, want 41 — marker column 50 plus width 1 minus text width 10", got)
	}
	cells, spans := m.Clip(0, 41, 10)
	if got := cellsText(cells); got != "xxxxxxxxx " {
		t.Fatalf("clip at the reveal offset = %q, want the marker painted at the edge", got)
	}
	if len(spans) != 1 || spans[0] != (filebuffer.Span{Start: 9, End: 10}) {
		t.Fatalf("clip spans = %+v, want [{9 10}]", spans)
	}

	// Wrapped at width 10 the marker lands on its own row 5; a height-2
	// window reveals it, clamped to the EOF boundary.
	src = bufferOf(t, line+"\n", stopAt(1, 50, 50))
	m = NewRowModel(RowModelKey{TextWidth: 10, Wrap: true}, src)
	if got := m.TargetRow(stopAt(1, 50, 50)); got != 5 {
		t.Fatalf("wrapped TargetRow = %d, want 5", got)
	}
	v = &Viewport{}
	v.SetLayout(m, 2)
	v.Reveal(stopAt(1, 50, 50))
	if got := v.Top(); got != 4 {
		t.Fatalf("top = %d, want 4 so marker row 5 is visible", got)
	}
}

// A marker whose position maps inside a cluster reveals at the cluster
// the marker marks — the cluster's start column and full width, never a
// split position.
func TestMarkerInsideClusterRevealsClusterStart(t *testing.T) {
	// 世 holds bytes [4,7) and columns 4-5; the zero-width position at
	// byte 5 maps to the cluster start.
	line := strings.Repeat("x", 4) + "世" + strings.Repeat("x", 45)
	src := bufferOf(t, line, stopAt(1, 5, 5))
	m := NewRowModel(RowModelKey{TextWidth: 10}, src)
	col, width := m.TargetColumn(stopAt(1, 5, 5))
	if col != 4 || width != 2 {
		t.Fatalf("TargetColumn = (%d, %d), want (4, 2) — 世's cluster", col, width)
	}
}
