package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// deliverStageOne runs a load command and feeds its fileLoadedMsg
// through Update, returning the layout-preparation command stage one
// issued — the seam separating "load completed" from "layout
// installed" in Issue #28's two-stage contract.
func deliverStageOne(t *testing.T, m *model, cmd tea.Cmd) tea.Cmd {
	t.Helper()
	msg, ok := fileLoadOf(cmd)
	if !ok {
		t.Fatal("load command produced no fileLoadedMsg")
	}
	_, lc := m.Update(msg)
	return lc
}

// consultedRows records every rowSource call — the probe proving a
// load completion performs no row-based decision (Issue #28): stage
// one files the result and records the intent without ever consulting
// the row model.
type consultedRows struct {
	calls []string
}

func (c *consultedRows) Len() int         { c.calls = append(c.calls, "Len"); return 60 }
func (c *consultedRows) GutterWidth() int { c.calls = append(c.calls, "GutterWidth"); return 3 }

func (c *consultedRows) Key() viewport.Key {
	c.calls = append(c.calls, "Key")
	return viewport.Key{Wrap: true}
}

func (c *consultedRows) At(i int) viewport.Row {
	c.calls = append(c.calls, "At")
	return viewport.Row{Line: filebuffer.Line{Number: int64(i + 1)}}
}

func (c *consultedRows) AnchorAt(row int) viewport.Anchor {
	c.calls = append(c.calls, "AnchorAt")
	return viewport.Anchor{Line: int64(row + 1)}
}

func (c *consultedRows) RowOf(a viewport.Anchor) int {
	c.calls = append(c.calls, "RowOf")
	return int(a.Line) - 1
}

func (c *consultedRows) TargetRow(st searchindex.Stop) int {
	c.calls = append(c.calls, "TargetRow")
	return int(st.Number) - 1
}

func (c *consultedRows) StopTarget(st searchindex.Stop) viewport.Target {
	c.calls = append(c.calls, "StopTarget")
	return viewport.Target{Line: st.Number}
}

// Stage one is limited to filing the load's result: the buffer is
// cached, the content revision bumps, and a prepared layout is
// requested for the current (path, revision, text width, wrap mode) —
// but no row-based decision runs. The reveal intent for the latest
// selection is recorded in the model, no viewport state is created,
// and "Loading…" stays up until the matching layout installs and
// commits the intent (Issue #28).
func TestLoadCompletionStageOneDefersToInstall(t *testing.T) {
	gate := newHeldLayouts()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, gateOpts(gate))
	idx := navIndex(t, []navFile{longFile(200)})
	keyA := string(idx.Files[0].Path)
	cmd := startBrowse(t, m, idx)

	lc := deliverStageOne(t, m, cmd)
	if lc == nil {
		t.Fatal("load completion issued no layout request")
	}
	if m.bufs[keyA] == nil {
		t.Fatal("the completed load was not cached")
	}
	if got := m.revs[keyA]; got != 1 {
		t.Fatalf("content revision = %d after the first load, want 1", got)
	}
	want := m.layoutKey(keyA, m.bufs[keyA])
	if got := m.layoutReqs[keyA]; got != want {
		t.Fatalf("requested key = %+v, want the current %+v", got, want)
	}
	if !m.pendingReveals[keyA] {
		t.Fatal("load completion recorded no reveal intent")
	}
	if vp, ok := m.vps[keyA]; ok {
		t.Fatalf("viewport state %+v created at load completion, want none", vp)
	}
	worker := runCmd(lc)
	<-gate.entered
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… until the matching layout installs", v)
	}

	close(gate.release)
	m.Update(<-worker)
	if m.pendingReveals[keyA] {
		t.Fatal("the reveal intent survived its own commit")
	}
	if got := m.vps[keyA].Top(); got != 192 {
		t.Fatalf("committed reveal top = %d, want 192 (row 199 at content row 7)", got)
	}
}

