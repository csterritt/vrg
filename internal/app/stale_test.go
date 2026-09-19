package app

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// staleFile is a fixture file whose stops may carry several recorded
// submatches — the multi-submatch fixture Issue #29's partial-survival
// cases need.
type staleFile struct {
	name    string
	content string
	stops   []staleStop
}

// staleStop is one matched line's 1-based number and recorded
// submatch byte ranges.
type staleStop struct {
	line int64
	subs [][2]int
}

// staleIndex writes the fixture files and builds the index, recording
// every listed submatch with its true line bytes — navIndex's
// multi-submatch generalization.
func staleIndex(t *testing.T, files []staleFile) *searchindex.Index {
	t.Helper()
	dir := t.TempDir()
	var stream strings.Builder
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.content), 0o644); err != nil {
			t.Fatal(err)
		}
		path := "./" + f.name
		fmt.Fprintf(&stream, `{"type":"begin","data":{"path":{"bytes":%q}}}`+"\n", b64(path))
		for _, s := range f.stops {
			line := nthLine(f.content, s.line)
			var subs strings.Builder
			for i, r := range s.subs {
				if i > 0 {
					subs.WriteByte(',')
				}
				fmt.Fprintf(&subs, `{"match":{"bytes":%q},"start":%d,"end":%d}`,
					b64(line[r[0]:r[1]]), r[0], r[1])
			}
			fmt.Fprintf(&stream, `{"type":"match","data":{"path":{"bytes":%q},"lines":{"bytes":%q},"line_number":%d,"absolute_offset":0,"submatches":[%s]}}`+"\n",
				b64(path), b64(line), s.line, subs.String())
		}
		fmt.Fprintf(&stream, `{"type":"end","data":{"path":{"bytes":%q},"binary_offset":null,"stats":{}}}`+"\n", b64(path))
	}
	stream.WriteString(`{"type":"summary","data":{"stats":{}}}` + "\n")
	return searchindex.Build([]byte(stream.String()), dir)
}

// rowContaining returns the view row rendering text — the rendered
// row a content assertion applies to.
func rowContaining(t *testing.T, m *model, text string) string {
	t.Helper()
	for _, row := range strings.Split(viewText(m), "\n") {
		if strings.Contains(row, text) {
			return row
		}
	}
	t.Fatalf("no view row contains %q; view = %q", text, viewText(m))
	return ""
}

// A buffer whose recorded submatch no longer matches the loaded bytes
// carries "file changed since search" in the filename row's status
// slot on every display — composed at ordinary and constrained widths
// with the path truncating to make room, nothing overflowing, and the
// note persisting with no timer.
func TestStaleNoteInFilenameRow(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain() // no SGR — CellWidth measures the real row
	idx := navIndex(t, navFiles)
	keyA := string(idx.Files[0].Path)
	// The file changes between the search and its load: a same-length
	// replacement whose bytes no longer equal the recorded submatch.
	rewriteFile(t, idx.Files[0].Path,
		strings.Replace(navFiles[0].content, "aaa3", "zzz3", 1))
	finishLoad(t, m, startBrowse(t, m, idx))

	row0 := frameRow(t, m, 0)
	if !strings.Contains(row0, staleNote) {
		t.Fatalf("filename row = %q, want the status slot carrying %q", row0, staleNote)
	}
	if !strings.Contains(row0, "a.txt") {
		t.Fatalf("filename row = %q, want it still naming the path", row0)
	}
	if w := safepresentation.CellWidth(row0); w > m.width {
		t.Fatalf("filename row width = %d over the frame's %d — the slot overflowed", w, m.width)
	}
	if !m.bufs[keyA].Stale() {
		t.Fatal("the mismatched load did not validate stale")
	}

	// The note rides every display — scrolling and a constrained
	// resize leave it in place with no timer to expire.
	m.Update(keyDown)
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 44, Height: 24})
	deliverLayout(t, m, cmd)
	row0 = frameRow(t, m, 0)
	if !strings.Contains(row0, staleNote) {
		t.Fatalf("constrained filename row = %q, want %q still in the slot", row0, staleNote)
	}
	if w := safepresentation.CellWidth(row0); w > 44 {
		t.Fatalf("constrained filename row width = %d over 44 — the path did not yield", w)
	}
	if m.listWidth() < 0 || m.textWidth(m.bufs[keyA].GutterWidth()) < 0 {
		t.Fatalf("negative composed dimensions: list %d text %d",
			m.listWidth(), m.textWidth(m.bufs[keyA].GutterWidth()))
	}
}

