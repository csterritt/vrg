package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// runLayoutJob executes a layout command on its own goroutine — a
// gate-held job blocks until the gate closes or the model cancels —
// and reports the produced message on the returned channel.
func runLayoutJob(t *testing.T, cmd tea.Cmd) <-chan tea.Msg {
	t.Helper()
	return runWorker(t, cmd)
}

// assertHeld fails when a job completed while the layout gate is held.
func assertHeld(t *testing.T, done <-chan tea.Msg, what string) {
	t.Helper()
	select {
	case <-done:
		t.Fatalf("%s completed while the layout gate was held", what)
	default:
	}
}

// navMsgs runs a navigation command and returns every message its
// commands produce — load results, layout results, or pop-up expiries.
func navMsgs(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	out := make(chan tea.Msg, len(batch))
	for _, c := range batch {
		go func(c tea.Cmd) { out <- c() }(c)
	}
	msgs := make([]tea.Msg, 0, len(batch))
	deadline := time.After(10 * time.Second)
	for range batch {
		select {
		case got := <-out:
			msgs = append(msgs, got)
		case <-deadline:
			t.Fatal("a navigation command produced no message")
		}
	}
	return msgs
}

// navLayoutResult extracts the one layout result a navigation command
// carries, failing when none does.
func navLayoutResult(t *testing.T, cmd tea.Cmd) layoutResult {
	t.Helper()
	for _, msg := range navMsgs(t, cmd) {
		if lr, ok := msg.(layoutResult); ok {
			return lr
		}
	}
	t.Fatal("navigation carried no layout request")
	return layoutResult{}
}