// A load completion performs no row-based decision itself — no
// visibility test, placement, clamping, or horizontal reveal — even
// when a row model stands in as installed: the completion records the
// reveal intent without consulting the model, the viewport does not
// move, and the commit runs only when a matching layout installs.
// The fabricated completion exercises the non-reload branch the way
// TestStaleRevisionCompletionDiscarded fabricates the reload's.
func TestLoadCompletionMakesNoRowDecision(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{longFile(50)})
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))

	fake := &consultedRows{}
	m.rows[keyA] = fake
	for i := 0; i < 2; i++ {
		m.Update(keyPgUp) // top 0: the target row is hidden again
	}
	fake.calls = nil
	vp0 := m.vps[keyA]

	buf, err := filebuffer.Load(idx.Files[0].Path, idx.Files[0].Stops)
	if err != nil {
		t.Fatal(err)
	}
	m.Update(fileLoadedMsg{path: idx.Files[0].Path, req: mintRequest(m, idx.Files[0].Path), buf: buf})

	if len(fake.calls) != 0 {
		t.Fatalf("load completion consulted the row model %v, want no row-based decision", fake.calls)
	}
	if got := m.vps[keyA]; got != vp0 {
		t.Fatalf("viewport moved at load completion to %+v, want %+v", got, vp0)
	}
	if !m.pendingReveals[keyA] {
		t.Fatal("load completion recorded no reveal intent")
	}

	// The intent commits when the matching layout installs.
	key := m.layoutKey(keyA, m.bufs[keyA])
	m.Update(layoutReadyMsg{key: key, rows: viewport.Prepare(m.bufs[keyA], key)})
	if m.pendingReveals[keyA] {
		t.Fatal("the reveal intent survived its own commit")
	}
	rows := m.rows[keyA]
	want := rows.TargetRow(idx.Files[0].Stops[0]) - contentRows24/3
	if max := viewport.MaxTop(rows.Len(), contentRows24); want > max {
		want = max
	}
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("committed reveal top = %d, want %d", got, want)
	}
}

// A startup target hidden from the top of the file lands at
// floor(h / 3) only after both separately gated stages complete:
// "Loading…" stays up through the held load and again through the
// held layout, the install commits the reveal, and the first n
// advances to the second stop.
func TestStartupHiddenTargetCommitsAfterBothStages(t *testing.T) {
	loads := newHeldLoads()
	layouts := newHeldLayouts()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loadGate:   loads.fn(),
		layoutGate: layouts.fn(),
	})
	idx := navIndex(t, []navFile{longFile(200, 230)})
	keyA := string(idx.Files[0].Path)
	cmd := startBrowse(t, m, idx)

	worker := runCmd(cmd)
	<-loads.entered // stage one held: the load worker blocks
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… while the load is held", v)
	}
	close(loads.release)
	_, lc := m.Update(<-worker)

	layoutWorker := runCmd(lc)
	<-layouts.entered // stage two held separately
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… while the layout is held", v)
	}
	close(layouts.release)
	m.Update(<-layoutWorker)

	if got := m.vps[keyA].Top(); got != 192 {
		t.Fatalf("startup reveal top = %d, want 192 (row 199 at content row 7)", got)
	}

	// The first n advances to the second stop — line 230's row 229
	// lands at 229 − 7 = 222.
	m.Update(keyN)
	if cur, _ := m.idx.Cursor(); cur.Stop != 1 {
		t.Fatalf("first n selected stop %d, want 1", cur.Stop)
	}
	if got := m.vps[keyA].Top(); got != 222 {
		t.Fatalf("second stop top = %d, want 222", got)
	}
}

// A startup target visible from the top of the file does not scroll:
// the install's commit leaves top 0 and records no vertical state.
func TestStartupVisibleTargetKeepsTopThroughStages(t *testing.T) {
	gate := newHeldLayouts()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, gateOpts(gate))
	idx := navIndex(t, []navFile{longFile(5)})
	keyA := string(idx.Files[0].Path)

	lc := deliverStageOne(t, m, startBrowse(t, m, idx))
	worker := runCmd(lc)
	<-gate.entered
	close(gate.release)
	m.Update(<-worker)

	if vp, ok := m.vps[keyA]; ok || vp.Top() != 0 {
		t.Fatalf("visible startup commit recorded state (ok=%v top=%d), want none", ok, vp.Top())
	}
}

