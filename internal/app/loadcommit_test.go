package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// gatedTwoStage enters the browse state with the startup file's load
// held behind the load gate and the layout gate installed, so the two
// stages of a completion — the load result, then the keyed prepared-
// layout install — are released separately.
func gatedTwoStage(t *testing.T, dir string, idx *searchindex.Index, w, h int) (Model, chan struct{}, chan struct{}, <-chan tea.Msg) {
	t.Helper()
	m, loadGate, job := gatedBrowse(t, dir, idx, w, h)
	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate
	return m, loadGate, layoutGate, job
}

// navStageJob runs the staging member — a load or layout request — of
// a file-change navigation command, the batch's first element, and
// reports the worker's message channel; a gate-held request stays
// silent until released.
func navStageJob(t *testing.T, cmd tea.Cmd) <-chan tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("navigation returned no command")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("navigation produced %T, want the stage+timer batch", msg)
	}
	return runWorker(t, batch[0])
}

// A current-file load completion performs no row-based decision: it
// retires the request, caches the validated source, and establishes the
// new content revision — then requests the prepared layout keyed to the
// current parameters. The viewport is never touched, the placeholder
// stays, and the pending reveal intent survives. Only the matching
// install commits it — a hidden startup target lands at
// floor(content height / 3).
func TestLoadCompletionDefersRevealToLayoutInstall(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-50\n", 50, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-80\n", 80, 0, 4, "line"))
	idx.Finish()

	m, loadGate, layoutGate, job := gatedTwoStage(t, dir, idx, 80, 24)
	m.theme = theme.Plain()

	// Stage one: the completion establishes the revision and caches
	// the source, but moves nothing — no viewport exists yet, the
	// placeholder still renders, and the reveal intent stays pending.
	close(loadGate)
	m, cmd := update(t, m, collectMsg(t, job, "a.txt's load"))
	if m.buffers["a.txt"] != nil {
		t.Fatal("the load completion installed a layout")
	}
	if m.sources["a.txt"] == nil || m.revs["a.txt"] != 1 {
		t.Fatalf("completion left source %v / revision %d, want the cached source at revision 1",
			m.sources["a.txt"], m.revs["a.txt"])
	}
	if _, ok := m.loading["a.txt"]; ok {
		t.Fatal("the completed request was not retired")
	}
	if m.pendingIntent != intentReveal {
		t.Fatalf("the startup reveal intent = %v, want pending", m.pendingIntent)
	}
	if vp := m.vps["a.txt"]; vp != nil && (vp.Top() != 0 || vp.Offset() != 0) {
		t.Fatalf("the load completion moved the viewport to top %d, offset %d",
			vp.Top(), vp.Offset())
	}
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("the load completion replaced the placeholder:\n%s", got)
	}

	// The completion requested exactly one prepared layout, keyed to
	// the established parameters — the loaded file's final gutter
	// participates: panel 73 − gutter 5 → text width 68.
	if cmd == nil {
		t.Fatal("the load completion issued no layout request")
	}
	jobL := runLayoutJob(t, cmd)
	assertHeld(t, jobL, "the startup layout")
	close(layoutGate)
	lr, ok := collectMsg(t, jobL, "the startup layout").(layoutResult)
	if !ok {
		t.Fatal("the layout job produced no layoutResult")
	}
	if want := (viewport.RowModelKey{Path: "a.txt", Revision: 1, TextWidth: 68, Wrap: true}); lr.key != want {
		t.Fatalf("layout key = %+v, want %+v", lr.key, want)
	}

	// Stage two: the matching install commits the reveal — target row
	// 49 is hidden from top 0, so the top lands at 49 − floor(23/3).
	m, _ = update(t, m, lr)
	if got := m.vps["a.txt"].Top(); got != 42 {
		t.Fatalf("committed reveal top = %d, want 42 (49 − 23/3)", got)
	}
	if m.pendingIntent != intentNone {
		t.Fatal("the matching layout left the reveal intent pending")
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-43") {
		t.Fatalf("panel top row = %q, want line-43", row)
	}
}