// A clean reload recomputes the stale state: content validating fully
// again clears the note, while still-mismatched content keeps it.
func TestStaleNoteClearedByCleanReload(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, navFiles)
	rewriteFile(t, idx.Files[0].Path,
		strings.Replace(navFiles[0].content, "aaa3", "zzz3", 1))
	finishLoad(t, m, startBrowse(t, m, idx))
	if !strings.Contains(frameRow(t, m, 0), staleNote) {
		t.Fatal("the stale load showed no note")
	}

	// The file reverts: the reload validates fully and the slot
	// empties — the note disappears only on clean content.
	rewriteFile(t, idx.Files[0].Path, navFiles[0].content)
	finishLoad(t, m, reloadCmd(t, m))
	if strings.Contains(frameRow(t, m, 0), staleNote) {
		t.Fatal("a fully validating reload kept the stale note")
	}

	// Still mismatched again: the note returns.
	rewriteFile(t, idx.Files[0].Path,
		strings.Replace(navFiles[0].content, "aaa3", "zzz3", 1))
	finishLoad(t, m, reloadCmd(t, m))
	if !strings.Contains(frameRow(t, m, 0), staleNote) {
		t.Fatal("a still-mismatched reload did not restore the note")
	}
}

// The stale fixture: a.txt's line 30 carries two recorded "alpha"
// submatches beside an ordinary stop at line 10 — deep enough to
// scroll — and b.txt provides nothing to reach.
var staleSurvivorContent = strings.Replace(
	numberedContent("a", 60), "a line 30\n", "alpha xx alpha\n", 1)

// Navigation during a held reload selects the stop whose first
// recorded submatch the new content drops: the matching prepared
// layout installs per Issue #28's two-stage path and the commit
// reveals the first surviving submatch — never the dropped one's cell.
func TestStaleGatedReloadCommitRevealsSurvivor(t *testing.T) {
	loads := newHeldNthLoad(2)
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loadGate: loads.fn()})
	idx := staleIndex(t, []staleFile{{
		name:    "a.txt",
		content: staleSurvivorContent,
		stops: []staleStop{
			{line: 10, subs: [][2]int{{0, 1}}},
			{line: 30, subs: [][2]int{{0, 5}, {9, 14}}},
		},
	}})
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	// The disk edit drops the first recorded submatch's bytes —
	// "omega" replaced "alpha" — while the second survives.
	rewriteFile(t, idx.Files[0].Path,
		strings.Replace(staleSurvivorContent, "alpha xx alpha\n", "omega xx alpha\n", 1))

	_, cmd := m.Update(keyR)
	worker := runCmd(cmd)
	<-loads.entered // the reload is in flight and held

	m.Update(keyN) // the latest selection: the stale stop at line 30
	if !m.pendingReveals[keyA] {
		t.Fatal("navigation during the load recorded no reveal intent")
	}
	close(loads.release)
	_, lc := m.Update(<-worker)
	deliverLayout(t, m, lc)

	st := idx.Files[0].Stops[1]
	rows := m.rows[keyA].(*viewport.Rows)
	if tg := rows.StopTarget(st); tg.Line != 30 || tg.Cell != 9 {
		t.Fatalf("StopTarget = %+v, want {30, 9} — the first survivor's start cell", tg)
	}
	want := rows.TargetRow(st) - contentRows24/3
	if max := viewport.MaxTop(rows.Len(), contentRows24); want > max {
		want = max
	}
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("commit top = %d, want the survivor's reveal top %d", got, want)
	}

	row := rowContaining(t, m, "omega xx")
	if !strings.Contains(row, "\x1b[30;47;4malpha") {
		t.Fatalf("stale row = %q, want the surviving submatch highlighted", row)
	}
	if strings.Contains(row, "47;4momega") || strings.Contains(row, "47momega") {
		t.Fatalf("stale row = %q, the dropped submatch must not be highlighted", row)
	}
	if !strings.Contains(frameRow(t, m, 0), staleNote) {
		t.Fatal("the stale reload showed no note")
	}
}