// A resize between the stages discards the old-width layout while the
// intent survives: the replacement request mints under the new
// parameters, the superseded completion is discarded without
// consuming the intent, and the commit lands at the new width.
func TestResizeBetweenLoadAndLayoutCommitsAtNewWidth(t *testing.T) {
	gate := newHeldLayouts()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, gateOpts(gate))
	idx := navIndex(t, []navFile{longFile(200)})
	keyA := string(idx.Files[0].Path)

	lc := deliverStageOne(t, m, startBrowse(t, m, idx))
	worker := runCmd(lc)
	<-gate.entered
	tw0 := m.layoutReqs[keyA].TextWidth

	_, rc := m.Update(tea.WindowSizeMsg{Width: frameWidthFor(m, keyA, tw0+30), Height: 24})
	if rc == nil {
		t.Fatal("the resize minted no replacement layout request")
	}
	worker2 := runCmd(rc)
	<-gate.entered
	if !m.pendingReveals[keyA] {
		t.Fatal("the resize consumed the pending reveal intent")
	}

	close(gate.release)
	m.Update(<-worker) // the old-width layout is obsolete
	if m.rows[keyA] != nil {
		t.Fatal("the old-width layout installed under the new parameters")
	}
	if !m.pendingReveals[keyA] {
		t.Fatal("the obsolete layout consumed or mutated the reveal intent")
	}
	if _, ok := m.vps[keyA]; ok {
		t.Fatal("the obsolete layout moved the viewport")
	}
	m.Update(<-worker2)
	rows, ok := m.rows[keyA].(*viewport.Rows)
	if !ok {
		t.Fatal("the new-width layout did not install")
	}
	if got := rows.Key().TextWidth; got != tw0+30 {
		t.Fatalf("installed text width = %d, want the new %d", got, tw0+30)
	}
	if got := m.vps[keyA].Top(); got != 192 {
		t.Fatalf("commit top = %d at the new width, want 192", got)
	}
}

// n/p while the layout is pending move the cursor immediately; the
// model-carried intent keeps tracking the latest selection, so the
// commit reveals the newest stop's target — never a target captured
// when the load was requested.
func TestNavigateWhileLayoutPendingCommitsNewestTarget(t *testing.T) {
	gate := newHeldLayouts()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, gateOpts(gate))
	idx := navIndex(t, gatedFiles)
	keyA := string(idx.Files[0].Path)

	lc := deliverStageOne(t, m, startBrowse(t, m, idx))
	worker := runCmd(lc)
	<-gate.entered

	m.Update(keyN)
	m.Update(keyN) // cursor on stop 2 — the newest selection
	if cur, _ := m.idx.Cursor(); cur.Stop != 2 {
		t.Fatalf("cursor stop = %d while the layout was held, want 2", cur.Stop)
	}
	if !m.pendingReveals[keyA] {
		t.Fatal("navigation while the layout was held left no reveal intent")
	}
	close(gate.release)
	m.Update(<-worker)

	rows := m.rows[keyA].(*viewport.Rows)
	want := rows.TargetRow(idx.Files[0].Stops[2]) - contentRows24/3
	if max := viewport.MaxTop(rows.Len(), contentRows24); want > max {
		want = max
	}
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("commit top = %d, want the newest selection's %d", got, want)
	}
}

// A file-list hide between the stages re-keys the pending layout —
// Issue #24's list change participates in the final text width — and
// the commit lands against it.
func TestListHideBetweenStagesCommitsAtFinalWidth(t *testing.T) {
	gate := newHeldLayouts()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, gateOpts(gate))
	idx := navIndex(t, []navFile{longFile(200)})
	keyA := string(idx.Files[0].Path)

	lc := deliverStageOne(t, m, startBrowse(t, m, idx))
	worker := runCmd(lc)
	<-gate.entered
	tw0 := m.layoutReqs[keyA].TextWidth

	_, lc2 := m.Update(keyLeft) // hide the file list
	if lc2 == nil {
		t.Fatal("hiding the file list minted no replacement request")
	}
	worker2 := runCmd(lc2)
	<-gate.entered

	close(gate.release)
	m.Update(<-worker) // the listed-width layout is obsolete
	if !m.pendingReveals[keyA] {
		t.Fatal("the superseded layout consumed the reveal intent")
	}
	m.Update(<-worker2)
	rows, ok := m.rows[keyA].(*viewport.Rows)
	if !ok {
		t.Fatal("the list-hidden layout did not install")
	}
	if got := rows.Key().TextWidth; got <= tw0 {
		t.Fatalf("installed text width = %d, want the wider list-hidden width > %d", got, tw0)
	}
	if got := m.vps[keyA].Top(); got != 192 {
		t.Fatalf("commit top = %d, want 192", got)
	}
}