// With the layout worker held behind the gate after a resize, every
// input stays actionable: n/p move the cursor immediately with the
// pending reveal preserved for whichever stop is newest, w flips the
// mode and issues its own keyed request, and a second resize is
// accepted — all without releasing the gate. Once a layout matching the
// current parameters installs, the pending reveal commits with the
// Issue 14 rules; the superseded completions are discarded without
// touching the display, the anchor, or the intent.
func TestGatedLayoutKeepsInputResponsive(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-05\n", 5, 0, 4, "line"))
	addRec(t, idx, matchRec("a.txt", "line-95\n", 95, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// Scroll mid-file so the pending reveal's commit visibly moves the
	// viewport when the matching layout lands.
	for i := 0; i < 30; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if got := m.vps["a.txt"].Top(); got != 30 {
		t.Fatalf("scrolled to top %d, want 30", got)
	}

	gate := make(chan struct{})
	m.layoutGate = gate

	// A width-changing resize requests a prepared layout; the worker
	// holds behind the gate and the previous layout stays on screen.
	m, resize := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if resize == nil {
		t.Fatal("resize requested no layout preparation")
	}
	var jobs []<-chan tea.Msg
	jobs = append(jobs, runLayoutJob(t, resize))
	assertHeld(t, jobs[0], "the resized layout")
	if row := contentRow(t, m); !strings.Contains(row, "line-31") {
		t.Fatalf("held layout lost the previous layout's display: %q", row)
	}

	// n and p move the cursor immediately; the pending reveal intent is
	// preserved for whichever stop is newest.
	m, c := update(t, m, keyMsg("n"))
	if c != nil {
		t.Fatalf("n during a held layout returned a command: %v", c)
	}
	if stop, _ := m.index.Current(); stop.Line != 95 {
		t.Fatalf("n during a held layout selected line %d, want 95", stop.Line)
	}
	m, c = update(t, m, keyMsg("p"))
	if stop, _ := m.index.Current(); stop.Line != 5 {
		t.Fatalf("p during a held layout selected line %d, want 5", stop.Line)
	}
	if m.pendingIntent != intentReveal {
		t.Fatal("the pending reveal intent was lost while the layout was held")
	}

	// w flips the wrap mode immediately and issues its own keyed layout
	// request; the existing gate stays held.
	m, wCmd := update(t, m, keyMsg("w"))
	if m.wrap {
		t.Fatal("w did not flip the wrap mode while the layout was held")
	}
	if wCmd == nil {
		t.Fatal("w issued no layout request for the new mode")
	}
	jobs = append(jobs, runLayoutJob(t, wCmd))
	assertHeld(t, jobs[1], "the wrap-mode layout")

	// A second resize is accepted while the gate stays held.
	m, resize2 := update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 || m.height != 40 {
		t.Fatalf("second resize not accepted: %dx%d, want 120x40", m.width, m.height)
	}
	if resize2 == nil {
		t.Fatal("second resize requested no layout preparation")
	}
	jobs = append(jobs, runLayoutJob(t, resize2))
	for i, j := range jobs {
		assertHeld(t, j, fmt.Sprintf("layout job %d", i))
	}

	// Release the gate and collect the three completions in issue order:
	// 100-wide wrap, 100-wide nowrap, 120-wide nowrap.
	close(gate)
	var results []layoutResult
	for _, j := range jobs {
		select {
		case msg := <-j:
			lr, ok := msg.(layoutResult)
			if !ok {
				t.Fatalf("layout job produced %T, want layoutResult", msg)
			}
			results = append(results, lr)
		case <-time.After(10 * time.Second):
			t.Fatal("a layout job did not complete after the gate was released")
		}
	}
	want := m.rowKey("a.txt", m.sources["a.txt"])
	if results[2].key != want {
		t.Fatalf("newest layout's key = %+v, want the current %+v", results[2].key, want)
	}

	// The superseded layouts are discarded: no install, no anchor or
	// display change, no consumption of the pending intent.
	old := m.buffers["a.txt"].Key()
	anchor := m.vps["a.txt"].Anchor()
	for _, lr := range results[:2] {
		m, _ = update(t, m, lr)
		if got := m.buffers["a.txt"].Key(); got != old {
			t.Fatalf("obsolete layout %+v installed over %+v", lr.key, old)
		}
		if got := m.vps["a.txt"].Anchor(); got != anchor {
			t.Fatalf("obsolete layout moved the anchor to %+v, want %+v", got, anchor)
		}
		if m.pendingIntent != intentReveal {
			t.Fatal("an obsolete layout consumed the pending reveal intent")
		}
	}

	// The matching layout installs and commits the pending reveal with
	// the Issue 14 rules: the newest stop line-05's target row 4 is
	// hidden above the window, so the top moves and clamps to the top of
	// the file.
	m, _ = update(t, m, results[2])
	if got := m.buffers["a.txt"].Key(); got != want {
		t.Fatalf("matching layout did not install: key = %+v, want %+v", got, want)
	}
	if m.pendingIntent != intentNone {
		t.Fatal("the matching layout left the pending reveal uncommitted")
	}
	if got := m.vps["a.txt"].Top(); got != 0 {
		t.Fatalf("committed reveal top = %d, want 0 (target row 4 clamps to BOF)", got)
	}
	if row := viewRow(t, m, 5); !strings.Contains(row, "line-05") {
		t.Fatalf("committed reveal lacks the newest stop's line: %q", row)
	}
}

// ctrl+c while a layout job is gate-held exits 130 immediately through
// the cancellation path; the cancelled worker is released promptly and
// its late result cannot revive the UI.
func TestGatedLayoutCtrlCExits130(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-01\n", 1, 0, 4, "line"))
	idx.Finish()

	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		terminated: make(chan struct{}),
	}
	m := New(child, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchResult{index: idx, integrity: completeStream})
	m = applyLoad(t, m, cmd())

	// The width-changing resize's layout job is the held worker.
	m.layoutGate = make(chan struct{}) // never closed; cancellation releases it
	m, resize := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if resize == nil {
		t.Fatal("resize requested no layout preparation")
	}
	done := runLayoutJob(t, resize)
	assertHeld(t, done, "the resized layout")

	m, quit := update(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if quit == nil {
		t.Fatal("ctrl+c during a held layout returned no command, want tea.Quit")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c during a held layout returned %T, want tea.QuitMsg", quit())
	}
	if m.status != 130 {
		t.Fatalf("exit status = %d, want 130", m.status)
	}
	requireClosed(t, child.terminated, "child termination")

	// Cancellation released the gate-held worker; its late result must
	// not revive the UI.
	var late tea.Msg
	select {
	case late = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("gate-held layout did not return promptly after cancellation")
	}
	m, c := update(t, m, late)
	if c != nil {
		t.Fatalf("late layout result after cancellation returned a command: %v", c)
	}
	if m.state != stateCancelled || m.status != 130 {
		t.Fatalf("late layout result revived the UI: state %d, status %d", m.state, m.status)
	}
}

// q while a layout job is gate-held exits with the fixed ordinary
// status immediately — the held worker is not waited on.
func TestGatedLayoutQExitsOrdinarily(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-01\n", 1, 0, 4, "line"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())

	m.layoutGate = make(chan struct{}) // never closed
	m, resize := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if resize == nil {
		t.Fatal("resize requested no layout preparation")
	}
	assertHeld(t, runLayoutJob(t, resize), "the resized layout")

	m, quit := update(t, m, keyMsg("q"))
	requireQuit(t, quit, "q during a held layout")
	if m.status != 0 {
		t.Fatalf("q during a held layout exited %d, want the fixed ordinary 0", m.status)
	}
}

// Successive resizes issue keyed layout jobs; delivered out of order,
// only the layout matching the current parameters installs — the
// superseded ones are discarded without touching the display or the
// anchor.
func TestOutOfOrderLayoutCompletionsDiscarded(t *testing.T) {
	m := browseLoaded(t, "a.txt", numberedContent(100), 80, 24)
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	anchor := m.vps["a.txt"].Anchor()
	old := m.buffers["a.txt"].Key()

	m, j1 := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m, j2 := update(t, m, tea.WindowSizeMsg{Width: 120, Height: 24})
	m, j3 := update(t, m, tea.WindowSizeMsg{Width: 140, Height: 24})
	for i, j := range []tea.Cmd{j1, j2, j3} {
		if j == nil {
			t.Fatalf("resize %d requested no layout preparation", i+1)
		}
	}

	// Deliver out of order: the two superseded results are discarded.
	for _, j := range []tea.Cmd{j2, j1} {
		m = deliverCmd(t, m, j)
		if got := m.buffers["a.txt"].Key(); got != old {
			t.Fatalf("an obsolete layout installed: key = %+v, want %+v", got, old)
		}
		if got := m.vps["a.txt"].Anchor(); got != anchor {
			t.Fatalf("an obsolete layout moved the anchor to %+v, want %+v", got, anchor)
		}
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-11") {
		t.Fatalf("obsolete layouts changed the display: %q, want line-11", row)
	}

	// The current-parameters layout installs and re-derives the top
	// from the unchanged anchor.
	m = deliverCmd(t, m, j3)
	want := m.rowKey("a.txt", m.sources["a.txt"])
	if got := m.buffers["a.txt"].Key(); got != want {
		t.Fatalf("matching layout did not install: key = %+v, want %+v", got, want)
	}
	if got := m.vps["a.txt"].Anchor(); got != anchor {
		t.Fatalf("the matching layout moved the anchor to %+v, want %+v", got, anchor)
	}
	if row := contentRow(t, m); !strings.Contains(row, "line-11") {
		t.Fatalf("the matching layout's display = %q, want line-11", row)
	}
}

// Rapid wrap toggles issue keyed requests; the obsolete completion —
// prepared for a mode that is no longer current — is discarded and the
// anchor is unaffected throughout.
func TestRapidWrapToggleDiscardsObsoleteLayout(t *testing.T) {
	// A 100-cell first line wraps to two rows at the 80-column fixture's
	// text width; the short tail keeps the file taller than the panel.
	m := browseLoaded(t, "a.txt", strings.Repeat("x", 100)+"\n"+numberedContent(50), 80, 24)

	// Scroll onto the wrapped line's continuation row: the anchor is a
	// mid-line logical column.
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	anchor := m.vps["a.txt"].Anchor()
	if anchor.Line != 0 || anchor.Col == 0 {
		t.Fatalf("anchor = %+v, want a mid-line column on line 0", anchor)
	}

	m, w1 := update(t, m, keyMsg("w")) // → run-off-edge request
	m, w2 := update(t, m, keyMsg("w")) // → wrap request
	if !m.wrap {
		t.Fatal("two w presses did not restore wrap mode")
	}

	// The first toggle's layout is obsolete on arrival: discarded
	// without touching the installed model, the anchor, or the display.
	old := m.buffers["a.txt"].Key()
	m = deliverCmd(t, m, w1)
	if got := m.buffers["a.txt"].Key(); got != old {
		t.Fatalf("obsolete nowrap layout installed: key = %+v, want %+v", got, old)
	}
	if got := m.vps["a.txt"].Anchor(); got != anchor {
		t.Fatalf("obsolete nowrap layout moved the anchor to %+v, want %+v", got, anchor)
	}

	// The second toggle's layout matches current parameters and
	// installs; the mid-line anchor maps back to the continuation row.
	m = deliverCmd(t, m, w2)
	if got := m.buffers["a.txt"].Key(); !got.Wrap {
		t.Fatalf("installed key = %+v, want wrap mode", got)
	}
	if got := m.vps["a.txt"].Top(); got != 1 {
		t.Fatalf("top after the wrap round trip = %d, want the anchor's row 1", got)
	}
}

// A prepared layout for a file that is no longer current is discarded:
// the visible panel, the current file's installed layout, and the
// departed file's saved state stay untouched.
func TestLayoutForDepartedFileDiscarded(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(50))
	var b strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&b, "row-%02d\n", i)
	}
	writeMatchFile(t, dir, "b.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-06\n", 6, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "row-04\n", 4, 0, 3, "row"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// The resize's a.txt layout job is captured, not delivered — by the
	// time it lands, a.txt is no longer current.
	m, stale := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	if stale == nil {
		t.Fatal("resize requested no layout preparation")
	}
	aKey := m.buffers["a.txt"].Key()
	aAnchor := m.vps["a.txt"].Anchor()

	m, nav := update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, nav))
	if got := contentRow(t, m); !strings.Contains(got, "row-01") {
		t.Fatalf("b.txt panel = %q, want its first row", got)
	}

	// The departed file's late layout: discarded without touching
	// a.txt's installed model or saved viewport, or b.txt's display.
	m = deliverCmd(t, m, stale)
	if got := m.buffers["a.txt"].Key(); got != aKey {
		t.Fatalf("departed file's installed key = %+v, want unchanged %+v", got, aKey)
	}
	if got := m.vps["a.txt"].Anchor(); got != aAnchor {
		t.Fatalf("departed file's saved anchor = %+v, want unchanged %+v", got, aAnchor)
	}
	if got := contentRow(t, m); !strings.Contains(got, "row-01") {
		t.Fatalf("departed layout changed the visible panel: %q", got)
	}
}