// With every recorded submatch dropped but the line still present, the
// commit reveals the first recorded start clamped to a valid display
// cell — and paints no invented highlight.
func TestStaleGatedReloadCommitRevealsClampedFallback(t *testing.T) {
	loads := newHeldNthLoad(2)
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loadGate: loads.fn()})
	content := strings.Replace(
		numberedContent("a", 60), "a line 30\n", "the needle here\n", 1)
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: content,
		stops: []navStop{
			{line: 10, start: 0, end: 1},
			{line: 30, start: 4, end: 10}, // "needle"
		},
	}})
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	// Same-length replacement: "sizzle" occupies the recorded range
	// but its bytes differ — the submatch drops, the line stays.
	rewriteFile(t, idx.Files[0].Path,
		strings.Replace(content, "the needle here\n", "the sizzle here\n", 1))

	_, cmd := m.Update(keyR)
	worker := runCmd(cmd)
	<-loads.entered

	m.Update(keyN) // the latest selection: the stale stop at line 30
	close(loads.release)
	_, lc := m.Update(<-worker)
	deliverLayout(t, m, lc)

	st := idx.Files[0].Stops[1]
	rows := m.rows[keyA].(*viewport.Rows)
	if tg := rows.StopTarget(st); tg.Line != 30 || tg.Cell != 4 {
		t.Fatalf("StopTarget = %+v, want {30, 4} — the clamped recorded start", tg)
	}
	want := rows.TargetRow(st) - contentRows24/3
	if max := viewport.MaxTop(rows.Len(), contentRows24); want > max {
		want = max
	}
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("commit top = %d, want the fallback's reveal top %d", got, want)
	}

	row := rowContaining(t, m, "the sizzle here")
	if strings.Contains(row, "\x1b[30;47") {
		t.Fatalf("stale row = %q, the fallback invents no highlight", row)
	}
	if !strings.Contains(frameRow(t, m, 0), staleNote) {
		t.Fatal("the stale reload showed no note")
	}
}

// A stop whose recorded line the reloaded file no longer has lands at
// the last source line's start — no invented highlight or marker.
func TestStaleMissingLineLandsOnLastLine(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: numberedContent("a", 20),
		stops: []navStop{
			{line: 5, start: 0, end: 1},
			{line: 20, start: 0, end: 1},
		},
	}})
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	// The trailing matched line is deleted on disk.
	rewriteFile(t, idx.Files[0].Path, numberedContent("a", 19))
	finishLoad(t, m, reloadCmd(t, m))
	if !m.bufs[keyA].Stale() {
		t.Fatal("the deleted line did not validate stale")
	}

	m.Update(keyN) // select the stale stop — its line is gone
	st := idx.Files[0].Stops[1]
	rows := m.rows[keyA].(*viewport.Rows)
	if tg := rows.StopTarget(st); tg.Line != 19 || tg.Cell != 0 {
		t.Fatalf("StopTarget = %+v, want {19, 0} — the last source line's start", tg)
	}
	if tr, top := rows.TargetRow(st), m.vps[keyA].Top(); tr < top || tr >= top+m.contentRows() {
		t.Fatalf("target row %d outside the viewport [%d, %d) — the landing is not revealed",
			tr, top, top+m.contentRows())
	}
	row := rowContaining(t, m, "a line 19")
	if strings.Contains(row, "\x1b[30;47") {
		t.Fatalf("last-line row = %q, the missing-line fallback invents no highlight", row)
	}
	if !strings.Contains(frameRow(t, m, 0), staleNote) {
		t.Fatal("the stale reload showed no note")
	}
}