// A startup target already visible from top-of-file keeps the top at
// zero — neither stage scrolls — and the first n advances the cursor
// to the second stop.
func TestStartupVisibleTargetKeepsTopZero(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(50))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-08\n", 8, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-15\n", 15, 0, 4, "line"))
	idx.Finish()

	m, loadGate, layoutGate, job := gatedTwoStage(t, dir, idx, 80, 24)
	m.theme = theme.Plain()
	close(loadGate)
	m, cmd := update(t, m, collectMsg(t, job, "a.txt's load"))
	jobL := runLayoutJob(t, cmd)
	assertHeld(t, jobL, "the startup layout")
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "the startup layout"))

	// Target row 7 is inside [0,23): the top stays at the file top.
	if got := m.vps["a.txt"].Top(); got != 0 {
		t.Fatalf("visible startup target scrolled to top %d, want 0", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-01") {
		t.Fatalf("panel top row = %q, want line-01", row)
	}

	// The first n advances to the second stop, still inside the window.
	m, _ = update(t, m, keyMsg("n"))
	if stop, _ := m.index.Current(); stop.Line != 15 {
		t.Fatalf("first n selected line %d, want the second stop 15", stop.Line)
	}
	if got := m.vps["a.txt"].Top(); got != 0 {
		t.Fatalf("n to a visible second stop scrolled to %d, want 0", got)
	}
}

// A resize landing between the two stages supersedes the in-flight
// layout: the old-width completion is discarded without consuming the
// intent, and the commit runs against the layout prepared for the new
// width — the horizontal reveal lands at the new text width.
func TestResizeBetweenStagesCommitsAtNewWidth(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		if i == 50 {
			b.WriteString(strings.Repeat("x", 100) + "hit\n")
		} else {
			fmt.Fprintf(&b, "line-%02d\n", i)
		}
	}
	writeMatchFile(t, dir, "a.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", strings.Repeat("x", 100)+"hit\n", 50, 100, 103, "hit"))
	idx.Finish()

	m, loadGate, layoutGate, job := gatedTwoStage(t, dir, idx, 80, 24)
	m.theme = theme.Plain()
	m, _ = update(t, m, keyMsg("w")) // run-off-edge, so the text width is observable in the offset

	close(loadGate)
	m, cmd := update(t, m, collectMsg(t, job, "a.txt's load"))
	jobOld := runLayoutJob(t, cmd)
	assertHeld(t, jobOld, "the 80-column layout")

	// The resize requests a fresh layout for the new width; the
	// in-flight one is superseded.
	m, resize := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	if resize == nil {
		t.Fatal("the resize requested no replacement layout")
	}
	jobNew := runLayoutJob(t, resize)
	assertHeld(t, jobNew, "the 100-column layout")

	// The stale layout arrives first: discarded — no install, no
	// commit, the intent intact, the placeholder untouched.
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobOld, "the 80-column layout"))
	if m.buffers["a.txt"] != nil {
		t.Fatal("the superseded layout installed")
	}
	if m.pendingIntent != intentReveal {
		t.Fatal("the superseded layout consumed the reveal intent")
	}
	if got := m.View().Content; !strings.Contains(got, "Loading…") {
		t.Fatalf("the superseded layout replaced the placeholder:\n%s", got)
	}

	// The matching layout installs and commits at the new width:
	// panel 93, gutter 5, indicator 1 → text width 87, so the
	// column-100 target lands at offset 100 + 1 − 87 = 14.
	m, _ = update(t, m, collectMsg(t, jobNew, "the 100-column layout"))
	if got := m.vps["a.txt"].Top(); got != 42 {
		t.Fatalf("committed top = %d, want 42 (49 − 23/3)", got)
	}
	if got := m.vps["a.txt"].Offset(); got != 14 {
		t.Fatalf("committed offset = %d, want 14 (100 + 1 − 87)", got)
	}
}