// Navigating back to a cached file whose installed layout is stale
// requests a prepared layout for the current parameters; the saved
// per-file position is the entry intent and commits when the matching
// layout installs — the stale layout stays on screen meanwhile.
func TestStaleLayoutNavigationRequestsPreparedLayout(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(50))
	var b strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&b, "row-%02d\n", i)
	}
	writeMatchFile(t, dir, "b.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-06\n", 6, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "row-25\n", 25, 0, 3, "row"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// Visit b.txt: the entry reveal puts its target row 24 at the
	// window's third — top 17. Scroll up 4 for a saved top of 13 that
	// still shows the target row.
	m, nav := update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, nav))
	for i := 0; i < 4; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	}
	if got := m.vps["b.txt"].Top(); got != 13 {
		t.Fatalf("b.txt saved top = %d, want 13", got)
	}
	bKey := m.buffers["b.txt"].Key()

	// Back to a.txt, then a resize makes every installed layout stale.
	m, _ = update(t, m, keyMsg("p"))
	m, resize := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m = deliverCmd(t, m, resize) // a.txt's fresh layout installs

	// n to b.txt: cached but stale — a keyed layout request is issued
	// and the entry intent is pending; the stale layout stays installed.
	m, nav = update(t, m, keyMsg("n"))
	if got := m.buffers["b.txt"].Key(); got != bKey {
		t.Fatalf("b.txt's installed key changed at selection: %+v, want %+v", got, bKey)
	}
	if m.pendingIntent != intentReveal {
		t.Fatal("the stale-file entry reveal was not pending")
	}
	lr := navLayoutResult(t, nav)
	want := m.rowKey("b.txt", m.sources["b.txt"])
	if lr.key != want {
		t.Fatalf("issued layout key = %+v, want the current %+v", lr.key, want)
	}

	// The matching layout installs and the intent commits: the saved
	// position is the reveal's starting point and the target row 24 is
	// visible inside it, so the saved top 13 survives.
	m, _ = update(t, m, lr)
	if m.pendingIntent != intentNone {
		t.Fatal("the matching layout left the entry intent pending")
	}
	if got := m.vps["b.txt"].Top(); got != 13 {
		t.Fatalf("committed entry top = %d, want the saved 13", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "row-14") {
		t.Fatalf("b.txt panel = %q, want the saved top row-14", row)
	}
}