// Gutter growth between the stages commits against the final text
// width: a reload that turns a small file into a five-digit
// line-number file computes the grown gutter at completion, so the
// request and the installed model carry the shrunken text width.
func TestGutterGrowthBetweenStagesCommitsAtFinalWidth(t *testing.T) {
	hold := newLayoutHold()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{layoutGate: hold.fn()})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))
	tw0 := m.rows[keyA].(*viewport.Rows).Key().TextWidth
	hold.armed.Store(true)

	rewriteFile(t, idx.Files[0].Path, numberedContent("a", 12000))
	msg, ok := fileLoadOf(reloadCmd(t, m))
	if !ok {
		t.Fatal("the reload produced no completion")
	}
	_, lc := m.Update(msg)
	if lc == nil {
		t.Fatal("the reload completion minted no layout request")
	}
	want := m.layoutReqs[keyA].TextWidth
	if want >= tw0 {
		t.Fatalf("request text width = %d, want < %d after the gutter grew", want, tw0)
	}
	worker := runCmd(lc)
	<-hold.entered
	close(hold.release)
	m.Update(<-worker)
	if got := m.rows[keyA].(*viewport.Rows).Key().TextWidth; got != want {
		t.Fatalf("installed text width = %d, want the final %d", got, want)
	}
}

// A layout minted under the superseded revision is discarded on
// arrival without consuming or mutating the pending intent; the new
// revision's matching layout installs and commits it — here the
// undisturbed reload's anchor preservation.
func TestStaleRevisionLayoutDiscardsWithoutConsuming(t *testing.T) {
	hold := newLayoutHold()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{layoutGate: hold.fn()})
	idx := navIndex(t, reloadFiles)
	keyA := string(idx.Files[0].Path)
	finishLoad(t, m, startBrowse(t, m, idx))
	installed := m.rows[keyA]
	for i := 0; i < 10; i++ {
		m.Update(keyDown)
	}
	anchor0 := m.vps[keyA].Anchor()
	hold.armed.Store(true)

	// Mint a revision-1 request at a fresh width — held.
	tw0 := m.rows[keyA].(*viewport.Rows).Key().TextWidth
	_, rc := m.Update(tea.WindowSizeMsg{Width: frameWidthFor(m, keyA, tw0+20), Height: 24})
	workerOld := runCmd(rc)
	<-hold.entered

	// The reload bumps the revision mid-flight; its own request is
	// keyed by revision 2.
	rewriteFile(t, idx.Files[0].Path, numberedContent("a", 90))
	msg, ok := fileLoadOf(reloadCmd(t, m))
	if !ok {
		t.Fatal("the reload produced no completion")
	}
	_, lc := m.Update(msg)
	workerNew := runCmd(lc)
	<-hold.entered

	// The revision-1 layout arrives late: obsolete against
	// revision 2 — discarded without touching the installed layout
	// or consuming the pending intent.
	close(hold.release)
	m.Update(<-workerOld)
	if m.rows[keyA] != installed {
		t.Fatal("a stale-revision layout replaced the installed model")
	}
	if !m.pendingAnchor[keyA] {
		t.Fatal("a stale-revision layout consumed the anchor intent")
	}
	m.Update(<-workerNew)
	rows, ok := m.rows[keyA].(*viewport.Rows)
	if !ok || rows.Key().Revision != 2 {
		t.Fatalf("installed layout = %#v, want revision 2", m.rows[keyA])
	}
	if got, want := m.vps[keyA].Top(), rows.RowOf(anchor0); got != want {
		t.Fatalf("top after install = %d, want the anchor's row %d", got, want)
	}
}