// n moves the cursor immediately while the load is held; the commit
// reveals the newest selected stop — never a target captured when the
// load was requested.
func TestNavigateDuringHeldLoadRevealsNewest(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-50\n", 50, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-80\n", 80, 0, 4, "line"))
	idx.Finish()

	m, loadGate, layoutGate, job := gatedTwoStage(t, dir, idx, 80, 24)
	m.theme = theme.Plain()

	// While the load is held, n advances the cursor immediately and
	// the pending reveal retargets to the newest stop.
	m, c := update(t, m, keyMsg("n"))
	if c != nil {
		t.Fatalf("n during the held load returned a command: %v", c)
	}
	if stop, _ := m.index.Current(); stop.Line != 80 {
		t.Fatalf("n during the held load selected line %d, want 80", stop.Line)
	}
	if m.pendingIntent != intentReveal {
		t.Fatal("the pending reveal intent was lost during the held load")
	}

	// The matching install reveals the latest target — line 80's row
	// 79 → top 79 − floor(23/3) = 72, not the load-time stop's row.
	close(loadGate)
	m, cmd := update(t, m, collectMsg(t, job, "a.txt's load"))
	jobL := runLayoutJob(t, cmd)
	assertHeld(t, jobL, "the startup layout")
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "the startup layout"))
	if got := m.vps["a.txt"].Top(); got != 72 {
		t.Fatalf("committed top = %d, want 72 for the latest selection", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-73") {
		t.Fatalf("panel top row = %q, want line-73", row)
	}
}

// n and p move the cursor immediately while the layout is held; the
// commit reveals whichever stop is selected at install time.
func TestNavigateDuringHeldLayoutRevealsNewest(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-50\n", 50, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-80\n", 80, 0, 4, "line"))
	idx.Finish()

	m, loadGate, layoutGate, job := gatedTwoStage(t, dir, idx, 80, 24)
	m.theme = theme.Plain()
	close(loadGate)
	m, cmd := update(t, m, collectMsg(t, job, "a.txt's load"))
	jobL := runLayoutJob(t, cmd)
	assertHeld(t, jobL, "the startup layout")

	// The layout is held: n then p still move immediately, ending back
	// on line 50 — and the commit targets that final selection.
	m, c := update(t, m, keyMsg("n"))
	if c != nil {
		t.Fatalf("n during the held layout returned a command: %v", c)
	}
	m, c = update(t, m, keyMsg("p"))
	if c != nil {
		t.Fatalf("p during the held layout returned a command: %v", c)
	}
	if stop, _ := m.index.Current(); stop.Line != 50 {
		t.Fatalf("n p during the held layout selected line %d, want 50", stop.Line)
	}
	if m.pendingIntent != intentReveal {
		t.Fatal("the pending reveal intent was lost during the held layout")
	}

	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "the startup layout"))
	if got := m.vps["a.txt"].Top(); got != 42 {
		t.Fatalf("committed top = %d, want 42 for the final selection", got)
	}
}

// The layout request is keyed by the text width computed with the
// loaded file's final gutter: a grown gutter also shrinks the file
// list through the Issue 24 width term, and the commit evaluates
// against the layout built at that final width.
func TestGutterGrowthFeedsFinalTextWidth(t *testing.T) {
	name := "thisisafairlylongfilenameforalistwidth.txt"
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec(name, "hit\n", 95, 0, 3, "hit"))
	idx.Finish()

	src := &stubSource{gutter: 8, widths: make([]int, 100)}
	loader := &stubLoader{fail: map[string]bool{}, src: src}
	m, loadGate, job := gatedLoaderBrowse(t, dir, idx, 30, 10, loader)
	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate

	close(loadGate)
	m, cmd := update(t, m, collectMsg(t, job, "the startup load"))
	jobL := runLayoutJob(t, cmd)
	assertHeld(t, jobL, "the startup layout")
	close(layoutGate)
	lr, ok := collectMsg(t, jobL, "the startup layout").(layoutResult)
	if !ok {
		t.Fatal("the layout job produced no layoutResult")
	}
	// Gutter 8 with a 45-cell widest entry at width 30: the 40% cap and
	// w − (gutter + 10) both give 12, so the list is 12, the panel 18,
	// and the wrap text width 18 − 8 = 10 — not the placeholder
	// gutter's 15.
	if want := (viewport.RowModelKey{Path: name, Revision: 1, TextWidth: 10, Wrap: true}); lr.key != want {
		t.Fatalf("layout key = %+v, want %+v — the grown gutter participates", lr.key, want)
	}

	// The matching install commits at that width: the line-95 stop
	// maps to row 94, landing at 94 − floor(9/3) = 91.
	m, _ = update(t, m, lr)
	if got := m.vps[name].Top(); got != 91 {
		t.Fatalf("committed top = %d, want 91 (94 − 9/3)", got)
	}
}