// Navigating to a cached file whose installed layout already matches
// the current parameters commits the reveal immediately — no load and
// no layout request is issued.
func TestMatchingLayoutNavigationIsImmediate(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(50))
	var b strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&b, "row-%02d\n", i)
	}
	writeMatchFile(t, dir, "b.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-06\n", 6, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "row-40\n", 40, 0, 3, "row"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()

	// Cache b.txt at the current parameters, then return to a.txt.
	m, nav := update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, nav))
	m, _ = update(t, m, keyMsg("p"))

	// n to b.txt again: the installed layout matches, so the command is
	// only the pop-up's expiry — no staging work — and the reveal has
	// already committed: the hidden target row 39 moves the top.
	m, nav = update(t, m, keyMsg("n"))
	for _, msg := range navMsgs(t, nav) {
		switch msg.(type) {
		case loadResult:
			t.Fatal("matching-layout navigation requested a load")
		case layoutResult:
			t.Fatal("matching-layout navigation requested a layout")
		}
	}
	if m.pendingIntent != intentNone {
		t.Fatal("the matching-layout entry left the reveal intent pending")
	}
	// Content height 23, third row 7: target row 39 → top 32, under the
	// 50-23=27 clamp → 27.
	if got := m.vps["b.txt"].Top(); got != 27 {
		t.Fatalf("immediate reveal top = %d, want 27", got)
	}
	if row := contentRow(t, m); !strings.Contains(row, "row-28") {
		t.Fatalf("b.txt panel = %q, want the revealed top row-28", row)
	}
}

