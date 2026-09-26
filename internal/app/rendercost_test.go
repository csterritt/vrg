package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"vrg/internal/filebuffer"
	"vrg/internal/present"
	"vrg/internal/searchindex"
)

// countingIndex wraps the prepared index to count every whole-stop
// materialization: Stops copies the full navigation list, so a call
// during a keystroke or a frame is the whole-index scan the Issue #40
// bound exists to reject. Navigation reads — Current, Next, Prev —
// pass through uncounted.
type countingIndex struct {
	*searchindex.Index
	stopsCalls *int
}

// Stops counts one whole-index materialization and delegates.
func (c countingIndex) Stops() []searchindex.Stop {
	*c.stopsCalls++
	return c.Index.Stops()
}

// manyFileModel returns a settled browse model over files×stopsPerFile
// stops spread across files distinct files, with the first file's
// content loaded and installed. Entry names are padded to width cells
// so list truncation is observable at narrow widths.
func manyFileModel(t *testing.T, files, stopsPerFile, width, w, h int) Model {
	t.Helper()
	dir := t.TempDir()
	var recs []string
	var first string
	for i := 0; i < files; i++ {
		name := fmt.Sprintf("f%04d-%s.txt", i, strings.Repeat("n", width))
		if i == 0 {
			first = name
		}
		recs = append(recs, fmt.Sprintf(`{"type":"begin","data":{"path":{"text":"%s"}}}`, name))
		for j := 0; j < stopsPerFile; j++ {
			recs = append(recs, fmt.Sprintf(
				`{"type":"match","data":{"path":{"text":"%s"},"lines":{"text":"hit\n"},"line_number":%d,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
				name, j+1))
		}
		recs = append(recs, fmt.Sprintf(`{"type":"end","data":{"path":{"text":"%s"},"binary_offset":null}}`, name))
	}
	recs = append(recs, `{"type":"summary","data":{}}`)
	var content strings.Builder
	for j := 0; j < stopsPerFile; j++ {
		content.WriteString("hit\n")
	}
	writeWorkFile(t, dir, first, content.String())
	m, cmd := browseModel(t, dir, w, h, recs...)
	return settle(t, m, cmd)
}

// A file-crossing n plus the frame it produces touches the file-list
// provider only inside the visible window and never materializes the
// whole stop list. One counter spans the Update calls and the
// resulting View together — the bound is on the combined transition,
// so a whole-list scan moved into navigation handling fails just as a
// per-frame scan does.
func TestNavigationRenderCostBoundedByVisibleWindow(t *testing.T) {
	m := manyFileModel(t, 300, 3, 6, 80, 24)

	var listCalls, stopsCalls int
	m.listEntry = func(b []byte) string { listCalls++; return present.Path(b) }
	m.index = countingIndex{Index: m.index.(*searchindex.Index), stopsCalls: &stopsCalls}

	// f0000:1 → f0000:2 → f0000:3 → f0001:1 — two same-file steps,
	// then the crossing that re-derives the list geometry.
	for i := 0; i < 3; i++ {
		var cmd tea.Cmd
		m, cmd = update(t, m, keyPress("n"))
		_ = cmd // the destination's load stays uninvoked
	}
	if s, _ := m.currentStop(); string(s.Path) != "f0001-nnnnnn.txt" {
		t.Fatalf("three n steps selected %+v, want f0001-nnnnnn.txt:1", s)
	}
	_ = m.View()
	if stopsCalls != 0 {
		t.Fatalf("navigation plus render materialized the whole stop list %d times", stopsCalls)
	}
	if listCalls > m.height {
		t.Fatalf("navigation plus render queried the list provider %d times, "+
			"want at most the %d visible rows", listCalls, m.height)
	}
	if listCalls == 0 {
		t.Fatal("frame never queried the list provider")
	}
}

// A resize — and a load that grows the gutter — re-truncate the
// visible entries against the new list width at grapheme boundaries
// without rescanning the file list. The counter bound covers each
// combined Update-plus-View transition.
func TestResizeAndGutterGrowthRetruncateWithinVisibleCost(t *testing.T) {
	// 26-cell names: untruncated inside the 28-cell list at width 80,
	// cut to a leading-… suffix inside the 16-cell list at width 40.
	m := manyFileModel(t, 200, 1, 16, 80, 24)

	var listCalls, stopsCalls int
	ix := m.index.(*searchindex.Index)
	m.listEntry = func(b []byte) string { listCalls++; return present.Path(b) }
	m.index = countingIndex{Index: ix, stopsCalls: &stopsCalls}

	before := listCalls
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 24})
	stripped := ansi.Strip(m.View().Content)
	if !strings.Contains(stripped, "…") {
		t.Fatalf("resized frame did not re-truncate the list entries:\n%s", stripped)
	}
	if got := listCalls - before; got > m.height {
		t.Fatalf("resize plus render queried the list provider %d times, "+
			"want at most the %d visible rows", got, m.height)
	}

	// A load widening the gutter re-runs the same geometry path:
	// visible paths re-truncate against the narrowed list with no
	// whole-list scan.
	var content strings.Builder
	for i := 0; i < 15; i++ {
		fmt.Fprintf(&content, "line %02d\n", i+1)
	}
	req := mintLoad(&m, string(m.files[0]))
	before = listCalls
	m, _ = update(t, m, loadDoneMsg{
		path: m.files[0],
		req:  req,
		buf:  filebuffer.Prepare([]byte(content.String()), stopsForFile(ix, m.files[0])),
	})
	_ = m.View()
	if got := listCalls - before; got > m.height {
		t.Fatalf("gutter growth plus render queried the list provider %d times, "+
			"want at most the %d visible rows", got, m.height)
	}
	if stopsCalls != 0 {
		t.Fatalf("relayouts materialized the whole stop list %d times", stopsCalls)
	}
}