// A file-list hide between the stages recomputes the text width: the
// superseded layout built for the narrower panel is discarded without
// consuming the intent, and the matching install commits at the
// list-free width.
func TestListToggleBetweenStagesCommitsFinalWidth(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-50\n", 50, 0, 4, "line"))
	idx.Finish()

	m, loadGate, layoutGate, job := gatedTwoStage(t, dir, idx, 80, 24)
	m.theme = theme.Plain()
	close(loadGate)
	m, cmd := update(t, m, collectMsg(t, job, "a.txt's load"))
	jobOld := runLayoutJob(t, cmd)
	assertHeld(t, jobOld, "the listed-panel layout")

	// Hiding the list widens the panel: a fresh keyed request goes out
	// and the in-flight layout is superseded.
	m, c := update(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if c == nil {
		t.Fatal("the list toggle requested no replacement layout")
	}
	jobNew := runLayoutJob(t, c)
	assertHeld(t, jobNew, "the full-width layout")

	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobOld, "the listed-panel layout"))
	if m.buffers["a.txt"] != nil {
		t.Fatal("the superseded layout installed")
	}
	if m.pendingIntent != intentReveal {
		t.Fatal("the superseded layout consumed the reveal intent")
	}

	// The full-panel layout installs — text width 80 − gutter 5 = 75 —
	// and the reveal commits against it.
	m, _ = update(t, m, collectMsg(t, jobNew, "the full-width layout"))
	if got := m.buffers["a.txt"].Key().TextWidth; got != 75 {
		t.Fatalf("installed text width = %d, want 75 (full panel 80 − gutter 5)", got)
	}
	if got := m.vps["a.txt"].Top(); got != 42 {
		t.Fatalf("committed top = %d, want 42", got)
	}
}

// A revisit starts from the saved per-file viewport: when the saved
// top hides the target the commit moves to the one-third row — after
// the matching layout installs — with the EOF clamp applied.
func TestRevisitCommitStartsFromSavedViewport(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(50))
	writeMatchFile(t, dir, "b.txt", numberedContent(50))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-40\n", 40, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "line-04\n", 4, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// The startup reveal landed the target row 39 at top 27; scroll
	// back up half a page so the saved top 16 hides it below the
	// window, then visit b.txt and make every layout stale.
	m, _ = update(t, m, keyMsg("u"))
	if got := m.vps["a.txt"].Top(); got != 16 {
		t.Fatalf("a.txt saved top = %d, want 16", got)
	}
	m, nav := update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, nav))
	m, resize := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m = deliverCmd(t, m, resize)

	// p back to a.txt: its layout is stale — the reveal is pending and
	// the stale layout stays installed until the prepared one lands.
	gate := make(chan struct{})
	m.layoutGate = gate
	m, nav = update(t, m, keyMsg("p"))
	if m.pendingIntent != intentReveal {
		t.Fatal("the revisit reveal was not pending on a stale layout")
	}
	jobL := navStageJob(t, nav)
	assertHeld(t, jobL, "a.txt's revisit layout")
	if got := m.buffers["a.txt"].Key().TextWidth; got != 69 {
		t.Fatalf("the stale layout was replaced early: text width %d, want 69", got)
	}

	// The matching install starts the reveal from the saved top 16:
	// target row 39 is hidden, so the top moves to 39 − 23/3 = 32,
	// clamped to the 50-row extent's maximum 27.
	close(gate)
	m, _ = update(t, m, collectMsg(t, jobL, "a.txt's revisit layout"))
	if got := m.vps["a.txt"].Top(); got != 27 {
		t.Fatalf("committed revisit top = %d, want 27 (one-third placement EOF-clamped)", got)
	}
	if got := m.buffers["a.txt"].Key().TextWidth; got != 89 {
		t.Fatalf("committed text width = %d, want 89 (panel 93 − gutter 4)", got)
	}
}