// A frame render queries the file list's visible window only: a
// counting item provider proves the frame never scans the whole list.
func TestRenderQueriesOnlyVisibleListEntries(t *testing.T) {
	idx := searchindex.New("/w")
	for i := 0; i < 10000; i++ {
		name := fmt.Sprintf("f%05d.txt", i)
		addRec(t, idx, matchRec(name, "hit\n", 1, 0, 3, "hit"))
	}
	idx.Finish()

	m, _ := startBrowse(t, "/w", idx, 80, 24)

	var calls []int
	m.itemName = func(i int) string {
		calls = append(calls, i)
		return fmt.Sprintf("f%05d.txt", i)
	}
	_ = m.View()

	if len(calls) > 24 {
		t.Fatalf("frame queried %d list entries of 10000, want only the %d visible rows",
			len(calls), 24)
	}
	for _, i := range calls {
		if i < 0 || i >= 24 {
			t.Fatalf("frame queried list entry %d outside the visible window", i)
		}
	}
}

// The prepared-layout key pins the full parameter tuple: a completion
// carrying another path's key never matches the current file's guard.
func TestLayoutKeyRequiresCurrentFile(t *testing.T) {
	m := browseLoaded(t, "a.txt", numberedContent(50), 80, 24)

	// A foreign key — another path's identity at the current geometry —
	// must not install over a.txt's model.
	foreign := viewport.NewRowModel(viewport.RowModelKey{
		Path:      "b.txt",
		Revision:  m.revs["a.txt"],
		TextWidth: m.buffers["a.txt"].Key().TextWidth,
		Wrap:      m.wrap,
	}, m.sources["a.txt"])
	top := m.vps["a.txt"].Top()
	m, cmd := update(t, m, layoutResult{key: foreign.Key(), model: foreign})
	if cmd != nil {
		t.Fatalf("a foreign layout returned a command: %v", cmd)
	}
	if m.buffers["a.txt"] == foreign {
		t.Fatal("a foreign-keyed layout installed over the current file")
	}
	if got := m.vps["a.txt"].Top(); got != top {
		t.Fatalf("a foreign layout moved the top to %d, want %d", got, top)
	}
}