// A saved-viewport revisit whose target stays visible from the saved
// position does not move: the stale layout pends the reveal, and the
// commit against the fresh install leaves the saved top in place.
func TestSavedViewportRevisitVisibleStays(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{popupTimer: popupStubTicks.popupTimer})
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{
			{line: 5, start: 0, end: 1},
			{line: 20, start: 0, end: 1},
		}},
		{name: "b.txt", content: numberedContent("b", 30), stops: []navStop{
			{line: 3, start: 0, end: 1},
		}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	keyA := string(idx.Files[0].Path)

	for i := 0; i < 10; i++ {
		m.Update(keyDown)
	}
	m.Update(keyN)                       // line 20's row 19 stays visible from top 10
	finishLoad(t, m, navCmd(t, m, keyN)) // to b.txt
	_, lc := m.Update(tea.WindowSizeMsg{ // make A's layout stale
		Width:  frameWidthFor(m, keyA, m.rows[keyA].(*viewport.Rows).Key().TextWidth+20),
		Height: 24,
	})
	deliverLayout(t, m, lc)

	_, nav := m.Update(keyP) // back to A: reveal pends on the stale layout
	if !m.pendingReveals[keyA] {
		t.Fatal("revisiting a stale-layout file did not pend the reveal")
	}
	for _, lm := range leafMsgs(nav) {
		if lr, ok := lm.(layoutReadyMsg); ok && lr.key.Path == keyA {
			m.Update(lr)
		}
	}
	if got := m.vps[keyA].Top(); got != 10 {
		t.Fatalf("revisit top = %d, want the saved 10 — the target was visible", got)
	}
}

// A saved-viewport revisit whose target is hidden from the saved
// position moves on commit: the reveal places the target a third
// down — BOF clamped.
func TestSavedViewportRevisitHiddenMoves(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{popupTimer: popupStubTicks.popupTimer})
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{
			{line: 5, start: 0, end: 1},
		}},
		{name: "b.txt", content: numberedContent("b", 30), stops: []navStop{
			{line: 3, start: 0, end: 1},
		}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	keyA := string(idx.Files[0].Path)

	for i := 0; i < 10; i++ {
		m.Update(keyDown) // saved top 10: line 5's row 4 is hidden above
	}
	finishLoad(t, m, navCmd(t, m, keyN)) // to b.txt
	_, lc := m.Update(tea.WindowSizeMsg{
		Width:  frameWidthFor(m, keyA, m.rows[keyA].(*viewport.Rows).Key().TextWidth+20),
		Height: 24,
	})
	deliverLayout(t, m, lc)

	_, nav := m.Update(keyP)
	for _, lm := range leafMsgs(nav) {
		if lr, ok := lm.(layoutReadyMsg); ok && lr.key.Path == keyA {
			m.Update(lr)
		}
	}
	// Row 4 hidden from the restored top 10: the reveal lands it at
	// content row 7 — clamped to top 0 by BOF content.
	if got := m.vps[keyA].Top(); got != 0 {
		t.Fatalf("revisit top = %d, want the BOF-clamped 0", got)
	}
}

// A terminator-only $ match far down the file commits to the marker's
// row — the marker cell at the line's end — and paints it in
// run-off-edge mode: the offset stays 0 because the marker cell is
// already inside the window.
func TestMarkerTargetCommitPaintsMarkerCell(t *testing.T) {
	hold := newLayoutHold()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{layoutGate: hold.fn()})
	var b strings.Builder
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&b, "pad %02d\n", i)
	}
	content := b.String() + "hit\r\n"
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: content,
		stops:   []navStop{{line: 61, start: 4, end: 4}}, // $ on hit\r\n
	}})
	hold.armed.Store(true)
	cmd := startBrowse(t, m, idx)
	m.Update(tea.WindowSizeMsg{Width: frameWidthForG(m, 4, 41), Height: 24})
	m.Update(keyW) // run-off-edge

	lc := deliverStageOne(t, m, cmd)
	worker := runCmd(lc)
	<-hold.entered
	close(hold.release)
	m.Update(<-worker)

	keyA := string(idx.Files[0].Path)
	rows := m.rows[keyA]
	target := rows.TargetRow(idx.Files[0].Stops[0])
	want := target - contentRows24/3
	if max := viewport.MaxTop(rows.Len(), contentRows24); want > max {
		want = max
	}
	if got := m.vps[keyA].Top(); got != want {
		t.Fatalf("top = %d, want %d — the marker's row revealed", got, want)
	}
	if got := m.vps[keyA].Off(); got != 0 {
		t.Fatalf("off = %d, want 0 — the marker cell was already painted", got)
	}
	row := frameRow(t, m, target-want+1)
	if !strings.Contains(row, "hit") {
		t.Fatalf("target row = %q, want the marker's line", row)
	}
	if !strings.Contains(row, "\x1b[30;47;4m ") {
		t.Fatalf("target row = %q, want the marker's inverse-underline space painted", row)
	}
}