// A revisit whose saved top already shows the target keeps that
// position: the commit applies visible-target no-scroll against the
// installed rows.
func TestRevisitVisibleTargetKeepsSavedTop(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(50))
	writeMatchFile(t, dir, "b.txt", numberedContent(50))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-06\n", 6, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "line-04\n", 4, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// Scroll so the saved top shows the line-6 target row 5 inside
	// [3,26), visit b.txt, and make every layout stale.
	for i := 0; i < 3; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if got := m.vps["a.txt"].Top(); got != 3 {
		t.Fatalf("a.txt saved top = %d, want 3", got)
	}
	m, nav := update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, nav))
	m, resize := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m = deliverCmd(t, m, resize)

	gate := make(chan struct{})
	m.layoutGate = gate
	m, nav = update(t, m, keyMsg("p"))
	jobL := navStageJob(t, nav)
	assertHeld(t, jobL, "a.txt's revisit layout")

	// The matching install leaves the saved top alone: row 5 stays
	// inside [3,26).
	close(gate)
	m, _ = update(t, m, collectMsg(t, jobL, "a.txt's revisit layout"))
	if got := m.vps["a.txt"].Top(); got != 3 {
		t.Fatalf("committed revisit top = %d, want the saved 3", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-04") {
		t.Fatalf("panel top row = %q, want line-04", row)
	}
}

// A terminator-only match is an ordinary marker target through the
// two-stage commit: the marker's row is revealed at one-third and the
// marker cell itself paints in run-off-edge mode.
func TestMarkerTargetCommitsThroughBothStages(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		if i == 50 {
			b.WriteString("hit\r\n")
		} else {
			fmt.Fprintf(&b, "line-%02d\n", i)
		}
	}
	writeMatchFile(t, dir, "a.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit\r\n", 50, 4, 4, ""))
	idx.Finish()

	m, loadGate, layoutGate, job := gatedTwoStage(t, dir, idx, 80, 24)
	m, _ = update(t, m, keyMsg("w")) // run-off-edge
	close(loadGate)
	m, cmd := update(t, m, collectMsg(t, job, "a.txt's load"))
	jobL := runLayoutJob(t, cmd)
	assertHeld(t, jobL, "the startup layout")
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "the startup layout"))

	// The marker cell at display column 3 is the target; its rendered
	// row 49 lands at one-third, and the cell is visible from the left
	// edge.
	if got := m.vps["a.txt"].Top(); got != 42 {
		t.Fatalf("marker commit top = %d, want 42", got)
	}
	if got := m.vps["a.txt"].Offset(); got != 0 {
		t.Fatalf("marker commit offset = %d, want 0", got)
	}
	row := viewRow(t, m, 8) // target row 49 at window row 49 − 42 = 7 → screen row 8
	if !strings.Contains(row, "hit\x1b[4m\x1b[30;47m \x1b[24m") {
		t.Fatalf("marker row lacks the painted marker cell: %q", row)
	}
}

// A first submatch that starts mid-cluster resolves to the cluster's
// start cell: the run-off-edge commit reveals the target by exactly
// enough to paint the whole cluster at the right edge.
func TestClusterTargetCommitsThroughBothStages(t *testing.T) {
	dir := t.TempDir()
	wide := strings.Repeat("x", 300) + "世zz"
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		if i == 50 {
			b.WriteString(wide + "\n")
		} else {
			fmt.Fprintf(&b, "line-%02d\n", i)
		}
	}
	writeMatchFile(t, dir, "a.txt", b.String())
	idx := searchindex.New(dir)
	// The submatch [301,303) starts inside the 世 cluster's bytes
	// [300,303); its display target is the cluster's start cell 300.
	addRec(t, idx, matchRec("a.txt", wide+"\n", 50, 301, 303, "x"))
	idx.Finish()

	m, loadGate, layoutGate, job := gatedTwoStage(t, dir, idx, 80, 24)
	m.theme = theme.Plain()
	m, _ = update(t, m, keyMsg("w"))
	close(loadGate)
	m, cmd := update(t, m, collectMsg(t, job, "a.txt's load"))
	jobL := runLayoutJob(t, cmd)
	assertHeld(t, jobL, "the startup layout")
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "the startup layout"))

	// Panel 73, gutter 5, indicator 1 → text width 67; the two-cell
	// cluster at column 300 reveals to 300 + 2 − 67 = 235.
	if got := m.vps["a.txt"].Top(); got != 42 {
		t.Fatalf("cluster commit top = %d, want 42", got)
	}
	if got := m.vps["a.txt"].Offset(); got != 235 {
		t.Fatalf("cluster commit offset = %d, want 235 (300 + 2 − 67)", got)
	}
	if row := viewRow(t, m, 8); !strings.HasSuffix(row, "世 ") {
		t.Fatalf("the revealed row does not end with the whole cluster: %q", row)
	}
}