// A mid-cluster target — the submatch's first byte lands inside a
// grapheme cluster — commits to the cluster's start cell: the
// horizontal reveal moves the offset the fewest columns that paint
// the whole cluster at the right edge.
func TestClusterTargetCommitPaintsWholeCluster(t *testing.T) {
	hold := newLayoutHold()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{layoutGate: hold.fn()})
	m.theme = theme.Plain()
	// 文 occupies cells 298–299 of line 1 (three bytes); the submatch
	// starts on its second byte — a mid-cluster target.
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("x", 298) + "文\n",
		stops:   []navStop{{line: 1, start: 299, end: 301}},
	}})
	hold.armed.Store(true)
	cmd := startBrowse(t, m, idx)
	m.Update(tea.WindowSizeMsg{Width: frameWidthForG(m, 3, 41), Height: 24})
	m.Update(keyW) // run-off-edge: 40 text cells

	lc := deliverStageOne(t, m, cmd)
	worker := runCmd(lc)
	<-hold.entered
	close(hold.release)
	m.Update(<-worker)

	keyA := string(idx.Files[0].Path)
	if got := m.vps[keyA].Off(); got != 260 {
		t.Fatalf("off = %d, want 260 — cluster start 298 + width 2 − text width 40", got)
	}
	row := frameRow(t, m, 1)
	if !strings.HasSuffix(strings.TrimSpace(row), "文") {
		t.Fatalf("row = %q, want the whole 文 painted at the right edge", row)
	}
}

// A completion for a file that is no longer current updates only its
// own cache: the visible panel, the current file's viewport, and the
// intents are untouched — and no intent is recorded for the foreign
// file, for either stage.
func TestNonCurrentCompletionLeavesPanelUntouched(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, isoFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	finishLoad(t, m, navCmd(t, m, keyN)) // B current
	before := viewText(m)

	buf, err := filebuffer.Load(idx.Files[2].Path, idx.Files[2].Stops)
	if err != nil {
		t.Fatal(err)
	}
	mintRequest(m, idx.Files[2].Path)
	_, lc := m.Update(fileLoadedMsg{path: idx.Files[2].Path, buf: buf})
	deliverLayout(t, m, lc) // even C's install stays invisible
	keyC := string(idx.Files[2].Path)

	if got := viewText(m); got != before {
		t.Fatal("a non-current completion changed the panel")
	}
	if m.pendingReveals[keyC] || m.pendingAnchor[keyC] {
		t.Fatal("a non-current completion recorded an intent")
	}
}

// The file-change pop-up is unaffected by either stage: the same
// instance survives the load completion and the layout install.
func TestPopupSurvivesBothStages(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, isoFiles)
	finishLoad(t, m, startBrowse(t, m, idx))

	cmd := navCmd(t, m, keyN) // the crossing starts the pop-up
	if m.popupID == 0 {
		t.Fatal("no pop-up on the file crossing")
	}
	id := m.popupID

	lc := deliverStageOne(t, m, cmd)
	if m.popupID != id {
		t.Fatal("the load completion disturbed the pop-up")
	}
	deliverLayout(t, m, lc)
	if m.popupID != id {
		t.Fatal("the layout install disturbed the pop-up")
	}
}