// A load completion for a file that is not current updates only its
// own cache and status: no layout is requested, no viewport or intent
// changes, and the visible panel is byte-identical — a late layout
// naming that path is likewise discarded.
func TestNonCurrentLoadCompletionIsIsolated(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	writeMatchFile(t, dir, "c.txt", "hit c\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("c.txt", "hit c\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, gate, jobA := gatedBrowse(t, dir, idx, 80, 24)
	m.theme = theme.Plain()

	// Visit c.txt through b.txt — all three loads stay held.
	m, nav := update(t, m, keyMsg("n"))
	jobsB := navJobs(t, nav)
	m, nav = update(t, m, keyMsg("n"))
	jobsC := navJobs(t, nav)

	// Deliver c.txt so the panel is real content, then land b.txt's
	// completion while c.txt is current.
	close(gate)
	var loadB tea.Msg
	for _, j := range jobsB {
		if lr, ok := collectMsg(t, j, "b.txt navigation job").(loadResult); ok {
			loadB = lr
		}
	}
	var loadC tea.Msg
	for _, j := range jobsC {
		if lr, ok := collectMsg(t, j, "c.txt navigation job").(loadResult); ok {
			loadC = lr
		}
	}
	m = applyLoad(t, m, collectMsg(t, jobA, "a.txt's load"))
	m = applyLoad(t, m, loadC)
	before := m.View().Content
	if !strings.Contains(before, "hit c") {
		t.Fatalf("c.txt's panel did not render:\n%s", before)
	}

	// b.txt's completion while c.txt is current: cache only — no
	// command, no panel change, no intent change.
	m, c := update(t, m, loadB)
	if c != nil {
		t.Fatalf("a non-current completion issued a command: %v", c)
	}
	if m.sources["b.txt"] == nil || m.revs["b.txt"] != 1 {
		t.Fatal("the non-current completion did not cache its source")
	}
	if m.pendingIntent != intentNone {
		t.Fatalf("the non-current completion left intent %v, want none", m.pendingIntent)
	}
	if got := m.View().Content; got != before {
		t.Fatalf("the non-current completion changed the panel:\nbefore:\n%s\nafter:\n%s", before, got)
	}

	// A prepared layout naming the non-current path is discarded too.
	key := m.rowKey("b.txt", m.sources["b.txt"])
	m, c = update(t, m, layoutResult{key: key, model: viewport.NewRowModel(key, m.sources["b.txt"])})
	if c != nil {
		t.Fatal("a non-current layout result issued a command")
	}
	if got := m.View().Content; got != before {
		t.Fatal("a non-current layout result changed the panel")
	}
	if m.buffers["b.txt"] != nil {
		t.Fatal("the non-current layout installed")
	}
}

// The file-change pop-up is a selection-time presentation: neither
// stage of the destination's completion disturbs it — only its own
// instance-keyed expiry dismisses it.
func TestPopupSurvivesLoadAndLayoutStages(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, gate, jobA := gatedBrowse(t, dir, idx, 80, 24)
	close(gate)
	m = applyLoad(t, m, collectMsg(t, jobA, "a.txt's load"))
	m.theme = theme.Plain()

	loadGate := make(chan struct{})
	m.loadGate = loadGate
	layoutGate := make(chan struct{})
	m.layoutGate = layoutGate

	m, nav := update(t, m, keyMsg("n"))
	if m.popup == nil {
		t.Fatal("cross-file n opened no pop-up")
	}
	id := m.popup.id
	jobB := navStageJob(t, nav)
	assertSilent(t, jobB, "b.txt's load")

	// Stage one: the load completion leaves the pop-up alone.
	close(loadGate)
	m, layout := update(t, m, collectMsg(t, jobB, "b.txt's load"))
	if m.popup == nil || m.popup.id != id {
		t.Fatal("the load completion disturbed the pop-up")
	}
	popupBox(t, m.View().Content, "b.txt")

	// Stage two: the layout install leaves it alone too.
	jobL := runLayoutJob(t, layout)
	assertHeld(t, jobL, "b.txt's layout")
	close(layoutGate)
	m, _ = update(t, m, collectMsg(t, jobL, "b.txt's layout"))
	if m.popup == nil || m.popup.id != id {
		t.Fatal("the layout install disturbed the pop-up")
	}
	popupBox(t, m.View().Content, "b.txt")

	// Only the instance's own expiry dismisses it.
	m, _ = update(t, m, popupExpiredMsg{id: id})
	if m.popup != nil {
		t.Fatal("the pop-up's own expiry did not dismiss it")
	}
}

// The deferred reveal commit runs the whole file-change reveal
// sequence against the installed rows — including the horizontal
// reset: the offset retained from the superseded layout is zeroed
// before the minimal horizontal reveal, so a target left of the
// retained window lands at offset zero, not its own column.
func TestPendingRevealCommitResetsOffset(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		switch i {
		case 1:
			b.WriteString("aaaaa" + strings.Repeat("x", 195) + "\n")
		case 50:
			b.WriteString("bbbbbcc\n")
		default:
			fmt.Fprintf(&b, "line-%02d\n", i)
		}
	}
	writeMatchFile(t, dir, "a.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "aaaaa"+strings.Repeat("x", 195)+"\n", 1, 5, 7, "xx"))
	addRec(t, idx, matchRec("a.txt", "bbbbbcc\n", 50, 5, 7, "cc"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m, _ = update(t, m, keyMsg("w")) // run-off-edge before the load lands
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// Pan the installed layout right; the line-1 target's column-5
	// start is hidden left of the retained window.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, keyMsg(">"))
	}
	if got := m.vps["a.txt"].Offset(); got != 50 {
		t.Fatalf("setup pan: offset = %d, want 50", got)
	}

	// A resize leaves a pending layout and a pending reveal once n
	// selects the same-file second stop.
	gate := make(chan struct{})
	m.layoutGate = gate
	m, resize := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	jobL := runLayoutJob(t, resize)
	assertHeld(t, jobL, "the resized layout")
	m, c := update(t, m, keyMsg("n"))
	if c != nil {
		t.Fatalf("n during the held layout returned a command: %v", c)
	}
	if stop, _ := m.index.Current(); stop.Line != 50 {
		t.Fatalf("n selected line %d, want 50", stop.Line)
	}
	if m.pendingIntent != intentReveal {
		t.Fatal("n during the held layout did not leave a pending reveal")
	}

	// The matching install commits the full destination sequence: the
	// offset resets to zero — the column-5 target is then already
	// visible — and the hidden target row lands at one-third.
	close(gate)
	m, _ = update(t, m, collectMsg(t, jobL, "the resized layout"))
	if got := m.vps["a.txt"].Top(); got != 42 {
		t.Fatalf("committed top = %d, want 42 (49 − 23/3)", got)
	}
	if got := m.vps["a.txt"].Offset(); got != 0 {
		t.Fatalf("committed offset = %d, want the reset 0 — the column-5 target is visible from the left edge", got)
	}
}
